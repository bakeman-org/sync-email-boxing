package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/clipboard"
	"sync-email-boxing/internal/config"
	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
	"sync-email-boxing/internal/save"
)

type viewerModel struct {
	vp      viewport.Model
	ready   bool
	loading bool
	summary mail.Summary
	body    mail.Body
}

func (v *viewerModel) open(s mail.Summary, body mail.Body) {
	v.summary = s
	v.body = body
	content := mail.RenderHeaders(s.From, s.Subject, s.Date, body) + "\n\n" + body.Text
	v.vp.GotoTop()
	v.vp.SetContent(content)
	v.ready = true
}

type savedMsg struct {
	paths    []string
	existing int // 已存在且内容一致、未重复写入的数量
	err      error
}

func saveAttachmentsCmd(base, email, mailbox string, uid uint32, body mail.Body) tea.Cmd {
	return func() tea.Msg {
		dir, err := save.EmailDir(email, mailbox, uid)
		if err != nil {
			return savedMsg{err: err}
		}
		paths, existing, err := save.Attachments(dir, body.Attachments)
		return savedMsg{paths: paths, existing: existing, err: err}
	}
}

func saveEmlCmd(a config.Account, mailbox string, uid uint32, subject string) tea.Cmd {
	return func() tea.Msg {
		raw, err := mail.FetchRaw(a, mailbox, uid)
		if err != nil {
			return savedMsg{err: err}
		}
		dir, err := save.EmailDir(a.Email, mailbox, uid)
		if err != nil {
			return savedMsg{err: err}
		}
		p, err := save.EML(dir, subject, uid, raw)
		if err != nil {
			return savedMsg{err: err}
		}
		return savedMsg{paths: []string{p}}
	}
}

// updateViewer 正文查看模式按键
func updateViewer(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeList
		return nil
	case "up", "k":
		m.viewer.vp.LineUp(1)
		return nil
	case "down", "j":
		m.viewer.vp.LineDown(1)
		return nil
	case "pgup":
		m.viewer.vp.HalfViewUp()
		return nil
	case "pgdown", " ":
		m.viewer.vp.HalfViewDown()
		return nil
	case "g":
		m.viewer.vp.GotoTop()
		return nil
	case "G":
		m.viewer.vp.GotoBottom()
		return nil
	case "c":
		if err := clipboard.Copy(m.viewer.body.Text); err != nil {
			return m.setError("复制失败: " + err.Error())
		}
		debug.Infof("已复制正文到剪贴板 (%d 字)", len([]rune(m.viewer.body.Text)))
		return m.setStatus(fmt.Sprintf("正文已复制到剪贴板（%d 字）", len([]rune(m.viewer.body.Text))))
	case "y":
		a := m.account()
		if a == nil {
			return nil
		}
		url := a.WebmailURL()
		if err := clipboard.Copy(url); err != nil {
			return m.setError("复制失败: " + err.Error())
		}
		debug.Infof("已复制网页邮箱链接 %s", url)
		return m.setStatus("网页邮箱链接已复制: " + url)
	case "d":
		if len(m.viewer.body.Attachments) == 0 {
			return m.setStatus("该邮件没有附件")
		}
		a := m.account()
		if a == nil {
			return nil
		}
		return tea.Batch(
			m.setStatus(fmt.Sprintf("正在保存 %d 个附件…", len(m.viewer.body.Attachments))),
			saveAttachmentsCmd(save.Base(), a.Email, m.folder, m.viewer.summary.UID, m.viewer.body))
	case "e":
		a := m.account()
		if a == nil {
			return nil
		}
		return tea.Batch(
			m.setStatus("正在导出整封邮件 (.eml)…"),
			saveEmlCmd(*a, m.folder, m.viewer.summary.UID, m.viewer.summary.Subject))
	}
	var cmd tea.Cmd
	m.viewer.vp, cmd = m.viewer.vp.Update(msg)
	return cmd
}

func renderViewer(m Model) string {
	var sb strings.Builder
	head := titleStyle.Render("邮件内容")
	if m.viewer.loading {
		head += "  " + dimStyle.Render("正在下载正文…")
	}
	sb.WriteString(head + "\n")
	if m.viewer.loading {
		sb.WriteString(dimStyle.Render("\n  拉取中…") + "\n")
	}
	if m.viewer.ready && !m.viewer.loading {
		sb.WriteString(strings.TrimRight(m.viewer.vp.View(), "\n") + "\n")
	}
	sb.WriteString("\n" + helpStyle.Render(helpBar(
		k("↑/↓", "滚动"),
		k("PgUp/PgDn", "翻页"),
		k("g/G", "顶/底"),
		k("c", "复制正文"),
		k("y", "网页邮箱链接"),
		k("d", "下载附件"),
		k("e", "导出.eml"),
		k("Esc", "返回"),
	)))
	sb.WriteString("\n" + renderEcho(m))
	return sb.String()
}
