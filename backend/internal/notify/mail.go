package notify

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Mailer delivers a login password by e-mail — the alternative to SMS/
// WhatsApp for staff who have an e-mail address. Like Notifier, it falls back
// to logging when no SMTP server is configured, so the flow works in dev.
type Mailer interface {
	SendPasswordEmail(ctx context.Context, to, name, password, loginURL string) error
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func NewMailer(cfg SMTPConfig) Mailer {
	if strings.TrimSpace(cfg.Host) == "" {
		return logMailer{}
	}
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	if cfg.From == "" {
		cfg.From = cfg.Username
	}
	return &smtpMailer{cfg: cfg, send: smtp.SendMail}
}

type logMailer struct{}

func (logMailer) SendPasswordEmail(_ context.Context, to, _, password, loginURL string) error {
	log.Printf("[notify:stub] email para %s: sua senha de acesso e %s, entre em %s (SMTP_HOST nao configurado, e-mail nao enviado de verdade)", to, password, loginURL)
	return nil
}

type smtpMailer struct {
	cfg  SMTPConfig
	send func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// SendPasswordEmail uses smtp.SendMail, which upgrades to STARTTLS whenever
// the server offers it; PLAIN auth is only sent over TLS (or to localhost).
func (m *smtpMailer) SendPasswordEmail(ctx context.Context, to, name, password, loginURL string) error {
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	msg := BuildPasswordEmail(m.cfg.From, to, name, password, loginURL, time.Now())
	done := make(chan error, 1)
	go func() { done <- m.send(net.JoinHostPort(m.cfg.Host, m.cfg.Port), auth, m.cfg.From, []string{to}, msg) }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("enviar e-mail: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BuildPasswordEmail renders the plain-text message (UTF-8, Portuguese)
// sent when a manager grants access by e-mail.
func BuildPasswordEmail(from, to, name, password, loginURL string, now time.Time) []byte {
	var b bytes.Buffer
	header := func(key, value string) { fmt.Fprintf(&b, "%s: %s\r\n", key, value) }
	header("From", from)
	header("To", to)
	header("Subject", mime.QEncoding.Encode("utf-8", "Seu acesso ao BarberFlow"))
	header("Date", now.Format(time.RFC1123Z))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "8bit")
	b.WriteString("\r\n")
	greeting := "Olá"
	if strings.TrimSpace(name) != "" {
		greeting += ", " + strings.TrimSpace(name)
	}
	fmt.Fprintf(&b, "%s!\r\n\r\n", greeting)
	b.WriteString("Seu acesso ao aplicativo da barbearia foi liberado.\r\n\r\n")
	fmt.Fprintf(&b, "Endereço: %s\r\n", loginURL)
	fmt.Fprintf(&b, "E-mail: %s\r\n", to)
	fmt.Fprintf(&b, "Senha temporária: %s\r\n\r\n", password)
	b.WriteString("Na tela de entrada, escolha \"Entrar com e-mail\". Depois de entrar, troque a senha em Perfil > Alterar senha.\r\n")
	return b.Bytes()
}
