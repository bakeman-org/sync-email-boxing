package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/config"
	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
)

// updateAccounts 账号切换/删除/新增
func updateAccounts(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeList
		return nil
	case "up", "k":
		if m.accIdx > 0 {
			m.accIdx--
		}
	case "down", "j":
		if m.accIdx < len(m.cfg.Accounts)-1 {
			m.accIdx++
		}
	case "enter":
		if len(m.cfg.Accounts) == 0 {
			return nil
		}
		m.cfg.Current = m.accIdx
		if err := config.Save(m.cfg); err != nil {
			return m.setError("保存配置失败: " + err.Error())
		}
		m.mode = modeList
		m.cursor = 0
		m.cache = map[uint32]mail.Body{}
		m.loading = true
		m.folder = mail.DefaultFolder
		m.folders = nil
		m.folderIdx = 0
		a := m.cfg.Accounts[m.accIdx]
		m.loadCachedInbox()
		debug.Infof("切换账号 -> %s", a.Email)
		return tea.Batch(m.setStatus("已切换到 "+a.Email+"，正在同步…"), fetchInboxCmd(a, m.folder))
	case "a", "n":
		m.mode = modeForm
		m.form = newForm()
		m.errMsg = ""
	case "d":
		if len(m.cfg.Accounts) == 0 {
			return nil
		}
		removed := m.cfg.Accounts[m.accIdx]
		m.cfg.Accounts = append(m.cfg.Accounts[:m.accIdx], m.cfg.Accounts[m.accIdx+1:]...)
		if m.cfg.Current >= len(m.cfg.Accounts) {
			m.cfg.Current = len(m.cfg.Accounts) - 1
		}
		if m.accIdx >= len(m.cfg.Accounts) {
			m.accIdx = len(m.cfg.Accounts) - 1
		}
		if m.accIdx < 0 {
			m.accIdx = 0
		}
		if err := config.Save(m.cfg); err != nil {
			return m.setError("保存配置失败: " + err.Error())
		}
		m.status = "已删除账号 " + removed.Email
		m.statusAt = time.Now()
		if len(m.cfg.Accounts) == 0 {
			m.mode = modeForm
			m.form = newForm()
		}
	}
	return nil
}

func renderAccounts(m Model) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("账号列表") + "\n\n")
	if len(m.cfg.Accounts) == 0 {
		sb.WriteString(dimStyle.Render("  （无账号）") + "\n")
	}
	for i, a := range m.cfg.Accounts {
		line := fmt.Sprintf("%s %-10s %s", a.Name, a.Email, dimStyle.Render(fmt.Sprintf("smtp:%s:%d imap:%s:%d", a.SMTPServer, a.SMTPPort, a.IMAPServer, a.IMAPPort)))
		prefix := "  "
		if i == m.cfg.Current {
			prefix = selectedMarkStyle.Render("● ")
		}
		if i == m.accIdx {
			line = selRowStyle.Render(" " + line + " ")
		}
		sb.WriteString(prefix + line + "\n")
	}
	sb.WriteString("\n" + helpStyle.Render(helpBar(
		k("↑/↓", "选择"),
		k("Enter", "切换到该账号"),
		k("a", "添加"),
		k("d", "删除"),
		k("Esc", "返回"),
	)))
	sb.WriteString("\n" + renderEcho(m))
	return sb.String()
}
