package mail

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"

	"sync-email-boxing/internal/config"
	"sync-email-boxing/internal/debug"
)

// Summary 收件箱邮件列表项
type Summary struct {
	UID     uint32 `json:"uid"`
	Subject string `json:"subject"`
	From    string `json:"from"`
	Date    string `json:"date"`
	Seen    bool   `json:"seen"`
}

// Folder 邮件文件夹
type Folder struct {
	Name     string `json:"name"`
	Messages uint32 `json:"messages"`
	Attrs    string `json:"attrs"`
}

const (
	dialTimeout   = 20 * time.Second
	DefaultFolder = "INBOX"
)

func dial(acct config.Account) (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", acct.IMAPServer, acct.IMAPPort)
	debug.Infof("IMAP 连接 %s (账号 %s)", addr, acct.Email)
	start := time.Now()

	dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}
	c, err := client.DialWithDialerTLS(dialer, addr, &tls.Config{ServerName: acct.IMAPServer})
	if err != nil {
		debug.Errf("IMAP 连接失败 %s (%s): %v", addr, debug.Since(start), err)
		return nil, fmt.Errorf("连接 IMAP %s 失败: %w", addr, err)
	}
	debug.Infof("IMAP 已连接 %s (%s)，开始登录", addr, debug.Since(start))

	c.Timeout = 60 * time.Second
	if err := c.Login(acct.Email, acct.AuthCode); err != nil {
		debug.Errf("IMAP 登录失败 %s: %v", acct.Email, err)
		c.Logout()
		return nil, fmt.Errorf("登录失败（检查邮箱与授权码）: %w", err)
	}
	debug.Infof("IMAP 登录成功 %s (%s)", acct.Email, debug.Since(start))

	if err := sendClientID(c); err != nil {
		debug.Errf("IMAP ID 失败: %v", err)
	}
	return c, nil
}

// sendClientID 发送 IMAP ID 命令。网易(163/126)要求客户端在 SELECT 前自报身份，
// 否则返回 "Unsafe Login"；其他服务器支持 ID 时发送也无副作用。
func sendClientID(c *client.Client) error {
	caps, err := c.Capability()
	if err != nil {
		return err
	}
	if !caps["ID"] {
		debug.Debugf("服务器不支持 ID 命令，跳过")
		return nil
	}
	cmd := &imap.Command{
		Name: "ID",
		Arguments: []interface{}{
			imap.RawString(`("name" "sync-email-boxing" "version" "1.0.0" "vendor" "sync-email-boxing" "support-email" "sync-email-boxing@localhost")`),
		},
	}
	status, err := c.Execute(cmd, nil)
	if err != nil {
		return err
	}
	if status != nil {
		debug.Infof("已发送 IMAP ID: %s %s", status.Type, status.Info)
	}
	return nil
}

func formatAddrs(addrs []*imap.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a.PersonalName != "" {
			parts = append(parts, decodeHeader(a.PersonalName))
		} else {
			parts = append(parts, a.MailboxName+"@"+a.HostName)
		}
	}
	return strings.Join(parts, ", ")
}

// ListFolders 列出账号下所有可选文件夹及邮件数
func ListFolders(acct config.Account) ([]Folder, error) {
	start := time.Now()
	c, err := dial(acct)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	ch := make(chan *imap.MailboxInfo, 100)
	done := make(chan error, 1)
	go func() { done <- c.List("", "*", ch) }()

	var out []Folder
	for mi := range ch {
		f := Folder{Name: mi.Name, Attrs: strings.Join(mi.Attributes, ",")}
		noSelect := false
		for _, a := range mi.Attributes {
			if strings.EqualFold(a, imap.NoSelectAttr) {
				noSelect = true
			}
		}
		if noSelect {
			continue
		}
		if st, err := c.Status(mi.Name, []imap.StatusItem{imap.StatusMessages}); err == nil && st != nil {
			f.Messages = st.Messages
		} else if err != nil {
			debug.Debugf("STATUS %s 失败: %v", mi.Name, err)
		}
		out = append(out, f)
	}
	if err := <-done; err != nil {
		debug.Errf("列出文件夹失败: %v", err)
		return nil, fmt.Errorf("列出文件夹失败: %w", err)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name == DefaultFolder {
			return true
		}
		if out[j].Name == DefaultFolder {
			return false
		}
		return out[i].Name < out[j].Name
	})
	debug.Infof("文件夹 %d 个，耗时 %s", len(out), debug.Since(start))
	for _, f := range out {
		debug.Infof("  文件夹 %q 共 %d 封 (%s)", f.Name, f.Messages, f.Attrs)
	}
	return out, nil
}

