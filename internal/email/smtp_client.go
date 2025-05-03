package email

import (
	"crypto/tls"
	"fmt"
	"log"

	"gomodules.avm99963.com/zenithplanner/internal/config"

	gomail "gopkg.in/gomail.v2"
)

// Client handles sending emails via SMTP using gomail.
type Client struct {
	dialer *gomail.Dialer
	cfg    config.SMTPConfig
}

// NewClient creates a new SMTP email client using gomail.
func NewClient(cfg config.SMTPConfig) *Client {
	d := gomail.NewDialer(cfg.Host, cfg.Port, cfg.User, cfg.Password)

	// Handle skipping TLS verification if configured (USE WITH CAUTION,
	// the configuration is not documented due to its danger).
	if cfg.SkipTLSVerify {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		log.Println("Warning: Skipping SMTP TLS verification. You should NOT be using this. This is very dangerous.")
	}

	return &Client{
		dialer: d,
		cfg:    cfg,
	}
}

// SendConfirmation sends the appropriate confirmation email based on the type and changes.
func (c *Client) SendConfirmation(changes map[string]string) error {
	if c.dialer == nil || c.cfg.SenderAddress == "" || c.cfg.RecipientAddress == "" {
		log.Println("SMTP configuration incomplete or client not initialized, skipping email.")
		return nil
	}

	var subject, htmlBody string
	var err error

	subject, htmlBody, err = c.formatGenericSyncEmail(changes)

	if err != nil {
		return fmt.Errorf("failed to format email: %w", err)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", c.cfg.SenderAddress)
	m.SetHeader("To", c.cfg.RecipientAddress)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody) // Set HTML body

	if err := c.dialer.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email via SMTP: %w", err)
	}

	log.Printf("Successfully sent confirmation email to %s (Subject: %s)", c.cfg.RecipientAddress, subject)
	return nil
}
