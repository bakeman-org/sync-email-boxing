package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/config"
	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
)

const (
	fName = iota
	fEmail
	fAuth
	fSMTPHost
	fSMTPPort
	fIMAPHost
	fIMAPPort
	fieldCount
)

type formModel struct {
	inputs []textinput.Model
	focus  int
}

func newForm() formModel {
	labels := []struct{ ph string; pw int }{
		{"昵称（可选）", 24},
		{"邮箱地址，如 user@qq.com", 32},
		{"授权码（不是登录密码）", 32},
		{"SMTP 服务器", 24},
		{"SMTP 端口", 8},
		{"IMAP 服务器", 24},
		{"IMAP 端口", 8},
	}
	inputs := make([]textinput.Model, fieldCount)
	for i, l := range labels {
		in := textinput.New()
		in.Placeholder = l.ph
		in.Width = l.pw
		inputs[i] = in
	}
	inputs[fAuth].EchoMode = textinput.EchoPassword
	inputs[fAuth].EchoCharacter = '*'
	inputs[fSMTPPort].Placeholder = "465"
	inputs[fIMAPPort].Placeholder = "993"
	inputs[0].Focus()
	return formModel{inputs: inputs}
}

// applyDefaults 邮箱填完后自动补全服务器配置
func (f *formModel) applyDefaults() {
	email := strings.TrimSpace(f.inputs[fEmail].Value())
	if email == "" || !strings.Contains(email, "@") {
		return
	}
	sh, sp, ih, ip := config.DefaultServers(email)
	if sh != "" {
		if strings.TrimSpace(f.inputs[fSMTPHost].Value()) == "" {
			f.inputs[fSMTPHost].SetValue(sh)
		}
		if strings.TrimSpace(f.inputs[fIMAPHost].Value()) == "" {
			f.inputs[fIMAPHost].SetValue(ih)
		}
		if strings.TrimSpace(f.inputs[fSMTPPort].Value()) == "" {
			f.inputs[fSMTPPort].SetValue(strconv.Itoa(sp))
		}
		if strings.TrimSpace(f.inputs[fIMAPPort].Value()) == "" {
			f.inputs[fIMAPPort].SetValue(strconv.Itoa(ip))
		}
	}
}

func (f *formModel) moveFocus(delta int) {
	f.inputs[f.focus].Blur()
	f.focus = (f.focus + delta + fieldCount) % fieldCount
	if f.focus == fSMTPHost {
		f.applyDefaults()
	}
	f.inputs[f.focus].Focus()
}

func (f *formModel) save() (config.Account, error) {
	var a config.Account
	a.Name = strings.TrimSpace(f.inputs[fName].Value())
	a.Email = strings.TrimSpace(f.inputs[fEmail].Value())
	a.AuthCode = strings.TrimSpace(f.inputs[fAuth].Value())
	a.SMTPServer = strings.TrimSpace(f.inputs[fSMTPHost].Value())
	a.IMAPServer = strings.TrimSpace(f.inputs[fIMAPHost].Value())
	if a.SMTPServer == "" || a.IMAPServer == "" {
		f.applyDefaults()
		a.SMTPServer = strings.TrimSpace(f.inputs[fSMTPHost].Value())
		a.IMAPServer = strings.TrimSpace(f.inputs[fIMAPHost].Value())
	}
	a.SMTPPort = parsePort(f.inputs[fSMTPPort].Value(), 465)
	a.IMAPPort = parsePort(f.inputs[fIMAPPort].Value(), 993)
	if a.Email == "" || !strings.Contains(a.Email, "@") {
		return a, errInvalid("邮箱地址无效")
	}
	if a.AuthCode == "" {
		return a, errInvalid("授权码不能为空")
	}
	if a.SMTPServer == "" || a.IMAPServer == "" {
		return a, errInvalid("请填写 SMTP/IMAP 服务器地址")
	}
	if a.Name == "" {
		a.Name = a.Email
	}
	return a, nil
}

func parsePort(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 || n > 65535 {
		return def
	}
	return n
}

type errInvalid string

func (e errInvalid) Error() string { return string(e) }

// updateForm 处理表单模式的按键
func updateForm(m *Model, msg tea.KeyMsg) tea.Cmd {
	f := &m.form
	switch msg.String() {
	case "esc":
		m.errMsg = ""
		if len(m.cfg.Accounts) > 0 {
			m.mode = modeList
		} else {
			m.mode = modeAccounts
		}
		return nil
	case "tab", "down":
		f.moveFocus(1)
		return nil
	case "shift+tab", "up":
		f.moveFocus(-1)
		return nil
	case "enter":
		if f.focus != fieldCount-1 {
			f.moveFocus(1)
			return nil
		}
		acct, err := f.save()
		if err != nil {
			return m.setError(err.Error())
		}
		m.cfg.Accounts = append(m.cfg.Accounts, acct)
		m.cfg.Current = len(m.cfg.Accounts) - 1
		if err := config.Save(m.cfg); err != nil {
			return m.setError("保存配置失败: " + err.Error())
		}
		m.mode = modeList
		m.msgs = nil
		m.cursor = 0
		m.cache = map[uint32]mail.Body{}
		m.loading = true
		m.folder = mail.DefaultFolder
		m.folders = nil
		m.folderIdx = 0
		debug.Infof("新增账号 %s (smtp %s:%d, imap %s:%d)", acct.Email, acct.SMTPServer, acct.SMTPPort, acct.IMAPServer, acct.IMAPPort)
		return tea.Batch(m.setStatus("已添加账号 "+acct.Email+"，正在首次同步…"), fetchInboxCmd(acct, m.folder))
	}
	var cmd tea.Cmd
	m.form.inputs[m.form.focus], cmd = m.form.inputs[m.form.focus].Update(msg)
	return cmd
}

func renderForm(m Model) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("添加邮箱账号") + "\n\n")
	fields := []string{"昵称", "邮箱", "授权码", "SMTP 服务器", "SMTP 端口", "IMAP 服务器", "IMAP 端口"}
	for i, name := range fields {
		cursor := "  "
		if i == m.form.focus {
			cursor = "> "
		}
		sb.WriteString(cursor + labelStyle.Render(name) + m.form.inputs[i].View() + "\n")
	}
	sb.WriteString("\n" + helpStyle.Render(helpBar(
		k("Tab/Shift+Tab", "切换字段"),
		k("Enter", "下一项/保存"),
		k("Esc", "取消"),
	)))
	sb.WriteString("\n" + renderEcho(m))
	return sb.String()
}
