package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
)

// updateFolders 文件夹选择模式按键
func updateFolders(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeList
		return nil
	case "up", "k":
		if m.folderIdx > 0 {
			m.folderIdx--
		}
	case "down", "j":
		if m.folderIdx < len(m.folders)-1 {
			m.folderIdx++
		}
	case "r":
		a := m.account()
		if a == nil {
			return nil
		}
		m.foldersLoad = true
		m.errMsg = ""
		return fetchFoldersCmd(*a)
	case "enter":
		if len(m.folders) == 0 {
			return nil
		}
		a := m.account()
		if a == nil {
			return nil
		}
		name := m.folders[m.folderIdx].Name
		if name == m.folder {
			m.mode = modeList
			return nil
		}
		m.folder = name
		m.mode = modeList
		m.cursor = 0
		m.msgs = nil
		m.cache = map[uint32]mail.Body{}
		m.loading = true
		m.loadCachedInbox()
		debug.Infof("切换文件夹 -> %s", name)
		return tea.Batch(m.setStatus("正在打开 "+name+" …"), fetchInboxCmd(*a, name))
	}
	return nil
}

func renderFolders(m Model) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("选择文件夹") + "\n")
	if a := m.account(); a != nil {
		sb.WriteString(dimStyle.Render("  "+a.Email) + "\n")
	}
	sb.WriteString("\n")
	if m.foldersLoad {
		sb.WriteString(dimStyle.Render("  加载文件夹列表…") + "\n")
	}
	if len(m.folders) == 0 && !m.foldersLoad {
		sb.WriteString(dimStyle.Render("  （没有可选文件夹，按 r 重试）") + "\n")
	}
	for i, f := range m.folders {
		line := fmt.Sprintf("%-24s %s", f.Name, dimStyle.Render(fmt.Sprintf("%d 封", f.Messages)))
		prefix := "  "
		if f.Name == m.folder {
			prefix = selectedMarkStyle.Render("● ")
		}
		if i == m.folderIdx {
			line = selRowStyle.Render(" " + line + " ")
		}
		sb.WriteString(prefix + line + "\n")
	}
	sb.WriteString("\n" + helpStyle.Render(helpBar(
		k("↑/↓", "选择"),
		k("Enter", "打开该文件夹"),
		k("r", "刷新列表"),
		k("Esc", "返回"),
	)))
	sb.WriteString("\n" + renderEcho(m))
	return sb.String()
}
