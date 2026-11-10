package translate

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSogouSign(t *testing.T) {
	// 签名 = md5(from+to+text+salt) 的 hex（与 go-fy 的 otto JS 结果等价）
	sum := md5.Sum([]byte("auto" + "zh-CHS" + "test" + sogouSalt))
	want := hex.EncodeToString(sum[:])
	if got := sogouSign("auto", "zh-CHS", "test"); got != want {
		t.Fatalf("sogouSign = %q, want %q", got, want)
	}
}

func TestIsChinese(t *testing.T) {
	if !IsChinese("你好 world") {
		t.Error("含汉字应返回 true")
	}
	if IsChinese("hello world") {
		t.Error("纯英文应返回 false")
	}
}

func TestSogouConvert(t *testing.T) {
	if sogouConvert(Chinese) != "zh-CHS" {
		t.Fatalf("zh-CN 应映射为 zh-CHS: %s", sogouConvert(Chinese))
	}
	if sogouConvert("en") != "en" {
		t.Fatalf("en 不应被改写: %s", sogouConvert("en"))
	}
}

func TestSettingsDefaults(t *testing.T) {
	var s *Settings
	if s.ActiveName() != "sogou" || s.TargetOrDefault() != "zh-CN" {
		t.Fatalf("空设置应回落到 sogou/zh-CN: %s/%s", s.ActiveName(), s.TargetOrDefault())
	}
	if got := (&Settings{Active: "youdao", Target: "en"}).ActiveName(); got != "youdao" {
		t.Fatalf("Active 应生效: %s", got)
	}
}

func TestProviderKind(t *testing.T) {
	if (&Provider{Type: "http"}).kind("youdao") != "http" {
		t.Error("显式 type 应优先")
	}
	if (&Provider{}).kind("youdao") != "youdao" {
		t.Error("youdao 未写 type 时应推断为 youdao")
	}
	if (&Provider{}).kind("mine") != "http" {
		t.Error("未知名字应推断为 http")
	}
}

func TestMigrateLegacy(t *testing.T) {
	youdaoJSON, _ := json.Marshal(map[string]string{
		"app_key": "k", "app_secret": "s",
	})
	customJSON, _ := json.Marshal(map[string]any{
		"api_url": "https://x/api", "api_key": "sk", "result_path": "data.r",
	})
	s := MigrateLegacy(&Settings{}, "custom", "en", customJSON, youdaoJSON)

	if s.Active != "custom" || s.Target != "en" {
		t.Fatalf("迁移后 active/target 不对: %s/%s", s.Active, s.Target)
	}
	if p := s.Provider("youdao"); p == nil || p.Type != "youdao" || p.AppKey != "k" {
		t.Fatalf("有道未迁移: %+v", p)
	}
	p := s.Provider("custom")
	if p == nil || p.Type != "http" || p.URL != "https://x/api" || p.key("api_key") != "sk" {
		t.Fatalf("自定义源未迁移: %+v", p)
	}

	// 已有配置不应被覆盖
	exist := &Settings{Providers: map[string]*Provider{"youdao": {Type: "youdao", AppKey: "new"}}}
	s2 := MigrateLegacy(exist, "youdao", "zh-CN", nil, youdaoJSON)
	if s2.Provider("youdao").AppKey != "new" {
		t.Error("已有 provider 被旧配置覆盖")
	}
}

func TestTranslateUnknownSourceFallsBack(t *testing.T) {
	// 未知翻译源会被跳过，自动回退到 sogou（无网络时报全部失败错误也算正确）
	res, src, err := Translate("hi", Chinese, &Settings{Active: "nope"})
	if err == nil {
		if src != "sogou" {
			t.Fatalf("回退后应使用 sogou: %s", src)
		}
		if res == "" {
			t.Fatal("回退成功时应有译文")
		}
	}
}

func TestHTTPTranslate(t *testing.T) {
	// 本地假翻译服务器验证自定义源全链路：模板渲染、Keys 注入、result_path 提取
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			http.Error(w, "bad key", http.StatusUnauthorized)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["text"] != "hello" || body["to"] != "zh-CN" {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": "你好"}})
	}))
	defer srv.Close()

	s := &Settings{
		Active: "myapi",
		Providers: map[string]*Provider{
			"myapi": {
				Type:       "http",
				URL:        srv.URL,
				Keys:       map[string]string{"api_key": "sk-test"},
				Body:       `{"text": "{{.Text}}", "to": "{{.Target}}"}`,
				Headers:    map[string]string{"Authorization": "Bearer {{.Keys.api_key}}"},
				ResultPath: "data.result",
			},
		},
	}
	res, src, err := Translate("hello", Chinese, s)
	if err != nil {
		t.Fatalf("自定义源失败: %v", err)
	}
	if src != "myapi" {
		t.Fatalf("应使用 myapi 源: %s", src)
	}
	if res != "你好" {
		t.Fatalf("译文不对: %q", res)
	}
}

