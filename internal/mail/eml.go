package mail

import (
	"bytes"
	"fmt"
	"net/mail"
	"os"
	"strings"
)

// ParseEML 解析原始 .eml 字节，返回摘要（已解码中文）与正文/附件
func ParseEML(raw []byte) (Summary, Body) {
	var s Summary
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return Summary{Subject: "(无法解析的邮件)"}, ParseBody(raw)
	}
	s.Subject = decodeHeader(msg.Header.Get("Subject"))
	s.From = decodeHeader(msg.Header.Get("From"))
	s.Date = decodeHeader(msg.Header.Get("Date"))
	s.Seen = true
	return s, ParseBody(raw)
}

// ParseEMLFile 读取并解析 .eml 文件
func ParseEMLFile(path string) (Summary, Body, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, Body{}, nil, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	s, b := ParseEML(raw)
	return s, b, raw, nil
}

// EMLFullHeaders 生成完整头信息文本（解码 RFC2047，常见头逐行列出）
func EMLFullHeaders(raw []byte) string {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return "(无法解析头信息)"
	}
	ordered := []string{
		"From", "To", "Cc", "Bcc", "Subject", "Date",
		"Message-ID", "In-Reply-To", "References",
		"Content-Type", "MIME-Version", "X-Mailer", "User-Agent",
	}
	seen := map[string]bool{}
	var lines []string
	add := func(name string) {
		if v := msg.Header.Get(name); v != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", name, decodeHeader(v)))
		}
		seen[name] = true
	}
	for _, h := range ordered {
		add(h)
	}
	for name := range msg.Header {
		if !seen[name] {
			add(name)
		}
	}
	return strings.Join(lines, "\n")
}

// ExtractAddress 从 "Name <a@b>" 头中提取纯邮箱地址
func ExtractAddress(header string) string {
	addr, err := mail.ParseAddress(decodeHeader(header))
	if err != nil {
		return header
	}
	return addr.Address
}
