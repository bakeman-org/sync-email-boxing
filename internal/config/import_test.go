package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadExternalSendEmailFormat(t *testing.T) {
	// go-sendemail 仓库的 qq_email_config.json 格式
	p := writeTemp(t, `{
  "sender_email": "someone@qq.com",
  "sender_key": "test-qq-key",
  "smtp_server": "smtp.qq.com",
  "smtp_port": 465
}`)
	a, err := LoadExternalFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if a.Email != "someone@qq.com" || a.AuthCode != "test-qq-key" {
		t.Fatalf("解析失败: %+v", a)
	}
	if a.IMAPServer != "imap.qq.com" || a.IMAPPort != 993 {
		t.Fatalf("IMAP 推导失败: %+v", a)
	}
}

func TestLoadExternal163(t *testing.T) {
	p := writeTemp(t, `{
  "sender_email": "someone@163.com",
  "sender_key": "test-163-key",
  "smtp_server": "smtp.163.com",
  "smtp_port": 465
}`)
	a, err := LoadExternalFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if a.IMAPServer != "imap.163.com" || a.IMAPPort != 993 {
		t.Fatalf("163 IMAP 推导失败: %+v", a)
	}
}

func TestImportUpsertAndSwitch(t *testing.T) {
	// 隔离配置目录，避免测试写进真实 ~/.config
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	qq := writeTemp(t, `{
  "sender_email": "a@qq.com",
  "sender_key": "k1",
  "smtp_server": "smtp.qq.com",
  "smtp_port": 465
}`)
	c := &Config{}
	res, err := ImportFiles(c, []string{qq})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 || res.Updated != 0 || c.Current != 0 {
		t.Fatalf("首次导入结果错误: %+v", res)
	}

	// 再导入一次：应更新而不是报 0 新增，并且仍然选中该账号
	res, err = ImportFiles(c, []string{qq})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 0 || res.Updated != 1 || c.Current != 0 {
		t.Fatalf("重复导入应更新: %+v", res)
	}

	// 导入第二个文件后当前账号应切到该文件对应账号
	mail163 := writeTemp(t, `{
  "sender_email": "b@163.com",
  "sender_key": "k2",
  "smtp_server": "smtp.163.com",
  "smtp_port": 465
}`)
	c.Accounts = append(c.Accounts, Account{Email: "other@qq.com", Name: "other"})
	res, err = ImportFiles(c, []string{mail163})
	if err != nil {
		t.Fatal(err)
	}
	if c.Accounts[c.Current].Email != "b@163.com" {
		t.Fatalf("导入后未切换到目标账号: current=%d %+v", c.Current, c.Accounts)
	}
	if res.Added != 1 || len(c.Accounts) != 3 {
		t.Fatalf("第二个账号导入错误: %+v accounts=%d", res, len(c.Accounts))
	}
}

func TestImportBadFileKeepsConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c := &Config{}
	bad := writeTemp(t, `{"foo": 1}`)
	if _, err := ImportFiles(c, []string{bad}); err == nil {
		t.Fatal("应当报错")
	}
	if len(c.Accounts) != 0 {
		t.Fatalf("失败时不应写入账号: %+v", c.Accounts)
	}
}