// ListMailbox 拉取指定文件夹最近 limit 封邮件（新邮件在前）
// 只取 ENVELOPE/FLAGS/UID，不拉正文，保证速度
func ListMailbox(acct config.Account, mailbox string, limit int) ([]Summary, error) {
	if mailbox == "" {
		mailbox = DefaultFolder
	}
	start := time.Now()
	c, err := dial(acct)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	status, err := c.Select(mailbox, true)
	if err != nil {
		debug.Errf("打开 %s 失败: %v", mailbox, err)
		return nil, fmt.Errorf("打开 %s 失败: %w", mailbox, err)
	}
	count := status.Messages
	debug.Infof("%s 共 %d 封，准备拉取最近 %d 封", mailbox, count, limit)
	if count == 0 {
		return []Summary{}, nil
	}
	begin := uint32(1)
	if count > uint32(limit) {
		begin = count - uint32(limit) + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(begin, count)

	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid}
	ch := make(chan *imap.Message, limit)
	done := make(chan error, 1)
	fetchStart := time.Now()
	go func() { done <- c.Fetch(seqset, items, ch) }()

	var out []Summary
	for m := range ch {
		s := Summary{UID: m.Uid}
		for _, f := range m.Flags {
			if f == imap.SeenFlag {
				s.Seen = true
			}
		}
		if m.Envelope != nil {
			s.Subject = decodeHeader(m.Envelope.Subject)
			s.From = formatAddrs(m.Envelope.From)
			if !m.Envelope.Date.IsZero() {
				s.Date = m.Envelope.Date.Local().Format("2006-01-02 15:04")
			}
		}
		out = append(out, s)
	}
	if err := <-done; err != nil {
		debug.Errf("拉取 %s 失败: %v", mailbox, err)
		return nil, fmt.Errorf("拉取邮件列表失败: %w", err)
	}
	// Fetch 按序号升序返回，倒转为新邮件在前
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	debug.Infof("%s 列表完成：%d 封，fetch 耗时 %s，总耗时 %s", mailbox, len(out), debug.Since(fetchStart), debug.Since(start))
	return out, nil
}

// FetchBody 按 UID 拉取并解析一封邮件的正文
func FetchBody(acct config.Account, mailbox string, uid uint32) (Body, error) {
	if mailbox == "" {
		mailbox = DefaultFolder
	}
	start := time.Now()
	debug.Infof("拉取正文 %s uid=%d", mailbox, uid)
	c, err := dial(acct)
	if err != nil {
		return Body{}, err
	}
	defer c.Logout()

	if _, err := c.Select(mailbox, true); err != nil {
		debug.Errf("打开 %s 失败: %v", mailbox, err)
		return Body{}, fmt.Errorf("打开 %s 失败: %w", mailbox, err)
	}
	uidset := new(imap.SeqSet)
	uidset.AddNum(uid)

	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{section.FetchItem()}
	ch := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(uidset, items, ch) }()

	m := <-ch
	if m == nil {
		if err := <-done; err != nil {
			debug.Errf("拉取正文失败 uid=%d: %v", uid, err)
			return Body{}, fmt.Errorf("拉取邮件正文失败: %w", err)
		}
		return Body{}, fmt.Errorf("未找到 UID %d 的邮件", uid)
	}
	r := m.GetBody(section)
	if r == nil {
		<-done
		return Body{}, fmt.Errorf("邮件 %d 正文为空", uid)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		<-done
		return Body{}, err
	}
	<-done
	body := ParseBody(raw)
	debug.Infof("正文完成 %s uid=%d：原始 %d 字节，文本 %d 字，HTML %d 字节，附件 %d 个，耗时 %s",
		mailbox, uid, len(raw), len([]rune(body.Text)), len(body.HTML), len(body.Attachments), debug.Since(start))
	return body, nil
}

// FetchRaw 按 UID 拉取原始 RFC822 邮件（导出 .eml 用，含全部附件）
func FetchRaw(acct config.Account, mailbox string, uid uint32) ([]byte, error) {
	if mailbox == "" {
		mailbox = DefaultFolder
	}
	start := time.Now()
	debug.Infof("拉取原始邮件 %s uid=%d", mailbox, uid)
	c, err := dial(acct)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	if _, err := c.Select(mailbox, true); err != nil {
		return nil, fmt.Errorf("打开 %s 失败: %w", mailbox, err)
	}
	uidset := new(imap.SeqSet)
	uidset.AddNum(uid)

	section := &imap.BodySectionName{Peek: true}
	ch := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(uidset, []imap.FetchItem{section.FetchItem()}, ch) }()

	m := <-ch
	if m == nil {
		if err := <-done; err != nil {
			return nil, fmt.Errorf("拉取原始邮件失败: %w", err)
		}
		return nil, fmt.Errorf("未找到 UID %d 的邮件", uid)
	}
	r := m.GetBody(section)
	if r == nil {
		<-done
		return nil, fmt.Errorf("邮件 %d 内容为空", uid)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		<-done
		return nil, err
	}
	<-done
	debug.Infof("原始邮件完成 uid=%d：%d 字节，耗时 %s", uid, len(raw), debug.Since(start))
	return raw, nil
}
