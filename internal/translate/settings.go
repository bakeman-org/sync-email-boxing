package translate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Settings 翻译设置，独立保存在 translators.json，与账号配置 config.json 解耦。
// 这样后续加多少翻译源都不会把 config.json 撑大。
type Settings struct {
	Active    string               `json:"active,omitempty"`    // 首选翻译源名，空 = sogou
	Target    string               `json:"target,omitempty"`    // 目标语言，空 = zh-CN
	Providers map[string]*Provider `json:"providers,omitempty"` // 翻译源定义：名字 -> 配置
}

// Provider 一个翻译源。
// type 决定用哪套实现：
//   - sogou  ：内置（无需任何密钥）
//   - youdao ：内置有道智云（app_key/app_secret）
//   - llm    ：OpenAI 兼容 chat/completions（DeepSeek、豆包、通义、Ollama...）
//   - http   ：通用 HTTP API，用模板拼请求（见 http.go），可配任意密钥
type Provider struct {
	Type       string            `json:"type,omitempty"`
	Model      string            `json:"model,omitempty"` // llm 源：模型名（deepseek-chat / qwen-plus / ...）
	AppKey     string            `json:"app_key,omitempty"`
	AppSecret  string            `json:"app_secret,omitempty"`
	Keys       map[string]string `json:"keys,omitempty"` // 任意凭据，模板里 {{.Keys.xxx}} 取
	URL        string            `json:"url,omitempty"`
	Method     string            `json:"method,omitempty"` // 默认 POST
	Headers    map[string]string `json:"headers,omitempty"`
	Query      map[string]string `json:"query,omitempty"`
	Body       string            `json:"body,omitempty"`
	ResultPath string            `json:"result_path,omitempty"` // gjson 路径，空 = 整个响应当译文
}

// key 读取凭据：Keys 优先，app_key/app_secret 兼容顶层字段
func (p *Provider) key(name string) string {
	if p == nil {
		return ""
	}
	if v := p.Keys[name]; v != "" {
		return v
	}
	switch name {
	case "app_key":
		return p.AppKey
	case "app_secret":
		return p.AppSecret
	}
	return ""
}

// kind 该源使用的实现类型；未写 type 时按名字推断
func (p *Provider) kind(name string) string {
	if p != nil && p.Type != "" {
		return p.Type
	}
	switch name {
	case "sogou", "youdao":
		return name
	}
	return "http"
}

// ActiveName 首选翻译源名（默认 sogou）
func (s *Settings) ActiveName() string {
	if s != nil && s.Active != "" {
		return s.Active
	}
	return DefaultSource
}

// TargetOrDefault 目标语言（默认中文）
func (s *Settings) TargetOrDefault() string {
	if s != nil && s.Target != "" {
		return s.Target
	}
	return DefaultTarget
}

// Provider 取翻译源配置（内置 sogou 可以没有配置）
func (s *Settings) Provider(name string) *Provider {
	if s == nil || s.Providers == nil {
		return nil
	}
	return s.Providers[name]
}

// SettingsPath translators.json 路径：$XDG_CONFIG_HOME/sync-email-boxing/translators.json
func SettingsPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "sync-email-boxing", "translators.json")
}

// LoadSettings 读取翻译设置，不存在时返回默认（sogou）
func LoadSettings() *Settings {
	data, err := os.ReadFile(SettingsPath())
	if err != nil {
		return &Settings{}
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return &Settings{}
	}
	if s.Providers == nil {
		s.Providers = map[string]*Provider{}
	}
	return &s
}

// Save 写入 translators.json（目录 0700，文件 0600 保护密钥）
func (s *Settings) Save() error {
	p := SettingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o600)
}

// MigrateLegacy 把旧版塞在 config.json 里的 translate_* 字段迁移成 provider。
// customJSON / youdaoJSON 为原始 JSON（可为空）。
func MigrateLegacy(s *Settings, source, target string, customJSON, youdaoJSON []byte) *Settings {
	if s == nil {
		s = &Settings{}
	}
	if s.Providers == nil {
		s.Providers = map[string]*Provider{}
	}

	if len(youdaoJSON) > 0 {
		var y struct {
			AppKey    string `json:"app_key"`
			AppSecret string `json:"app_secret"`
		}
		if json.Unmarshal(youdaoJSON, &y) == nil && y.AppKey != "" && y.AppSecret != "" {
			if _, ok := s.Providers["youdao"]; !ok {
				s.Providers["youdao"] = &Provider{Type: "youdao", AppKey: y.AppKey, AppSecret: y.AppSecret}
			}
		}
	}

	if len(customJSON) > 0 {
		var c struct {
			Name       string            `json:"name"`
			APIURL     string            `json:"api_url"`
			APIKey     string            `json:"api_key"`
			Method     string            `json:"method"`
			Headers    map[string]string `json:"headers"`
			Query      map[string]string `json:"query"`
			Body       string            `json:"body"`
			ResultPath string            `json:"result_path"`
		}
		if json.Unmarshal(customJSON, &c) == nil && c.APIURL != "" {
			name := c.Name
			if name == "" || name == "custom" {
				name = "custom"
			}
			if _, ok := s.Providers[name]; !ok {
				p := &Provider{
					Type: "http", URL: c.APIURL, Method: c.Method,
					Headers: c.Headers, Query: c.Query, Body: c.Body, ResultPath: c.ResultPath,
				}
				if c.APIKey != "" {
					p.Keys = map[string]string{"api_key": c.APIKey}
				}
				s.Providers[name] = p
			}
			if source == "" || source == "custom" {
				source = name
			}
		}
	}

	if s.Target == "" {
		s.Target = target
	}
	if s.Active == "" {
		switch {
		case source != "":
			s.Active = source
		case len(s.Providers) == 1:
			for name := range s.Providers {
				s.Active = name
			}
		default:
			s.Active = DefaultSource
		}
	}
	return s
}

// Builtin 返回内置源列表（供文档/自我检查用）
func Builtin() []string {
	return []string{"sogou", "youdao"}
}

// AvailableSources 已配置可用（或内置无需凭据）的翻译源，顺序稳定：sogou → youdao → 自定义（按名字排序）
func (s *Settings) AvailableSources() []string {
	names := []string{"sogou"}
	if s != nil {
		// 内置 youdao：配了密钥才可切换
		if p := s.Provider("youdao"); p != nil && p.key("app_key") != "" && p.key("app_secret") != "" {
			names = append(names, "youdao")
		}
		var custom []string
		for name, p := range s.Providers {
			if p == nil || name == "sogou" || name == "youdao" {
				continue
			}
			if (p.kind(name) == "http" || p.kind(name) == "llm") && p.URL != "" {
				custom = append(custom, name)
			}
		}
		sort.Strings(custom)
		names = append(names, custom...)
	}
	return names
}

// NextSource 循环切换到下一个可用翻译源；只有 sogou 可用时返回当前值
func (s *Settings) NextSource() string {
	names := s.AvailableSources()
	cur := s.ActiveName()
	idx := 0
	for i, n := range names {
		if n == cur {
			idx = i
			break
		}
	}
	return names[(idx+1)%len(names)]
}
