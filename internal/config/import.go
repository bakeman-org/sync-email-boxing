package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// externalSchema 兼容 go-sendemail 仓库的配置格式
// （sender_email/sender_key/smtp_server/smtp_port，见 163_email_config.json / qq_email_config.json）
type externalSchema struct {
	SenderEmail string `json:"sender_email"`
	SenderKey   string `json:"sender_key"`
	SMTPServer  string `json:"smtp_server"`
	SMTPPort    int    `json:"smtp_port"`
	// 兼容本项目的完整格式字段（可选）
	Name       string `json:"name"`
	Email      string `json:"email"`
	AuthCode   string `json:"auth_code"`
	IMAPServer string `json:"imap_server"`
	IMAPPort   int    `json:"imap_port"`
}

// LoadExternalFile 解析外部配置文件，转换为 Account
func LoadExternalFile(path string) (Account, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Account{}, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	var ext externalSchema
	if err := json.Unmarshal(data, &ext); err != nil {
		return Account{}, fmt.Errorf("解析 %s 失败: %w", path, err)
	}

	var a Account
	if ext.Email != "" {
		a.Email = ext.Email
		a.AuthCode = ext.AuthCode
		a.Name = ext.Name
		a.IMAPServer = ext.IMAPServer
		a.IMAPPort = ext.IMAPPort
	} else if ext.SenderEmail != "" {
		a.Email = ext.SenderEmail
		a.AuthCode = ext.SenderKey
		a.SMTPServer = ext.SMTPServer
		a.SMTPPort = ext.SMTPPort
	} else {
		return Account{}, fmt.Errorf("%s 中没有账号信息", path)
	}

	if a.Name == "" {
		a.Name = a.Email
	}
	// 外部文件通常只写 SMTP，IMAP 按域名推导
	sh, sp, ih, ip := DefaultServers(a.Email)
	if a.SMTPServer == "" {
		a.SMTPServer = sh
	}
	if a.SMTPPort == 0 {
		a.SMTPPort = sp
	}
	if a.IMAPServer == "" {
		a.IMAPServer = ih
	}
	if a.IMAPPort == 0 {
		a.IMAPPort = ip
	}
	if a.SMTPServer == "" || a.IMAPServer == "" {
		return Account{}, fmt.Errorf("%s: 无法确定 %s 的服务器地址", path, a.Email)
	}
	return a, nil
}

// ImportResult 导入结果
type ImportResult struct {
	Added   int      // 新增账号数
	Updated int      // 覆盖更新的账号数
	Emails  []string // 本次处理的账号，最后一个为当前账号
	Current int      // 当前账号下标
}

// ImportFiles 把外部配置写入长期存储：
// 已存在的账号会被覆盖更新，最后指定的文件对应的账号成为当前账号
func ImportFiles(c *Config, paths []string) (ImportResult, error) {
	var res ImportResult
	// 先全部解析成功再写入，避免中途失败留下半套配置
	parsed := make([]Account, 0, len(paths))
	for _, p := range paths {
		acct, err := LoadExternalFile(p)
		if err != nil {
			return res, err
		}
		parsed = append(parsed, acct)
	}
	for _, acct := range parsed {
		idx := c.IndexOfEmail(acct.Email)
		if idx >= 0 {
			// 保留用户自定义昵称（如果外部文件没写）
			if acct.Name == acct.Email && c.Accounts[idx].Name != "" {
				acct.Name = c.Accounts[idx].Name
			}
			c.Accounts[idx] = acct
			res.Updated++
			res.Current = idx
		} else {
			c.Accounts = append(c.Accounts, acct)
			res.Added++
			res.Current = len(c.Accounts) - 1
		}
		res.Emails = append(res.Emails, acct.Email)
	}
	c.Current = res.Current
	if err := Save(c); err != nil {
		return res, err
	}
	return res, nil
}

// IndexOfEmail 查找账号下标，不存在返回 -1
func (c *Config) IndexOfEmail(email string) int {
	for i, a := range c.Accounts {
		if strings.EqualFold(a.Email, email) {
			return i
		}
	}
	return -1
}
