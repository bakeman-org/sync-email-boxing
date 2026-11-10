// Package translate 邮件正文翻译。
//
// 内置源：
//   - sogou  ：纯 Go，无需密钥（sogou 实现移植自 go-fy，原版用 otto 跑 JS 算签名，
//     实际就是 md5(from+to+text+salt) 的 hex）
//   - youdao ：有道智云官方 API，需 app_key/app_secret
//
// 通用源：
//   - http   ：配置 url/headers/body 模板 + 任意 keys，支持 md5/sha256/hmac 等
//     模板函数，足以描述国内各家需要签名的翻译 API（见 http.go）
//
// 设置存放在独立的 translators.json，不写入 config.json。
package translate

import (
	"context"
	"fmt"
	"time"
)

// 语言常量（与 go-fy 的 fy_support_source 保持一致的取值）
const (
	Chinese  = "zh-CN"
	English  = "en"
	Japanese = "ja"
	Korean   = "ko"
	Russian  = "ru"
	German   = "de"
	French   = "fr"
	Spanish  = "es"

	DefaultTarget  = Chinese
	DefaultSource  = "sogou"
	perSourceTimeo = 15 * time.Second
)

// Translate 把 text 翻译成 toLang，按 settings.Active 选择首选源，失败回退 sogou。
// 返回译文与实际使用的翻译源名。
func Translate(text, toLang string, s *Settings) (string, string, error) {
	if s == nil {
		s = &Settings{}
	}
	if toLang == "" {
		toLang = s.TargetOrDefault()
	}

	active := s.ActiveName()
	var order []string
	if active != DefaultSource {
		order = append(order, active)
	}
	order = append(order, DefaultSource)

	var lastErr error
	for _, name := range order {
		p := s.Provider(name)
		ctx, cancel := context.WithTimeout(context.Background(), perSourceTimeo)
		var res string
		var err error
		switch p.kind(name) {
		case "sogou":
			res, err = sogouTranslate(ctx, text, toLang)
		case "youdao":
			res, err = YoudaoTranslate(ctx, text, toLang, p)
		case "llm":
			res, err = LLMTranslate(ctx, text, toLang, p)
		case "http":
			res, err = HTTPTranslate(ctx, text, toLang, p)
		default:
			err = fmt.Errorf("未知翻译源类型: %s", p.kind(name))
		}
		cancel()
		if err == nil {
			return res, name, nil
		}
		lastErr = err
	}
	return "", "", fmt.Errorf("全部翻译源失败，最后错误: %w", lastErr)
}

// IsChinese 文本是否包含汉字（用于跳过已是目标语言的正文）
func IsChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}
