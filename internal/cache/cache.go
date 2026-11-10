package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"sync-email-boxing/internal/config"
	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
)

// Store 邮件本地缓存：各文件夹的邮件列表 + 已读过的正文
type Store struct {
	root string
}

// Default 缓存根目录：~/.config/sync-email-boxing/cache
func Default() *Store {
	return &Store{root: filepath.Join(filepath.Dir(config.Path()), "cache")}
}

// Path 缓存根目录
func (s *Store) Path() string { return s.root }

func hashKey(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])[:10]
}

func (s *Store) dir(email string) string {
	return filepath.Join(s.root, hashKey(email))
}

func (s *Store) folderDir(email, mailbox string) string {
	if mailbox == "" || mailbox == mail.DefaultFolder {
		return s.dir(email) // 兼容旧版 INBOX 缓存路径
	}
	return filepath.Join(s.dir(email), "box-"+hashKey(mailbox))
}

type inboxFile struct {
	Email    string         `json:"email"`
	Mailbox  string         `json:"mailbox"`
	SyncedAt time.Time      `json:"synced_at"`
	Messages []mail.Summary `json:"messages"`
}

// LoadInbox 读取缓存的某个文件夹列表
func (s *Store) LoadInbox(email, mailbox string) ([]mail.Summary, time.Time, bool) {
	if mailbox == "" {
		mailbox = mail.DefaultFolder
	}
	p := filepath.Join(s.folderDir(email, mailbox), "inbox.json")
	data, err := os.ReadFile(p)
	if err != nil {
		debug.Debugf("缓存未命中 %s/%s (%v)", email, mailbox, err)
		return nil, time.Time{}, false
	}
	var f inboxFile
	if err := json.Unmarshal(data, &f); err != nil {
		debug.Errf("缓存损坏 %s: %v", p, err)
		return nil, time.Time{}, false
	}
	debug.Infof("缓存命中 %s/%s: %d 封，同步于 %s", email, mailbox, len(f.Messages), f.SyncedAt.Local().Format("2006-01-02 15:04:05"))
	return f.Messages, f.SyncedAt, true
}

// SaveInbox 写入某个文件夹的邮件列表缓存
func (s *Store) SaveInbox(email, mailbox string, msgs []mail.Summary) error {
	if mailbox == "" {
		mailbox = mail.DefaultFolder
	}
	p := filepath.Join(s.folderDir(email, mailbox), "inbox.json")
	f := inboxFile{Email: email, Mailbox: mailbox, SyncedAt: time.Now(), Messages: msgs}
	if err := writeJSON(p, f); err != nil {
		debug.Errf("写缓存失败 %s: %v", p, err)
		return err
	}
	debug.Infof("缓存已写入 %s/%s: %d 封", email, mailbox, len(msgs))
	return nil
}

// LoadBody 读取缓存的邮件正文
func (s *Store) LoadBody(email, mailbox string, uid uint32) (mail.Body, bool) {
	if mailbox == "" {
		mailbox = mail.DefaultFolder
	}
	p := filepath.Join(s.folderDir(email, mailbox), "bodies", itoa(uid)+".json")
	data, err := os.ReadFile(p)
	if err != nil {
		return mail.Body{}, false
	}
	var b mail.Body
	if err := json.Unmarshal(data, &b); err != nil {
		debug.Errf("缓存损坏 body %s: %v", p, err)
		return mail.Body{}, false
	}
	debug.Infof("缓存命中 body %s/%s uid=%d", email, mailbox, uid)
	return b, true
}

// SaveBody 写入邮件正文缓存
func (s *Store) SaveBody(email, mailbox string, uid uint32, b mail.Body) error {
	if mailbox == "" {
		mailbox = mail.DefaultFolder
	}
	p := filepath.Join(s.folderDir(email, mailbox), "bodies", itoa(uid)+".json")
	if err := writeJSON(p, b); err != nil {
		debug.Errf("写缓存失败 %s: %v", p, err)
		return err
	}
	debug.Debugf("缓存已写入 body %s/%s uid=%d", email, mailbox, uid)
	return nil
}

// Clear 清空指定账号缓存
func (s *Store) Clear(email string) error {
	debug.Infof("清空缓存 %s", email)
	return os.RemoveAll(s.dir(email))
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
