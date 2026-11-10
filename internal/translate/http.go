package translate

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/tidwall/gjson"
)

// HTTPTranslate 通用 HTTP 翻译源。
//
// url/headers/query/body 都是 Go text/template，可用变量：
//
//	{{.Text}}     原文            {{.Target}}  目标语言码（zh-CN/en/...）
//	{{.From}}     源语言(auto)    {{.Keys.xxx}} 该源 keys 里的任意凭据
//	{{.APIKey}}   等价 {{.Keys.api_key}}
//	{{.Salt}}     随机串（每次请求固定，可同时用于参数和签名）
//	{{.CurTime}}  秒级时间戳（同上）
//
// 还提供哈希/编码函数，够拼国内各家签名：
//
//	{{md5 "..."}} {{sha256 "..."}} {{sha1 "..."}}
//	{{hmacSHA256 .Keys.secret (printf "%s%s" .Text .Salt)}}
//	{{b64 "..."}} {{b64url "..."}} {{upper/lower "..."}}
func HTTPTranslate(ctx context.Context, text, toLang string, p *Provider) (string, error) {
	if p == nil || p.URL == "" {
		return "", fmt.Errorf("http 翻译源未配置 url")
	}
	data := templateData(text, toLang, p)

	method := strings.ToUpper(p.Method)
	if method == "" {
		method = http.MethodPost
	}

	var bodyReader io.Reader
	if method != http.MethodGet && method != http.MethodHead && p.Body != "" {
		b, err := renderTemplate(p.Body, data)
		if err != nil {
			return "", err
		}
		bodyReader = strings.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.URL, bodyReader)
	if err != nil {
		return "", err
	}
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range p.Headers {
		hv, err := renderTemplate(v, data)
		if err != nil {
			return "", err
		}
		req.Header.Set(k, hv)
	}
	if len(p.Query) > 0 {
		q := req.URL.Query()
		for k, v := range p.Query {
			qv, err := renderTemplate(v, data)
			if err != nil {
				return "", err
			}
			q.Set(k, qv)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http 翻译源请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http 翻译源 HTTP %d: %.200s", resp.StatusCode, string(raw))
	}

	out := string(raw)
	if p.ResultPath != "" {
		g := gjson.GetBytes(raw, p.ResultPath)
		if !g.Exists() || g.String() == "" {
			return "", fmt.Errorf("http 翻译源未返回结果（result_path=%q）: %.200s", p.ResultPath, out)
		}
		out = g.String()
	}
	return strings.TrimSpace(out), nil
}

// templateData 构造模板数据；Salt/CurTime 每次请求只算一次，保证各处签名一致
func templateData(text, toLang string, p *Provider) map[string]any {
	salt, _ := randHex(8)
	keys := map[string]string{}
	for k, v := range p.Keys {
		keys[k] = v
	}
	if p.AppKey != "" {
		keys["app_key"] = p.AppKey
	}
	if p.AppSecret != "" {
		keys["app_secret"] = p.AppSecret
	}
	return map[string]any{
		"Text":    text,
		"Target":  toLang,
		"To":      toLang,
		"From":    "auto",
		"Keys":    keys,
		"APIKey":  p.key("api_key"),
		"Salt":    salt,
		"CurTime": strconv.FormatInt(time.Now().Unix(), 10),
	}
}

// renderTemplate 渲染模板（不含 {{ 时原样返回）
func renderTemplate(s string, data map[string]any) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	tpl, err := template.New("t").Funcs(templateFuncs()).Parse(s)
	if err != nil {
		return "", fmt.Errorf("模板解析失败: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("模板渲染失败: %w", err)
	}
	return buf.String(), nil
}

// templateFuncs 签名/编码辅助函数
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"md5":    func(s string) string { h := md5.Sum([]byte(s)); return hex.EncodeToString(h[:]) },
		"sha1":   func(s string) string { h := sha1.Sum([]byte(s)); return hex.EncodeToString(h[:]) },
		"sha256": func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) },
		"hmacSHA256": func(key, msg string) string {
			h := hmac.New(sha256.New, []byte(key))
			h.Write([]byte(msg))
			return hex.EncodeToString(h.Sum(nil))
		},
		"b64":    func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) },
		"b64url": func(s string) string { return base64.URLEncoding.EncodeToString([]byte(s)) },
		"upper":  strings.ToUpper,
		"lower":  strings.ToLower,
		"randhex": func(n int) string {
			b, _ := randHex(n)
			return b
		},
		"now": func() int64 { return time.Now().Unix() },
	}
}

// randHex 生成 n 字节随机数的 hex
func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
