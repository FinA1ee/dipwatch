package notify

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/url"
	"strings"
	"testing"
)

func TestCheckWebhookAllowsOnlyFeishuAndLark(t *testing.T) {
	ok := []string{
		"https://open.feishu.cn/open-apis/bot/v2/hook/abc",
		"https://open.larksuite.com/open-apis/bot/v2/hook/abc",
	}
	bad := []string{
		"http://open.feishu.cn/open-apis/bot/v2/hook/abc",        // not https
		"https://open.feishu.cn.evil.example/open-apis/bot/v2/x", // look-alike host
		"https://evil.example/open-apis/bot/v2/hook/abc",
		"https://user:pw@open.feishu.cn/open-apis/bot/v2/hook/a", // credentials in URL
		"https://open.feishu.cn:8443/open-apis/bot/v2/hook/abc",  // other port
		"https://open.feishu.cn/other/path",
		"not a url",
	}
	for _, u := range ok {
		if err := CheckWebhook(u); err != nil {
			t.Errorf("%s refused: %v", u, err)
		}
	}
	for _, u := range bad {
		if CheckWebhook(u) == nil {
			t.Errorf("%s accepted", u)
		}
	}
}

func TestFeishuRefusesOtherHostsWithoutCalling(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	if err := (Feishu{Client: srv.Client()}).Send(context.Background(), srv.URL+"/open-apis/bot/v2/hook/x", "hi"); err == nil || called {
		t.Fatalf("sent to %s: err %v, called %v", srv.URL, err, called)
	}
}

// feishuAt points the allowed host at a test server, so Send's request and
// error handling run for real.
func feishuAt(t *testing.T, srv *httptest.Server) Feishu {
	t.Helper()
	target, _ := url.Parse(srv.URL)
	client := srv.Client()
	transport := client.Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	base := transport.DialContext
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return base(ctx, network, target.Host)
	}
	transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // test server certificate
	return Feishu{Client: &http.Client{Transport: transport}}
}

func TestFeishuSendsTextAndReportsErrors(t *testing.T) {
	var got map[string]any
	code := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": "token invalid"})
	}))
	defer srv.Close()
	f := feishuAt(t, srv)
	hook := "https://open.feishu.cn/open-apis/bot/v2/hook/test"

	if err := f.Send(context.Background(), hook, "RTX 5090 dropped"); err != nil {
		t.Fatal(err)
	}
	if got["msg_type"] != "text" || got["content"].(map[string]any)["text"] != "RTX 5090 dropped" {
		t.Fatalf("body: %v", got)
	}
	code = 19021
	if err := f.Send(context.Background(), hook, "x"); err == nil || !strings.Contains(err.Error(), "token invalid") {
		t.Fatalf("error code not reported: %v", err)
	}
}

func TestMessageEncodesSubjectAndBlocksHeaderInjection(t *testing.T) {
	from := mail.Address{Name: "Dipwatch", Address: "a@example.test"}
	to := mail.Address{Address: "b@example.test"}
	msg := string(Message(from, to, "索尼 降价\r\nBcc: x@evil.example", "line 1\nline 2"))
	if strings.Contains(msg, "\r\nBcc:") {
		t.Fatalf("header injected:\n%s", msg)
	}
	if !strings.Contains(msg, "Subject: =?UTF-8?b?") && !strings.Contains(msg, "Subject: =?UTF-8?B?") {
		t.Fatalf("subject not encoded:\n%s", msg)
	}
	if !strings.HasSuffix(msg, "line 1\r\nline 2") {
		t.Fatalf("body line endings:\n%q", msg)
	}
}

func TestNewMailerNeedsSMTPSAndSender(t *testing.T) {
	if _, err := NewMailer("smtps://u%40qq.com:code@smtp.qq.com:465", "Dipwatch <u@qq.com>"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][2]string{{"smtp://u:p@h:25", "a@b.test"}, {"smtps://h:465", "a@b.test"}, {"smtps://u:p@h:465", "not an address"}} {
		if _, err := NewMailer(bad[0], bad[1]); err == nil {
			t.Errorf("accepted %v", bad)
		}
	}
}
