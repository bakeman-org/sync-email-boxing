package save

import (
	"os"
	"path/filepath"
	"testing"

	"sync-email-boxing/internal/mail"
)

func TestAttachmentsDedup(t *testing.T) {
	dir := t.TempDir()
	atts := []mail.Attachment{{Name: "报告.pdf", ContentType: "application/pdf", Data: []byte("hello pdf")}}

	paths, existing, err := Attachments(dir, atts)
	if err != nil {
		t.Fatal(err)
	}
	if existing != 0 || len(paths) != 1 {
		t.Fatalf("首次保存应新写入 1 个: paths=%v existing=%d", paths, existing)
	}

	// 同样内容再保存一次：不重复写
	paths2, existing2, err := Attachments(dir, atts)
	if err != nil {
		t.Fatal(err)
	}
	if existing2 != 1 || paths2[0] != paths[0] {
		t.Fatalf("重复保存应复用原文件: paths=%v existing=%d", paths2, existing2)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("目录不应出现副本，实际 %d 个文件", len(files))
	}

	// 同名但内容不同：另存为带序号的新文件
	atts[0].Data = []byte("different content")
	paths3, existing3, err := Attachments(dir, atts)
	if err != nil {
		t.Fatal(err)
	}
	if existing3 != 0 || paths3[0] == paths[0] || filepath.Base(paths3[0]) == "报告.pdf" {
		t.Fatalf("同名不同内容应另存: %v", paths3)
	}
	files, _ = os.ReadDir(dir)
	if len(files) != 2 {
		t.Fatalf("应有 2 个文件，实际 %d", len(files))
	}
}

func TestEMLDedup(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("Subject: hi\r\n\r\nbody")
	p1, err := EML(dir, "主题", 42, raw)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := EML(dir, "主题", 42, raw)
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatalf("相同内容应复用同一文件: %s vs %s", p1, p2)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("不应重复写出 .eml，实际 %d 个文件", len(files))
	}
}

func TestSlugAndUniquePath(t *testing.T) {
	if got := slug("a/b:c*d?e\"f<g>h|i"); got != "a_b_c_d_e_f_g_h_i" {
		t.Fatalf("slug 清洗错误: %q", got)
	}
	if got := slug("   "); got != "untitled" {
		t.Fatalf("空名回退错误: %q", got)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(dir, "x.txt"); got == p {
		t.Fatalf("uniquePath 应避开已存在文件: %s", got)
	}
}
