package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"sync-email-boxing/internal/config"
	"sync-email-boxing/internal/debug"
	"sync-email-boxing/internal/save"
	"sync-email-boxing/internal/translate"
	"sync-email-boxing/internal/ui"
)

// pathsFlag 支持重复 -c 以及逗号分隔多个路径
type pathsFlag []string

func (p *pathsFlag) String() string { return strings.Join(*p, ",") }

func (p *pathsFlag) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			*p = append(*p, s)
		}
	}
	return nil
}

func main() {
	// view 子命令：查看本地 .eml 文件
	if len(os.Args) > 1 && (os.Args[1] == "view" || os.Args[1] == "eml") {
		paths := os.Args[2:]
		if len(paths) == 0 {
			fmt.Fprintln(os.Stderr, "用法: sync-email-boxing view <file.eml>")
			os.Exit(1)
		}
		runEMLViewer(paths[0])
		return
	}

	// reset 子命令：清空缓存/配置，恢复出厂状态
	if len(os.Args) > 1 && os.Args[1] == "reset" {
		runReset(os.Args[2:])
		return
	}

	// accounts 子命令：列出已配置的邮箱账号
	if len(os.Args) > 1 && (os.Args[1] == "accounts" || os.Args[1] == "list") {
		runAccounts()
		return
	}

	var extPaths pathsFlag
	var verbose bool
	var logPath string
	var useEmail string
	var downDir string
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `sync-email-boxing — QQ/163 等邮箱收件箱 TUI

用法:
  sync-email-boxing [flags]          启动收件箱 TUI
  sync-email-boxing accounts         列出已配置的邮箱账号
  sync-email-boxing view <file.eml>  查看本地 .eml 文件
  sync-email-boxing reset [目标]     清空缓存/配置（默认 all，见 reset -h）

Flags:
`)
		flag.PrintDefaults()
	}
	flag.Var(&extPaths, "c", "从外部配置文件导入账号（可重复或逗号分隔；兼容 go-sendemail 的 qq/163 格式）")
	flag.Var(&extPaths, "config", "同 -c")
	flag.StringVar(&useEmail, "u", "", "启动时切换到指定账号（邮箱地址或序号 1/2/…，先看 accounts）")
	flag.StringVar(&useEmail, "user", "", "同 -u")
	flag.StringVar(&downDir, "o", "", "附件/.eml 下载目录（默认 ~/Downloads/sync-email-boxing）")
	flag.StringVar(&downDir, "output", "", "同 -o")
	flag.BoolVar(&verbose, "v", false, "开启调试日志（开发者模式，默认关闭，不产生日志文件）")
	flag.BoolVar(&verbose, "debug", false, "同 -v")
	flag.StringVar(&logPath, "log", "", "调试日志文件路径（默认 ~/.config/sync-email-boxing/debug.log，需 -v 开启）")
	flag.Parse()

	save.SetBase(downDir)

	// 日志默认关闭；-v/--debug 开启，-log 指定路径也视为开启
	logEnabled := verbose || logPath != ""
	if err := debug.Init(logPath, logEnabled); err != nil {
		fmt.Fprintln(os.Stderr, "初始化日志失败:", err)
		os.Exit(1)
	}
	defer debug.Close()
	debug.Infof("配置文件: %s", config.Path())
	if logEnabled {
		debug.Infof("日志文件: %s", debug.Path())
	}

	cfg, err := config.Load()
	if err != nil {
		debug.Errf("加载配置失败: %v", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	debug.Infof("已加载 %d 个账号，当前第 %d 个", len(cfg.Accounts), cfg.Current)

	if len(extPaths) > 0 {
		res, err := config.ImportFiles(cfg, extPaths)
		if err != nil {
			debug.Errf("导入外部配置失败: %v", err)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		debug.Infof("导入外部配置 %v：新增 %d，更新 %d，当前账号 %s", extPaths, res.Added, res.Updated, cfg.Accounts[res.Current].Email)
		fmt.Printf("配置已保存: %s（新增 %d，更新 %d）\n", config.Path(), res.Added, res.Updated)
		fmt.Printf("当前账号: %s\n", cfg.Accounts[res.Current].Email)
	}

	if useEmail != "" {
		idx, ok := resolveAccount(cfg, useEmail)
		if !ok {
			fmt.Fprintf(os.Stderr, "配置中没有账号 %q，可用账号：\n", useEmail)
			printAccounts(cfg, os.Stderr)
			os.Exit(1)
		}
		cfg.Current = idx
		if err := config.Save(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "保存配置失败:", err)
			os.Exit(1)
		}
		debug.Infof("启动时切换账号 -> %s", cfg.Accounts[idx].Email)
		fmt.Printf("当前账号: %s\n", cfg.Accounts[idx].Email)
	}

	for i, a := range cfg.Accounts {
		debug.Infof("  账号[%d] %s imap=%s:%d%s", i, a.Email, a.IMAPServer, a.IMAPPort, currentMark(i == cfg.Current))
	}

	p := tea.NewProgram(ui.New(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		debug.Errf("TUI 运行失败: %v", err)
		fmt.Fprintln(os.Stderr, "运行失败:", err)
		os.Exit(1)
	}
	if logEnabled {
		fmt.Printf("日志已写入: %s\n", debug.Path())
	}
}

func currentMark(cur bool) string {
	if cur {
		return " (当前)"
	}
	return ""
}

// runAccounts 列出已配置账号（不启动 TUI）
func runAccounts() {
	if err := debug.Init("", false); err != nil {
		fmt.Fprintln(os.Stderr, "初始化日志失败:", err)
		os.Exit(1)
	}
	defer debug.Close()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("配置文件: %s\n", config.Path())
	if len(cfg.Accounts) == 0 {
		fmt.Println("（还没有账号：运行 sync-email-boxing 按 a 添加，或用 -c 导入配置文件）")
		return
	}
	printAccounts(cfg, os.Stdout)
}

// printAccounts 打印账号列表（序号 1 起始，与 -u 序号一致）
func printAccounts(cfg *config.Config, w io.Writer) {
	for i, a := range cfg.Accounts {
		mark := " "
		if i == cfg.Current {
			mark = "●"
		}
		fmt.Fprintf(w, "%s %d. %-14s %-28s imap %s:%d\n",
			mark, i+1, a.Name, a.Email, a.IMAPServer, a.IMAPPort)
	}
}

// resolveAccount 解析 -u：纯数字按 1 起始序号，否则按邮箱地址匹配
func resolveAccount(cfg *config.Config, s string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 1 && n <= len(cfg.Accounts) {
			return n - 1, true
		}
		return -1, false
	}
	idx := cfg.IndexOfEmail(s)
	return idx, idx >= 0
}

// runEMLViewer 启动本地 .eml 文件查看器
func runEMLViewer(path string) {
	if err := debug.Init("", false); err != nil {
		fmt.Fprintln(os.Stderr, "初始化日志失败:", err)
		os.Exit(1)
	}
	defer debug.Close()

	em, err := ui.NewEMLViewer(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := tea.NewProgram(em, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "运行失败:", err)
		os.Exit(1)
	}
}

// resetTarget reset 子命令可清理的目标
type resetTarget struct {
	name  string   // 子命令名
	desc  string   // 说明
	path  string   // 要删除的路径（空串=整个配置目录）
	extra []string // 一并删除的其他文件
}

// configBaseDir 配置根目录 ~/.config/sync-email-boxing/
func configBaseDir() string {
	return filepath.Dir(config.Path())
}

// runReset 清空缓存/配置：sync-email-boxing reset [cache|config|log] [-y]
func runReset(args []string) {
	base := configBaseDir()
	targets := []resetTarget{
		{"cache", "正文/附件缓存", filepath.Join(base, "cache"), nil},
		{"config", "账号配置（含授权码）+ 翻译设置", config.Path(), []string{translate.SettingsPath()}},
		{"log", "调试日志", filepath.Join(base, "debug.log"), nil},
		{"all", "以上全部 + 整个配置目录", base, nil},
	}

	var yes bool
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	fs.BoolVar(&yes, "y", false, "跳过确认，直接执行")
	fs.BoolVar(&yes, "yes", false, "同 -y")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: sync-email-boxing reset [目标] [-y]\n\n目标:\n")
		for _, t := range targets {
			fmt.Fprintf(os.Stderr, "  %-6s %s\n", t.name, t.desc)
		}
		fmt.Fprintf(os.Stderr, "\n不带目标时默认 all（清空整个 ~/.config/sync-email-boxing，恢复出厂状态）\n")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	name := "all"
	if fs.NArg() > 0 {
		name = fs.Arg(0)
	}
	var target *resetTarget
	for i := range targets {
		if targets[i].name == name {
			target = &targets[i]
			break
		}
	}
	if target == nil {
		fmt.Fprintf(os.Stderr, "未知目标 %q\n\n", name)
		fs.Usage()
		os.Exit(1)
	}

	// 展示将要删除的内容与大小
	exists := false
	for _, p := range append([]string{target.path}, target.extra...) {
		if _, err := os.Stat(p); err == nil {
			exists = true
			fmt.Printf("将删除: %s\n  %s (%s)\n", target.desc, p, humanSize(dirOrFileSize(p)))
		}
	}
	if !exists {
		fmt.Printf("%s 不存在，无需清理\n", target.desc)
		return
	}

	if !yes {
		fmt.Print("确认删除? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("已取消")
			return
		}
	}
	if err := os.RemoveAll(target.path); err != nil {
		fmt.Fprintln(os.Stderr, "删除失败:", err)
		os.Exit(1)
	}
	for _, p := range target.extra {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "删除失败:", err)
			os.Exit(1)
		}
	}
	fmt.Println("已删除。下次启动", filepath.Base(os.Args[0]), "将回到初始状态。")
}

// dirOrFileSize 目录或文件的总大小
func dirOrFileSize(p string) int64 {
	var total int64
	filepath.Walk(p, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// humanSize 字节数转可读大小
func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(n)/1024/1024)
	}
}