func TestHTTPTranslateSignTemplate(t *testing.T) {
	// 模板哈希函数：签名串 = md5(appid + text + salt + key)，与百度开放平台一致
	var gotSign, gotSalt, gotQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotQ, gotSalt, gotSign = q.Get("q"), q.Get("salt"), q.Get("sign")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"trans_result": []map[string]string{{"dst": "译文"}},
		})
	}))
	defer srv.Close()

	p := &Provider{
		Type: "http",
		URL:  srv.URL,
		Keys: map[string]string{"appid": "app1", "key": "secret1"},
		Query: map[string]string{
			"q":     "{{.Text}}",
			"to":    "{{.Target}}",
			"salt":  "{{.Salt}}",
			"appid": "{{.Keys.appid}}",
			"sign":  `{{md5 (printf "%s%s%s%s" .Keys.appid .Text .Salt .Keys.key)}}`,
		},
		ResultPath: "trans_result.0.dst",
	}
	out, err := HTTPTranslate(context.Background(), "hello", Chinese, p)
	if err != nil {
		t.Fatalf("签名模板请求失败: %v", err)
	}
	if out != "译文" {
		t.Fatalf("译文提取错误: %q", out)
	}
	if gotQ != "hello" || gotSalt == "" {
		t.Fatalf("参数渲染错误: q=%q salt=%q", gotQ, gotSalt)
	}
	sum := md5.Sum([]byte("app1" + "hello" + gotSalt + "secret1"))
	if want := hex.EncodeToString(sum[:]); gotSign != want {
		t.Fatalf("签名不对: got %q want %q", gotSign, want)
	}
}

func TestHTTPTranslateRequiresURL(t *testing.T) {
	if _, err := HTTPTranslate(context.Background(), "hi", Chinese, nil); err == nil {
		t.Fatal("nil provider 应报错")
	}
	if _, err := HTTPTranslate(context.Background(), "hi", Chinese, &Provider{Type: "http"}); err == nil {
		t.Fatal("没有 url 应报错")
	}
}

func TestNextSource(t *testing.T) {
	// 只有 sogou：NextSource 保持不动
	only := &Settings{}
	if got := only.NextSource(); got != "sogou" {
		t.Fatalf("只有 sogou 时应保持 sogou，got %s", got)
	}
	// sogou + youdao：来回切换（NextSource 是纯函数，不修改 Active）
	s := &Settings{Providers: map[string]*Provider{
		"youdao": {Type: "youdao", AppKey: "k", AppSecret: "v"},
	}}
	if got := s.NextSource(); got != "youdao" {
		t.Fatalf("应切到 youdao，got %s", got)
	}
	s.Active = "youdao"
	if got := s.NextSource(); got != "sogou" {
		t.Fatalf("应切回 sogou，got %s", got)
	}
	// 自定义源按名字排序参与循环；Active 是未配置的源时按默认位置（sogou）处理
	s2 := &Settings{
		Active: "gone",
		Providers: map[string]*Provider{
			"zeta":  {Type: "http", URL: "http://z"},
			"alpha": {Type: "http", URL: "http://a"},
			"nokey": {Type: "http"}, // 无 url，应跳过
		},
	}
	want := []string{"sogou", "alpha", "zeta"}
	if got := s2.AvailableSources(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("AvailableSources = %v, want %v", got, want)
	}
	if got := s2.NextSource(); got != "alpha" {
		t.Fatalf("Active 无效时按 sogou 位置循环，下一个应为 alpha，got %s", got)
	}
}

func TestLLMTranslate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-llm" {
			http.Error(w, "bad key", http.StatusUnauthorized)
			return
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "deepseek-chat" || len(body.Messages) != 2 {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if body.Messages[0].Role != "system" || body.Messages[1].Role != "user" ||
			body.Messages[1].Content != "hello world" {
			http.Error(w, "bad messages", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "```text\n你好，世界\n```"}},
			},
		})
	}))
	defer srv.Close()

	p := &Provider{Type: "llm", URL: srv.URL + "/chat/completions", Model: "deepseek-chat", AppKey: "sk-llm"}
	out, err := LLMTranslate(context.Background(), "hello world", Chinese, p)
	if err != nil {
		t.Fatalf("llm 翻译失败: %v", err)
	}
	if out != "你好，世界" {
		t.Fatalf("应去掉代码围栏得到译文，got %q", out)
	}
}

func TestLLMTranslateRequiresConfig(t *testing.T) {
	if _, err := LLMTranslate(context.Background(), "hi", Chinese, nil); err == nil {
		t.Fatal("nil provider 应报错")
	}
	if _, err := LLMTranslate(context.Background(), "hi", Chinese, &Provider{Type: "llm", URL: "http://x"}); err == nil {
		t.Fatal("缺 model 应报错")
	}
	if _, err := LLMTranslate(context.Background(), "hi", Chinese, &Provider{Type: "llm", URL: "http://x", Model: "m"}); err == nil {
		t.Fatal("缺 api_key 应报错")
	}
}

func TestTranslateSogouPath(t *testing.T) {
	// 仅保证调用路径不 panic；无网络时返回错误即可
	_, _, err := Translate("hello", Chinese, &Settings{})
	_ = err
}
