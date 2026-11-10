package translate

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 有道智云翻译 API（openapi.youdao.com），需要应用 app_key/app_secret。
// 签名规则（v3）：sha256(appKey + input + salt + curtime + appSecret)
func (p *Provider) youdaoValid() bool {
	return p != nil && p.key("app_key") != "" && p.key("app_secret") != ""
}

const youdaoAPI = "https://openapi.youdao.com/api"

// youdaoLang 把本应用语言码转成有道语言码
func youdaoLang(lang string) string {
	return map[string]string{
		Chinese:  "zh-CHS",
		English:  "en",
		Japanese: "ja",
		Korean:   "ko",
		Russian:  "ru",
		German:   "de",
		French:   "fr",
		Spanish:  "es",
	}[lang]
}

// youdaoSign 计算签名
func youdaoSign(appKey, input, salt, curtime, appSecret string) string {
	h := sha256.Sum256([]byte(appKey + input + salt + curtime + appSecret))
	return hex.EncodeToString(h[:])
}

// youdaoInput 按官方规则处理签名用原文：>20 字符取前 10 + 长度 + 后 10
func youdaoInput(q string) string {
	r := []rune(q)
	if len(r) > 20 {
		return string(r[:10]) + strconv.Itoa(len(r)) + string(r[len(r)-10:])
	}
	return q
}

type youdaoResp struct {
	ErrorCode   string   `json:"errorCode"`
	Translation []string `json:"translation"`
}

// YoudaoTranslate 调用有道智云翻译
func YoudaoTranslate(ctx context.Context, text, toLang string, p *Provider) (string, error) {
	if !p.youdaoValid() {
		return "", fmt.Errorf("youdao 未配置（translators.json 里 providers.youdao 需要 app_key/app_secret）")
	}
	to := youdaoLang(toLang)
	if to == "" {
		return "", fmt.Errorf("有道不支持目标语言 %s", toLang)
	}

	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	saltStr := hex.EncodeToString(salt)
	curtime := strconv.FormatInt(time.Now().Unix(), 10)
	input := youdaoInput(text)

	form := url.Values{
		"q":        {text},
		"from":     {"auto"},
		"to":       {to},
		"appKey":   {p.key("app_key")},
		"salt":     {saltStr},
		"curtime":  {curtime},
		"signType": {"v3"},
		"sign":     {youdaoSign(p.key("app_key"), input, saltStr, curtime, p.key("app_secret"))},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, youdaoAPI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("youdao 请求失败: %w", err)
	}
	defer resp.Body.Close()
	var yr youdaoResp
	if err := json.NewDecoder(resp.Body).Decode(&yr); err != nil {
		return "", fmt.Errorf("youdao 响应解析失败: %w", err)
	}
	// 有道成功时 errorCode 为 "0"（不是 200）
	if yr.ErrorCode != "" && yr.ErrorCode != "0" {
		return "", fmt.Errorf("youdao 错误码 %s（202=签名失败, 203=IP 不在白名单, 411=限频, 401=欠费）", yr.ErrorCode)
	}
	if len(yr.Translation) == 0 || yr.Translation[0] == "" {
		return "", fmt.Errorf("youdao 未返回译文")
	}
	return yr.Translation[0], nil
}
