package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/clipboard"
	"sync-email-boxing/internal/mail"
	"sync-email-boxing/internal/save"
)

// EMLModel 本地 .eml 文件查看器（sync-email-boxing view <file.eml>）
type EMLModel struct {
	path    string
	raw     []byte
	sum     mail.Summary
	body    mail.Body
	vp      viewport.Model
	ready   bool
	showAll bool // false=精简头信息, true=完整头信息

	status   string
	statusAt time.Time
	errMsg   string
	errAt    time.Time

	width  int
	height int
}

// NewEMLViewer 从 .eml 文件构造查看器
func NewEMLViewer(path string) (EMLModel, error) {
	sum, body, raw, err := mail.ParseEMLFile(path)
	if err != nil {
		return EMLModel{}, err
	}
	m := EMLModel{path: path, raw: raw, sum: sum, body: body}
	m.vp = viewport.New(80, 20)
	return m, nil
}

func (m *EMLModel) setSt(s string) tea.Cmd {
	m.status = s
	m.statusAt = time.Now()
	m.errMsg = ""
	return echoClearCmd()
}

func (m *EMLModel) setErr(s string) tea.Cmd {
	m.errMsg = s
	m.errAt = time.Now()
	m.status = ""
	return echoClearCmd()
}

func (m EMLModel) expired() bool {
	if m.errMsg != "" && time.Since(m.errAt) >= echoErrTimeout {
		return true
	}
	return m.status != "" && time.Since(m.statusAt) >= echoStatusTimeout
}

// reload 重新生成视口内容（切换精简/完整头信息后调用）
func (m *EMLModel) reload() {
	var head string
	if m.showAll {
		head = mail.EMLFullHeaders(m.raw)
	} else {
		head = mail.RenderHeaders(m.sum.From, m.sum.Subject, m.sum.Date, m.body)
	}
	m.vp.SetContent(head + "\n\n" + m.body.Text)
	m.vp.GotoTop()
	m.ready = true
}

func (m EMLModel) Init() tea.Cmd { return nil }

func (m EMLModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.vp.Width = msg.Width
		m.vp.Height = msg.Height - 6
		m.reload()
		return m, nil

	case echoTickMsg:
		if m.expired() {
			m.status, m.errMsg = "", ""
			m.statusAt, m.errAt = time.Time{}, time.Time{}
		}
		return m, nil

	case savedMsg:
		if msg.err != nil {
			return m, m.setErr(msg.err.Error())
		}
		if msg.existing > 0 {
			return m, m.setSt(fmt.Sprintf("已存在 %d/%d 个附件（内容相同，未重复保存）: %s",
				msg.existing, len(msg.paths), strings.Join(msg.paths, ", ")))
		}
		return m, m.setSt(fmt.Sprintf("已保存 %d 个附件: %s", len(msg.paths), strings.Join(msg.paths, ", ")))

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.vp.LineUp(1)
		case "down", "j":
			m.vp.LineDown(1)
		case "pgup":
			m.vp.HalfViewUp()
		case "pgdown", " ":
			m.vp.HalfViewDown()
		case "g":
			m.vp.GotoTop()
		case "G":
			m.vp.GotoBottom()
		case "h":
			m.showAll = !m.showAll
			m.reload()
			if m.showAll {
				return m, m.setSt("显示完整头信息")
			}
			return m, m.setSt("显示精简头信息")
		case "c":
			if err := clipboard.Copy(m.body.Text); err != nil {
				return m, m.setErr("复制失败: " + err.Error())
			}
			return m, m.setSt(fmt.Sprintf("正文已复制到剪贴板（%d 字）", len([]rune(m.body.Text))))
		case "f":
			addr := mail.ExtractAddress(m.sum.From)
			if err := clipboard.Copy(addr); err != nil {
				return m, m.setErr("复制失败: " + err.Error())
			}
			return m, m.setSt("发件人地址已复制: " + addr)
		case "d":
			if len(m.body.Attachments) == 0 {
				return m, m.setSt("该邮件没有附件")
			}
			return m, tea.Batch(
				m.setSt(fmt.Sprintf("正在保存 %d 个附件…", len(m.body.Attachments))),
				saveEmlAttsCmd(m.path, m.body))
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

// saveEmlAttsCmd 把 .eml 中的附件保存到 下载目录/eml-view/<文件名>/
func saveEmlAttsCmd(path string, body mail.Body) tea.Cmd {
	return func() tea.Msg {
		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		dir := filepath.Join(save.Base(), "eml-view", stem)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return savedMsg{err: err}
		}
		paths, existing, err := save.Attachments(dir, body.Attachments)
		return savedMsg{paths: paths, existing: existing, err: err}
	}
}

func (m EMLModel) View() string {
	var sb strings.Builder
	head := titleStyle.Render("EML 查看器") + "  " + dimStyle.Render(filepath.Base(m.path))
	sb.WriteString(head + "\n")

	if m.ready {
		sb.WriteString(strings.TrimRight(m.vp.View(), "\n") + "\n")
	}

	sb.WriteString("\n" + helpStyle.Render(helpBar(
		k("↑/↓", "滚动"),
		k("PgUp/PgDn", "翻页"),
		k("g/G", "顶/底"),
		k("h", "切换头信息"),
		k("c", "复制正文"),
		k("f", "复制发件人"),
		k("d", "下载附件"),
		k("q", "退出"),
	)))

	// 回显区
	prefix := helpStyle.Render("[EML]")
	switch {
	case m.errMsg != "":
		sb.WriteString("\n" + prefix + errStyle.Render(" ✗ "+truncateW(m.errMsg, maxInt(m.width-14, 20))))
	case m.status != "":
		sb.WriteString("\n" + prefix + statusStyle.Render(" ✓ "+truncateW(m.status, maxInt(m.width-14, 20))))
	default:
		sb.WriteString("\n" + prefix + dimStyle.Render(" （就绪）"))
	}
	return sb.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
