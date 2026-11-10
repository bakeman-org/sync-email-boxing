package ui

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/clipboard"
	"sync-email-boxing/internal/config"
	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/mail"
	"sync-email-boxing/internal/save"
	"sync-email-boxing/internal/translate"
)

type viewerModel struct {
	vp      viewport.Model
	ready   bool
	loading bool
	summary mail.Summary
	body    mail.Body
	// 翻译状态（按 t 触发）
	translating bool
	translation string // 译文（空 = 尚未翻译）
	transSrc    string // 实际使用的翻译源
	transTarget string // 目标语言
	transTook   time.Duration
	showTrans   bool // true = 显示译文，false = 显示原文
}

// content 组装查看内容：头部 + 正文（可选附上译文）
func (v *viewerModel) content() string {
	c := mail.RenderHeaders(v.summary.From, v.summary.Subject, v.summary.Date, v.body) + "\n\n" + v.body.Text
	if v.showTrans && v.translation != "" {
		c += fmt.Sprintf("\n\n―――― 翻译（%s → %s · %.1fs）――――\n\n%s",
			v.transSrc, v.transTarget, v.transTook.Seconds(), v.translation)
	}
	return c
}

func (v *viewerModel) open(s mail.Summary, body mail.Body) {
	v.summary = s
	v.body = body
	v.translating = false
	v.translation = ""
	v.showTrans = false
	v.vp.GotoTop()
	v.vp.SetContent(v.content())
	v.ready = true
}

type savedMsg struct {
	paths    []string
	existing int // 已存在且内容一致、未重复写入的数量
	err      error
}

type translateMsg struct {
	text   string // 发起翻译时的原文（防止期间切信导致错配）
	result string
	source string
	took   time.Duration
	err    error
}

func translateCmd(text, target string, s *translate.Settings) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		res, src, err := translate.Translate(text, target, s)
		return translateMsg{text: text, result: res, source: src, took: time.Since(start), err: err}
	}
}

// textHash 译文缓存键（正文 + 目标语言）
func textHash(text, target string) string {
	sum := sha1.Sum([]byte(text + "|" + target))
	return hex.EncodeToString(sum[:])[:10]
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
	case "t", "T":
		// t = 翻译/切回原文；T = 忽略缓存，强制用当前首选源重译
		force := msg.String() == "T"
		if m.viewer.translating {
			return nil
		}
		if m.viewer.body.Text == "" {
			return m.setStatus("该邮件没有可翻译的正文")
		}
		target := m.cfg.TranslateTargetWithDefault()
		source := m.cfg.TranslateSourceWithDefault()

		// 已有译文：t 切换显示原文/译文
		if m.viewer.translation != "" && !force {
			m.viewer.showTrans = !m.viewer.showTrans
			m.viewer.vp.SetContent(m.viewer.content())
			if m.viewer.showTrans {
				m.viewer.vp.GotoBottom()
			} else {
				m.viewer.vp.GotoTop()
			}
			return m.setStatus(fmt.Sprintf("已切换显示译文/原文 · 译文来自 %s · T 用 %s 重译",
				m.viewer.transSrc, source))
		}
		if !force && translate.IsChinese(m.viewer.body.Text) && target == translate.Chinese {
			return m.setStatus("正文已是中文，无需翻译")
		}
		// 命中本地译文缓存则直接显示，不联网
		if !force {
			if a := m.account(); a != nil {
				if tr, ok := m.store.LoadTranslation(a.Email, m.folder, m.viewer.summary.UID, textHash(m.viewer.body.Text, target)); ok && tr.Result != "" {
					m.viewer.translation = tr.Result
					m.viewer.transSrc = tr.Source
					m.viewer.transTarget = tr.Target
					m.viewer.showTrans = true
					m.viewer.vp.SetContent(m.viewer.content())
					m.viewer.vp.GotoBottom()
					if tr.Source != source {
						return m.setStatus(fmt.Sprintf("缓存译文来自 %s（当前首选 %s）· T 用 %s 重译",
							tr.Source, source, source))
					}
					return m.setStatus(fmt.Sprintf("已加载缓存译文（%s）· 再按 t 切回原文", tr.Source))
				}
			}
		}
		m.viewer.translating = true
		return tea.Batch(
			m.setStatus(fmt.Sprintf("正在翻译（%s → %s）…", source, target)),
			translateCmd(m.viewer.body.Text, target, m.cfg.TranslateSettings()))
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
	} else {
		head += "  " + dimStyle.Render(fmt.Sprintf("翻译源 %s → %s",
			m.cfg.TranslateSourceWithDefault(), m.cfg.TranslateTargetWithDefault()))
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
		k("t/T", "翻译/重译"),
		k("S", "翻译源"),
		k("y", "网页邮箱链接"),
		k("d", "下载附件"),
		k("e", "导出.eml"),
		k("Esc", "返回"),
	)))
	sb.WriteString("\n" + renderEcho(m))
	return sb.String()
}
