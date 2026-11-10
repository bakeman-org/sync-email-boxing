package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Account 一个邮箱账号（QQ/163 等，使用授权码）
type Account struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	AuthCode   string `json:"auth_code"`
	SMTPServer string `json:"smtp_server"`
	SMTPPort   int    `json:"smtp_port"`
	IMAPServer string `json:"imap_server"`
	IMAPPort   int    `json:"imap_port"`
}

// Config 配置文件结构，支持多账号与当前账号切换
type Config struct {
	Accounts []Account `json:"accounts"`
	Current  int       `json:"current"`
}

// Path 配置文件路径：$XDG_CONFIG_HOME/sync-email-boxing/config.json
func Path() string {
	base, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "sync-email-boxing", "config.json")
}

// Load 读取配置，文件不存在或损坏时返回空配置
func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if c.Current < 0 || c.Current >= len(c.Accounts) {
		c.Current = 0
	}
	return &c, nil
}

// Save 写入配置（目录 0700，文件 0600 保护授权码）
func Save(c *Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o600)
}

// CurrentAccount 返回当前选中账号
func (c *Config) CurrentAccount() (*Account, error) {
	if len(c.Accounts) == 0 {
		return nil, errors.New("还没有账号，按 a 添加账号")
	}
	if c.Current < 0 || c.Current >= len(c.Accounts) {
		return nil, errors.New("当前账号下标越界")
	}
	return &c.Accounts[c.Current], nil
}

// DefaultServers 根据邮箱域名给出常见服务商默认服务器
func DefaultServers(email string) (smtpHost string, smtpPort int, imapHost string, imapPort int) {
	domain := email
	if i := strings.LastIndex(email, "@"); i >= 0 {
		domain = email[i+1:]
	}
	switch strings.ToLower(strings.TrimSpace(domain)) {
	case "qq.com", "foxmail.com":
		return "smtp.qq.com", 465, "imap.qq.com", 993
	case "163.com":
		return "smtp.163.com", 465, "imap.163.com", 993
	case "126.com":
		return "smtp.126.com", 465, "imap.126.com", 993
	case "gmail.com":
		return "smtp.gmail.com", 465, "imap.gmail.com", 993
	default:
		return "", 465, "", 993
	}
}
