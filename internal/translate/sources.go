package translate

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	sogouSalt     = "109984457"
	sogouUA       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.77 Safari/537.36"
	sogouHomepage = "https://fanyi.sogou.com/"
	sogouAPI      = "https://fanyi.sogou.com/api/transpc/text/result"
)

// sogouSign 复刻 go-fy 用 otto 执行的 signJS：等价于 md5(from+to+text+salt) 的 hex。
// from 固定 "auto"，to 已转换为 sogou 的语言码（如 zh-CHS）
func sogouSign(from, to, text string) string {
	sum := md5.Sum([]byte(from + to + text + sogouSalt))
	return hex.EncodeToString(sum[:])
}

// sogouConvert go-fy 的 sogou 语言码映射
func sogouConvert(lang string) string {
	if lang == Chinese {
		return "zh-CHS"
	}
	return lang
}

type sogouResp struct {
	Code int `json:"code"`
	Data struct {
		Translate struct {
			Dit  string `json:"dit"`
			Code int    `json:"code"`
		} `json:"translate"`
	} `json:"data"`
}

// sogouTranslate 搜狗翻译（需要先 GET 首页拿 cookie 再调 API）
func sogouTranslate(ctx context.Context, text, toLang string) (string, error) {
	client := &http.Client{Timeout: perSourceTimeo}

	// 1. 首页拿 cookie
	home, err := http.NewRequestWithContext(ctx, http.MethodGet, sogouHomepage, nil)
	if err != nil {
		return "", err
	}
	home.Header.Set("User-Agent", sogouUA)
	hresp, err := client.Do(home)
	if err != nil {
		return "", fmt.Errorf("获取 sogou cookie 失败: %w", err)
	}
	var cookies []*http.Cookie
	for _, c := range hresp.Cookies() {
		cookies = append(cookies, c)
	}
	io.Copy(io.Discard, hresp.Body)
	hresp.Body.Close()

	// 2. 翻译 API
	to := sogouConvert(toLang)
	param, _ := json.Marshal(map[string]any{
		"client":   "pc",
		"exchange": false,
		"fr":       "browser_pc",
		"needQc":   1,
		"s":        sogouSign("auto", to, text),
		"text":     text,
		"to":       to,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sogouAPI, bytes.NewReader(param))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", sogouUA)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sogou 请求失败: %w", err)
	}
	defer resp.Body.Close()
	var sr sogouResp
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("sogou 响应解析失败: %w", err)
	}
	dit := sr.Data.Translate.Dit
	if dit == "" {
		return "", fmt.Errorf("sogou 未返回译文 (code=%d)", sr.Code)
	}
	return dit, nil
}
