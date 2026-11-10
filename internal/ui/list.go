package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/x/ansi"

	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
)

// updateList 收件箱列表模式按键
func updateList(m *Model, msg tea.KeyMsg) tea.Cmd {
	// 搜索输入态：输入实时过滤（fzf 风格），Esc 取消，Enter 确认
	if m.searching {
		switch msg.String() {
		case "esc":
			m.searching = false
			m.searchInput.Blur()
			if m.query != "" {
				m.query = ""
				m.cursor = 0
				return m.setStatus("已取消搜索")
			}
			return nil
		case "enter":
			m.searching = false
			m.searchInput.Blur()
			if m.query == "" {
				return m.setStatus("已清除搜索")
			}
			return m.setStatus(fmt.Sprintf("搜索 %q · %d/%d 封匹配 · Esc 清除", m.query, len(m.visible()), len(m.msgs)))
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			// 每次按键立即生效
			m.query = strings.TrimSpace(m.searchInput.Value())
			m.cursor = 0
			if m.query == "" {
				return cmd
			}
			return tea.Batch(cmd, m.setStatus(fmt.Sprintf("实时过滤 %d/%d 封匹配", len(m.visible()), len(m.msgs))))
		}
	}

	switch msg.String() {
	case "q":
		return tea.Quit
	case "/":
		m.searching = true
		m.searchInput.SetValue(m.query)
		m.searchInput.CursorEnd()
		return tea.Batch(m.searchInput.Focus(), textinput.Blink)
	case "esc":
		if m.query != "" {
			m.query = ""
			m.cursor = 0
			return m.setStatus("已清除搜索")
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visible())-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
		return m.setStatus("已跳到第一封")
	case "G":
		if n := len(m.visible()); n > 0 {
			m.cursor = n - 1
			return m.setStatus(fmt.Sprintf("已跳到最后一封 (%d/%d)", m.cursor+1, n))
		}
	case "enter":
		vis := m.visible()
		if len(vis) == 0 {
			return nil
		}
		s := m.msgs[vis[m.cursor]]
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
	case "S":
		// 切换首选翻译源（循环），写回 translators.json
		ts := m.cfg.TranslateSettings()
		next := ts.NextSource()
		if next == ts.ActiveName() {
			return m.setStatus("翻译源只有 sogou 可用（其他源未在 translators.json 配置）")
		}
		ts.Active = next
		if err := ts.Save(); err != nil {
			return m.setStatus("已切换到 " + next + "，但保存失败: " + err.Error())
		}
		return m.setStatus(fmt.Sprintf("翻译源 → %s（已保存）", next))
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

// visible 当前搜索过滤后的可见邮件（m.msgs 的下标），无搜索词时全部可见
func (m Model) visible() []int {
	if m.query == "" {
		idx := make([]int, len(m.msgs))
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	q := strings.ToLower(m.query)
	var idx []int
	for i, s := range m.msgs {
		if strings.Contains(strings.ToLower(s.Subject), q) ||
			strings.Contains(strings.ToLower(s.From), q) ||
			strings.Contains(strings.ToLower(s.Date), q) {
			idx = append(idx, i)
		}
	}
	return idx
}

func renderList(m Model) string {
	var sb strings.Builder

	vis := m.visible()

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
	if m.query != "" {
		head += "  " + searchStyle.Render(fmt.Sprintf("搜索:%s (%d/%d)", m.query, len(vis), len(m.msgs)))
	}
	sb.WriteString(head + "\n\n")

	// 搜索输入行
	if m.searching {
		sb.WriteString(searchStyle.Render(" "+m.searchInput.View()) + "\n\n")
	}

	if len(vis) == 0 && !m.loading {
		if m.query != "" {
			sb.WriteString(dimStyle.Render("\n  没有匹配的邮件 · Esc 清除搜索") + "\n")
		} else {
			sb.WriteString(dimStyle.Render("\n  该文件夹没有邮件 · 按 f 选择其他文件夹，r 刷新") + "\n")
		}
	}

	rows := m.height - 6
	if m.searching {
		rows -= 2
	}
	if rows < 1 {
		rows = 1
	}
	// 保持光标可见
	start, end := 0, len(vis)
	if len(vis) > rows {
		if m.cursor >= rows/2 && m.cursor < len(vis)-rows/2 {
			start = m.cursor - rows/2
		} else if m.cursor >= len(vis)-rows/2 {
			start = len(vis) - rows
		}
		end = start + rows
	}

	dateW, fromW := 16, 28
	subjW := 60
	if m.width > 0 {
		subjW = m.width - dateW - fromW - 12
	}
	for j, i := range vis[start:end] {
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
		if m.cursor == start+j {
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
		k("/", "搜索"),
		k("r", "刷新"),
		k("f", "文件夹"),
		k("g/G", "顶/底"),
		k("s", "账号"),
		k("a", "添加账号"),
		k("S", "翻译源"),
		k("L", "日志/缓存"),
		k("q", "退出"),
	)))
	sb.WriteString("\n" + renderEcho(m))
	return sb.String()
}
