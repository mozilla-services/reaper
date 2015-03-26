package reaper

import (
	"bytes"
	"net/mail"
	"net/smtp"

	. "github.com/tj/go-debug"
)

var (
	debug = Debug("reaper:mailer")
)

type Mailer struct {
	conf SMTPConfig
}

func NewMailer(conf SMTPConfig) *Mailer {
	debug("Creating new mailer with config:%s", conf.String())
	return &Mailer{conf}
}

// Send an HTML email
func (m *Mailer) Send(to mail.Address, subject, htmlBody string) error {

	buf := bytes.NewBuffer(nil)
	buf.WriteString("From: " + m.conf.From.Address.String() + "\n")
	buf.WriteString("To: " + to.String() + "\n")
	buf.WriteString("Subject: " + subject + "\n")
	buf.WriteString("MIME-Version: 1.0\n")
	buf.WriteString("Content-Type: text/html; charset=utf-8\n\n")
	buf.WriteString(htmlBody)
	buf.WriteString("\n")

	debug("Sending email to:%s, from:%s, subject:%s",
		to.String(),
		m.conf.From.Address.String(),
		subject)

	return smtp.SendMail(
		m.conf.Addr(),
		m.conf.Auth(),
		m.conf.From.Address.Address,
		[]string{to.Address},
		buf.Bytes(),
	)
}
