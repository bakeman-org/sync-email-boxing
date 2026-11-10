package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/x/ansi"

	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
)

// updateList 收件箱列表模式按键
func updateList(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		return tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.msgs)-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
		return m.setStatus("已跳到第一封")
	case "G":
		if len(m.msgs) > 0 {
			m.cursor = len(m.msgs) - 1
			return m.setStatus(fmt.Sprintf("已跳到最后一封 (%d/%d)", m.cursor+1, len(m.msgs)))
		}
	case "enter":
		if len(m.msgs) == 0 {
			return nil
		}
		s := m.msgs[m.cursor]
		if body, ok := m.cache[s.UID]; ok {
			m.viewer.open(s, body)
			m.mode = modeViewer
			return nil
		}
		a := m.account()
		if a == nil {
			return nil
		}
		if body, ok := m.store.LoadBody(a.Email, m.folder, s.UID); ok {
			m.cache[s.UID] = body
			m.viewer.open(s, body)
			m.mode = modeViewer
			return m.setStatus("正文来自本地缓存，无需联网")
		}
		m.viewer.loading = true
		m.viewer.summary = s
		return tea.Batch(m.setStatus("正在下载正文 "+truncateW(s.Subject, 24)+" …"), fetchBodyCmd(*a, m.folder, s.UID))
	case "r":
		a := m.account()
		if a == nil {
			return nil
		}
		m.loading = true
		return tea.Batch(m.setStatus("正在同步 "+m.folder+" …"), fetchInboxCmd(*a, m.folder))
	case "f":
		a := m.account()
		if a == nil {
			return nil
		}
		m.mode = modeFolders
		m.errMsg = ""
		if len(m.folders) > 0 {
			m.folderIdx = 0
			for i, f := range m.folders {
				if f.Name == m.folder {
					m.folderIdx = i
					break
				}
			}
			return nil
		}
		m.foldersLoad = true
		return fetchFoldersCmd(*a)
	case "s":
		m.mode = modeAccounts
		m.accIdx = m.cfg.Current
		return nil
	case "a":
		m.mode = modeForm
		m.form = newForm()
		m.errMsg = ""
		return nil
	case "L":
		logInfo := "未开启（用 -v/--debug 打开）"
		if p := debug.Path(); p != "" {
			logInfo = p
		}
		return m.setStatus(fmt.Sprintf("日志: %s | 缓存: %s", logInfo, m.store.Path()))
	}
	return nil
}

func truncateW(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

func renderList(m Model) string {
	var sb strings.Builder

	head := titleStyle.Render("收件箱")
	if m.folder != mail.DefaultFolder {
		head = titleStyle.Render(m.folder)
	}
	if a := m.account(); a != nil {
		head += "  " + dimStyle.Render(a.Name+" <"+a.Email+">")
	}
	if m.loading {
		if len(m.msgs) > 0 {
			head += "  " + dimStyle.Render("后台刷新中…")
		} else {
			head += "  " + dimStyle.Render("加载中…")
		}
	}
	sb.WriteString(head + "\n\n")

	if len(m.msgs) == 0 && !m.loading {
		sb.WriteString(dimStyle.Render("\n  该文件夹没有邮件 · 按 f 选择其他文件夹，r 刷新") + "\n")
	}

	rows := m.height - 6
	if rows < 1 {
		rows = 1
	}
	// 保持光标可见
	start, end := 0, len(m.msgs)
	if len(m.msgs) > rows {
		if m.cursor >= rows/2 && m.cursor < len(m.msgs)-rows/2 {
			start = m.cursor - rows/2
		} else if m.cursor >= len(m.msgs)-rows/2 {
			start = len(m.msgs) - rows
		}
		end = start + rows
	}

	dateW, fromW := 16, 28
	subjW := 60
	if m.width > 0 {
		subjW = m.width - dateW - fromW - 12
	}
	for i := start; i < end; i++ {
		s := m.msgs[i]
		mark := "  "
		if !s.Seen {
			mark = unreadStyle.Render("● ")
		}
		line := fmt.Sprintf("%s %s  %s  %s",
			mark,
			dimStyle.Render(truncateW(s.Date, dateW)),
			truncateW(s.From, fromW),
			truncateW(s.Subject, subjW),
		)
		if i == m.cursor {
			if m.width > 0 {
				line = selRowStyle.Render(ansi.Truncate(line, m.width-1, " "))
			} else {
				line = selRowStyle.Render(line)
			}
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n" + helpStyle.Render(helpBar(
		k("↑/↓", "选择"),
		k("Enter", "查看"),
		k("r", "刷新"),
		k("f", "文件夹"),
		k("g/G", "顶/底"),
		k("s", "账号"),
		k("a", "添加账号"),
		k("L", "日志/缓存"),
		k("q", "退出"),
	)))
	sb.WriteString("\n" + renderEcho(m))
	return sb.String()
}
