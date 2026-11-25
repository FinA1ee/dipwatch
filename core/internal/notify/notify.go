// Package notify delivers alerts: Feishu or Lark bot webhooks (spec T6) and
// email over SMTP when it is configured.
package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

// Hosts a Feishu webhook may point at (T6). Anything else is refused when saved
// and again before sending, so a stored URL can never reach another host.
var webhookHosts = map[string]bool{"open.feishu.cn": true, "open.larksuite.com": true}

// ErrWebhookHost means the URL is not a Feishu or Lark bot webhook.
var ErrWebhookHost = errors.New("webhook must be an https://open.feishu.cn or https://open.larksuite.com bot URL")

// CheckWebhook accepts only https bot webhooks on the Feishu or Lark hosts.
func CheckWebhook(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" ||
		!webhookHosts[strings.ToLower(u.Hostname())] || !strings.HasPrefix(u.Path, "/open-apis/bot/") {
		return ErrWebhookHost
	}
	return nil
}

// SendError means the webhook answered but did not accept the message.
type SendError struct {
	Status, Code int
	Msg          string
}

func (e *SendError) Error() string {
	return fmt.Sprintf("status %d, code %d: %s", e.Status, e.Code, e.Msg)
}

// Feishu posts text messages to bot webhooks.
type Feishu struct {
	Client *http.Client // nil: a client with a 10 s timeout
}

// Send posts one text message. Feishu answers 200 with a non-zero code on errors.
func (f Feishu) Send(ctx context.Context, webhook, text string) error {
	if err := CheckWebhook(webhook); err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"msg_type": "text", "content": map[string]string{"text": text}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// Never follow a redirect: it could lead away from the allowed hosts.
	noRedirect := *client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := noRedirect.Do(req)
	if err != nil {
		return fmt.Errorf("feishu: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	var answer struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(raw, &answer)
	if res.StatusCode != http.StatusOK || answer.Code != 0 {
		return &SendError{Status: res.StatusCode, Code: answer.Code, Msg: answer.Msg}
	}
	return nil
}

// Mailer sends plain-text email over SMTP with implicit TLS (port 465), e.g.
// SMTP_URL=smtps://you%40qq.com:<app code>@smtp.qq.com:465.
type Mailer struct {
	host, addr string
	auth       smtp.Auth
	from       mail.Address
}

// NewMailer parses SMTP_URL and MAIL_FROM. Both are required.
func NewMailer(smtpURL, from string) (*Mailer, error) {
	u, err := url.Parse(smtpURL)
	if err != nil || u.Scheme != "smtps" || u.Hostname() == "" || u.User == nil {
		return nil, errors.New("SMTP_URL must look like smtps://user:password@host:465")
	}
	sender, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("MAIL_FROM: %w", err)
	}
	port := u.Port()
	if port == "" {
		port = "465"
	}
	password, _ := u.User.Password()
	return &Mailer{
		host: u.Hostname(),
		addr: net.JoinHostPort(u.Hostname(), port),
		auth: smtp.PlainAuth("", u.User.Username(), password, u.Hostname()),
		from: *sender,
	}, nil
}

func (m *Mailer) Send(ctx context.Context, to, subject, text string) error {
	rcpt, err := mail.ParseAddress(to)
	if err != nil {
		return err
	}
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 15 * time.Second}, Config: &tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}}
	conn, err := dialer.DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp: %w", err)
	}
	defer func() { _ = c.Close() }()
	if err := c.Auth(m.auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := c.Mail(m.from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(rcpt.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(Message(m.from, *rcpt, subject, text)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// Message builds an RFC 5322 plain-text message. Header values come from our
// own templates and parsed addresses; newlines are stripped so none can inject headers.
func Message(from, to mail.Address, subject, text string) []byte {
	clean := strings.NewReplacer("\r", " ", "\n", " ")
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\n", from.String(), to.String(), mime.BEncoding.Encode("UTF-8", clean.Replace(subject)))
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.ReplaceAll(text, "\n", "\r\n"))
	return b.Bytes()
}
