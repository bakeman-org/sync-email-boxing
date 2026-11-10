package config

import "testing"

func TestWebmailURL(t *testing.T) {
	cases := map[string]string{
		"a@qq.com":      "https://mail.qq.com",
		"b@foxmail.com": "https://mail.qq.com",
		"b@163.com":     "https://mail.163.com",
		"c@126.com":     "https://mail.126.com",
		"d@gmail.com":   "https://mail.google.com",
		"e@gmail.cn":    "https://mail.gmail.cn",
		"f@outlook.com": "https://outlook.live.com/mail",
	}
	for email, want := range cases {
		a := Account{Email: email}
		if a.WebmailURL() != want {
			t.Fatalf("%s => %s, want %s", email, a.WebmailURL(), want)
		}
	}
}
