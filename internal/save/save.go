package save

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
)

var baseDir string

// SetBase 设置下载根目录；不设置则默认 ~/Downloads/sync-email-boxing
func SetBase(p string) {
	if p != "" {
		baseDir = p
	}
}

// Base 当前下载根目录
func Base() string {
	if baseDir != "" {
		return baseDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Downloads", "sync-email-boxing")
}

// EmailDir 某封邮件的下载目录：base/<邮箱>/<文件夹>-<uid>/
func EmailDir(email, mailbox string, uid uint32) (string, error) {
	dir := filepath.Join(Base(), slug(email), slug(mailbox)+"-"+itoa(uid))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Attachments 把附件写入 dir，自动重名处理。
// 若同名文件已存在且内容完全一致，则不重复写入（计入 existing），避免连按多次产生多份副本。
func Attachments(dir string, atts []mail.Attachment) (paths []string, existing int, err error) {
	for i, a := range atts {
		name := a.Name
		if name == "" || name == a.ContentType || strings.ContainsAny(name, "/\\") {
			name = fmt.Sprintf("attachment-%02d", i+1)
		}
		if p := filepath.Join(dir, name); sameContent(p, a.Data) {
			debug.Infof("附件已存在，跳过写入 %s", p)
			paths = append(paths, p)
			existing++
			continue
		}
		p := uniquePath(dir, name)
		if err := os.WriteFile(p, a.Data, 0o600); err != nil {
			debug.Errf("保存附件失败 %s: %v", p, err)
			return paths, existing, fmt.Errorf("保存 %s 失败: %w", p, err)
		}
		debug.Infof("附件已保存 %s (%d 字节)", p, len(a.Data))
		paths = append(paths, p)
	}
	return paths, existing, nil
}

// EML 把原始邮件保存为 .eml 文件（含全部附件），返回完整路径。
// 已存在同样内容的文件时直接复用，不重复写。
func EML(dir, subject string, uid uint32, raw []byte) (string, error) {
	name := fmt.Sprintf("%s-%s.eml", itoa(uid), slug(subject))
	if name == "-.eml" {
		name = fmt.Sprintf("%s.eml", itoa(uid))
	}
	if p := filepath.Join(dir, name); sameContent(p, raw) {
		debug.Infof("原始邮件已存在，复用 %s", p)
		return p, nil
	}
	p := uniquePath(dir, name)
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		return "", fmt.Errorf("保存 %s 失败: %w", p, err)
	}
	debug.Infof("原始邮件已保存 %s (%d 字节)", p, len(raw))
	return p, nil
}

// sameContent 判断 p 是否存在且内容与 data 完全一致
func sameContent(p string, data []byte) bool {
	st, err := os.Stat(p)
	if err != nil || st.Size() != int64(len(data)) {
		return false
	}
	old, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return string(old) == string(data)
}

// uniquePath 目标文件已存在时追加序号
func uniquePath(dir, name string) string {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err != nil {
		return p
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s(%d)%s", stem, i, ext))
		if _, err := os.Stat(cand); err != nil {
			return cand
		}
	}
}

// slug 把字符串转成安全的文件/目录名
func slug(s string) string {
	repl := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_", "\n", "_", "\r", "",
	)
	out := strings.TrimSpace(repl.Replace(s))
	if len(out) > 60 {
		out = out[:60]
	}
	if out == "" {
		out = "untitled"
	}
	return out
}

func itoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
