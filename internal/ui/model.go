package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/cache"
	"sync-email-boxing/internal/config"
	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
)

type mode int

const (
	modeForm mode = iota
	modeAccounts
	modeFolders
	modeList
	modeViewer
)

const inboxLimit = 100

type Model struct {
	cfg           *config.Config
	store         *cache.Store
	mode          mode
	form          formModel
	accIdx        int
	folder        string
	folders       []mail.Folder
	folderIdx     int
	foldersLoad   bool
	msgs          []mail.Summary
	cursor        int
	viewer        viewerModel
	loading       bool
	errMsg        string
	errAt         time.Time // 回显错误时间（超时清除）
	status        string
	statusAt      time.Time // 回显成功消息时间（超时清除）
	syncedAt      time.Time
	width         int
	height        int
	cache         map[uint32]mail.Body
}

func New(cfg *config.Config) Model {
	m := Model{
		cfg:    cfg,
		store:  cache.Default(),
		folder: mail.DefaultFolder,
		cache:  map[uint32]mail.Body{},
	}
	m.viewer.vp = viewport.New(80, 20)
	if len(cfg.Accounts) == 0 {
		m.mode = modeForm
		m.form = newForm()
	} else {
		m.mode = modeList
		m.loading = true
		m.loadCachedInbox()
	}
	return m
}

func (m Model) account() *config.Account {
	if a, err := m.cfg.CurrentAccount(); err == nil {
		return a
	}
	return nil
}

// loadCachedInbox 先用本地缓存填充列表，网络同步在后台进行
func (m *Model) loadCachedInbox() {
	a := m.account()
	if a == nil {
		return
	}
	msgs, at, ok := m.store.LoadInbox(a.Email, m.folder)
	if !ok {
		m.msgs = nil
		m.cursor = 0
		return
	}
	m.msgs = msgs
	m.cursor = 0
	m.syncedAt = at
	m.status = fmt.Sprintf("本地缓存 %d 封 · 上次同步 %s · 后台刷新中…", len(msgs), at.Local().Format("01-02 15:04"))
	m.statusAt = time.Now()
}

func (m Model) Init() tea.Cmd {
	if a := m.account(); a != nil {
		return fetchInboxCmd(*a, m.folder)
	}
	return textinput.Blink
}

type inboxMsg struct {
	email   string
	mailbox string
	msgs    []mail.Summary
	took    time.Duration
	err     error
}

type bodyMsg struct {
	uid  uint32
	body mail.Body
	took time.Duration
	err  error
}

type foldersMsg struct {
	email   string
	folders []mail.Folder
	err     error
}

func fetchInboxCmd(a config.Account, mailbox string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		msgs, err := mail.ListMailbox(a, mailbox, inboxLimit)
		return inboxMsg{email: a.Email, mailbox: mailbox, msgs: msgs, took: time.Since(start), err: err}
	}
}

func fetchBodyCmd(a config.Account, mailbox string, uid uint32) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		body, err := mail.FetchBody(a, mailbox, uid)
		return bodyMsg{uid: uid, body: body, took: time.Since(start), err: err}
	}
}

func fetchFoldersCmd(a config.Account) tea.Cmd {
	return func() tea.Msg {
		folders, err := mail.ListFolders(a)
		return foldersMsg{email: a.Email, folders: folders, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewer.vp.Width = msg.Width
		m.viewer.vp.Height = msg.Height - 8
		return m, nil

	case inboxMsg:
		// 账号或文件夹已被切换，丢弃过期响应
		if a := m.account(); a == nil || a.Email != msg.email || msg.mailbox != m.folder {
			debug.Infof("忽略过期列表响应 %s/%s", msg.email, msg.mailbox)
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			debug.Errf("同步失败: %v", msg.err)
			if len(m.msgs) > 0 {
				return m, m.setError(fmt.Sprintf("同步失败: %v（仍显示本地缓存 %d 封）", msg.err, len(m.msgs)))
			}
			return m, m.setError(msg.err.Error())
		}
		m.msgs = msg.msgs
		if m.cursor >= len(m.msgs) {
			m.cursor = max(0, len(m.msgs)-1)
		}
		m.syncedAt = time.Now()
		if a := m.account(); a != nil {
			if err := m.store.SaveInbox(a.Email, m.folder, m.msgs); err != nil {
				return m, m.setError("缓存写入失败: " + err.Error())
			}
		}
		debug.Infof("同步完成 %s %d 封，耗时 %s", m.folder, len(m.msgs), msg.took)
		return m, m.setStatus(fmt.Sprintf("已同步 %s %d 封 · 耗时 %.1fs · 已写入缓存", m.folder, len(m.msgs), msg.took.Seconds()))

	case bodyMsg:
		m.viewer.loading = false
		if msg.err != nil {
			return m, m.setError(msg.err.Error())
		}
		idx := -1
		for i, s := range m.msgs {
			if s.UID == msg.uid {
				idx = i
				break
			}
		}
		if idx < 0 {
			debug.Infof("忽略过期正文响应 uid=%d", msg.uid)
			return m, nil
		}
		m.cache[msg.uid] = msg.body
		if a := m.account(); a != nil {
			_ = m.store.SaveBody(a.Email, m.folder, msg.uid, msg.body)
		}
		m.viewer.open(m.msgs[idx], msg.body)
		m.mode = modeViewer
		return m, m.setStatus(fmt.Sprintf("正文加载完成 · 耗时 %.1fs（已缓存）", msg.took.Seconds()))

	case foldersMsg:
		m.foldersLoad = false
		if a := m.account(); a == nil || a.Email != msg.email {
			return m, nil
		}
		if msg.err != nil {
			m.mode = modeList
			return m, m.setError(msg.err.Error())
		}
		m.folders = msg.folders
		m.folderIdx = 0
		for i, f := range m.folders {
			if f.Name == m.folder {
				m.folderIdx = i
				break
			}
		}
		return m, m.setStatus(fmt.Sprintf("已获取 %d 个文件夹，按 f 选择", len(msg.folders)))

	case savedMsg:
		if msg.err != nil {
			return m, m.setError(msg.err.Error())
		}
		if msg.existing > 0 {
			return m, m.setStatus(fmt.Sprintf("已存在 %d/%d 个文件（内容相同，未重复保存）: %s",
				msg.existing, len(msg.paths), strings.Join(msg.paths, ", ")))
		}
		return m, m.setStatus(fmt.Sprintf("已保存 %d 个文件: %s", len(msg.paths), strings.Join(msg.paths, ", ")))

	case echoTickMsg:
		if m.echoExpired() {
			m.status, m.errMsg = "", ""
			m.statusAt, m.errAt = time.Time{}, time.Time{}
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			debug.Infof("用户退出")
			return m, tea.Quit
		}
		switch m.mode {
		case modeForm:
			return m, updateForm(&m, msg)
		case modeAccounts:
			return m, updateAccounts(&m, msg)
		case modeFolders:
			return m, updateFolders(&m, msg)
		case modeList:
			return m, updateList(&m, msg)
		case modeViewer:
			return m, updateViewer(&m, msg)
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.mode {
	case modeForm:
		return renderForm(m)
	case modeAccounts:
		return renderAccounts(m)
	case modeFolders:
		return renderFolders(m)
	case modeViewer:
		return renderViewer(m)
	default:
		return renderList(m)
	}
}
