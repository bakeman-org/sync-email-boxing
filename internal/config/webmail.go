package config

import "strings"

// WebmailURL 根据账号邮箱推导对应的网页版邮箱地址，方便粘贴到浏览器打开
func (a Account) WebmailURL() string {
	domain := a.Email
	if i := strings.Index(a.Email, "@"); i >= 0 {
		domain = a.Email[i+1:]
	}
	switch strings.ToLower(domain) {
	case "qq.com", "foxmail.com":
		return "https://mail.qq.com"
	case "163.com":
		return "https://mail.163.com"
	case "126.com":
		return "https://mail.126.com"
	case "gmail.com":
		return "https://mail.google.com"
	case "outlook.com", "hotmail.com", "live.com":
		return "https://outlook.live.com/mail"
	default:
		return "https://mail." + domain
	}
}
