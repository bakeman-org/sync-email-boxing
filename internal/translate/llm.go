package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// llmLangName 目标语言码 → 提示词里的语言名（未收录的直接用语言码，LLM 也认识）
var llmLangName = map[string]string{
	Chinese:  "Simplified Chinese (简体中文)",
	"zh-TW":  "Traditional Chinese (繁體中文)",
	English:  "English",
	Japanese: "Japanese (日本語)",
	Korean:   "Korean (한국어)",
	Russian:  "Russian (Русский)",
	German:   "German (Deutsch)",
	French:   "French (Français)",
	Spanish:  "Spanish (Español)",
}

// LLMTranslate 用 OpenAI 兼容的 chat/completions 接口翻译（DeepSeek、豆包 Ark、
// 通义 DashScope 兼容模式、OpenAI、Ollama 等都适用）。
//
// Provider 约定：
//   - url    ：chat/completions 完整地址
//   - model  ：模型名（deepseek-chat、doubao-pro-32k、qwen-plus、...）
//   - keys.api_key（或顶层 app_key）：Bearer 密钥
//   - keys.system（可选）：自定义系统提示词，默认让模型只输出译文
func LLMTranslate(ctx context.Context, text, toLang string, p *Provider) (string, error) {
	if p == nil || p.URL == "" {
		return "", fmt.Errorf("llm 翻译源未配置 url")
	}
	if p.Model == "" {
		return "", fmt.Errorf("llm 翻译源未配置 model")
	}
	apiKey := p.key("api_key")
	if apiKey == "" {
		apiKey = p.key("app_key") // 宽容：也接受 app_key 字段
	}
	if apiKey == "" {
		return "", fmt.Errorf("llm 翻译源未配置 api_key")
	}

	lang := toLang
	if name, ok := llmLangName[toLang]; ok {
		lang = name
	}
	sys := p.key("system")
	if sys == "" {
		sys = "You are a professional email translator. Translate the user's text into " +
			lang + ". Reply with the translation ONLY — no explanations, no notes, no quotes, no markdown."
	}

	payload, err := json.Marshal(map[string]any{
		"model": p.Model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": text},
		},
		"temperature": 0.2,
		"stream":      false,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm 翻译源请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm 翻译源 HTTP %d: %.200s", resp.StatusCode, string(raw))
	}

	out := ""
	if g := gjson.GetBytes(raw, "choices.0.message.content"); g.Exists() {
		out = g.String()
	}
	if out == "" {
		return "", fmt.Errorf("llm 翻译源未返回内容: %.200s", string(raw))
	}
	return strings.TrimSpace(stripFences(out)), nil
}

// stripFences 去掉模型偶尔输出的 markdown 代码围栏与首尾引号
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```")
	// 去掉可能的语言标注行
	if i := strings.IndexByte(s, '\n'); i >= 0 && !strings.HasPrefix(s, "\n") {
		first := strings.TrimSpace(s[:i])
		if first != "" && !strings.ContainsAny(first, "。！？;，, ") && len(first) < 12 {
			s = s[i+1:]
		}
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || strings.HasPrefix(s, "“")) {
		s = s[1:]
	}
	if len(s) >= 2 && (s[len(s)-1] == '"' || strings.HasSuffix(s, "”")) {
		s = s[:len(s)-1]
	}
	return strings.TrimSpace(s)
}
