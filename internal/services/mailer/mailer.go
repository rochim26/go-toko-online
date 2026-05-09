package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/tokoonline/app/internal/services/settings"
)

type Mailer struct{ s *settings.Store }

func New(s *settings.Store) *Mailer { return &Mailer{s: s} }

// Send delivers an HTML email. Falls back to local postfix at 127.0.0.1:25 (no auth)
// when SMTP host is empty in settings.
func (m *Mailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	if to == "" {
		return nil
	}
	cfg := m.s.Mailer()
	st := m.s.Store()
	host := cfg.SMTPHost
	port := cfg.SMTPPort
	user := cfg.SMTPUser
	pass := cfg.SMTPPass
	fromEmail := cfg.FromEmail
	fromName := cfg.FromName
	if fromEmail == "" {
		fromEmail = "noreply@toko.mdt.biz.id"
	}
	if fromName == "" {
		fromName = st.Name
		if fromName == "" {
			fromName = "Toko"
		}
	}
	useLocalPostfix := host == ""
	if useLocalPostfix {
		host = "127.0.0.1"
		port = 25
	}
	if port == 0 {
		port = 587
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	from := fmt.Sprintf("%s <%s>", fromName, fromEmail)
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + encodeSubject(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	msg := strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody

	if useLocalPostfix {
		return sendLocalSMTP(addr, fromEmail, to, []byte(msg))
	}

	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	return smtp.SendMail(addr, auth, fromEmail, []string{to}, []byte(msg))
}

// sendLocalSMTP delivers via loopback postfix without STARTTLS verification.
// Postfix on loopback typically presents a self-signed cert that Go's default
// SendMail rejects; for 127.0.0.1 traffic we either skip TLS or skip-verify.
func sendLocalSMTP(addr, from, to string, msg []byte) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	host, _, _ := net.SplitHostPort(addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		// Skip-verify is fine: traffic never leaves loopback.
		if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true, ServerName: host}); err != nil {
			// Fall back to plain (postfix accepts this on loopback by default)
			// New connection
			conn2, err2 := net.Dial("tcp", addr)
			if err2 != nil {
				return err2
			}
			c2, err2 := smtp.NewClient(conn2, host)
			if err2 != nil {
				return err2
			}
			defer c2.Close()
			c = c2
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// SendAsync sends in goroutine, ignoring errors (best-effort transactional).
func (m *Mailer) SendAsync(ctx context.Context, to, subject, htmlBody string) {
	go func() {
		_ = m.Send(context.Background(), to, subject, htmlBody)
	}()
}

func encodeSubject(s string) string {
	for _, r := range s {
		if r > 127 {
			return "=?UTF-8?B?" + b64(s) + "?="
		}
	}
	return s
}

func b64(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	out := make([]byte, 0, ((len(s)+2)/3)*4)
	src := []byte(s)
	for i := 0; i < len(src); i += 3 {
		var b [3]byte
		n := copy(b[:], src[i:])
		v := uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
		out = append(out, alphabet[(v>>18)&63], alphabet[(v>>12)&63])
		if n >= 2 {
			out = append(out, alphabet[(v>>6)&63])
		} else {
			out = append(out, '=')
		}
		if n >= 3 {
			out = append(out, alphabet[v&63])
		} else {
			out = append(out, '=')
		}
	}
	return string(out)
}
