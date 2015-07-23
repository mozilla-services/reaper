package events

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/smtp"

	"github.com/jordan-wright/email"

	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
)

type Mailer struct {
	Config *SMTPConfig
}

type HTTPConfig struct {
	TokenSecret string
	ApiURL      string
	Listen      string
	Token       string
	Action      string
}

type SMTPConfig struct {
	HTTPConfig
	EventReporterConfig

	CopyEmailAddresses []string

	Host     string
	Port     int
	AuthType string
	Username string
	Password string
	From     FromAddress
}

func (m *Mailer) SetDryRun(b bool) {
	m.Config.DryRun = b
}

func (s *SMTPConfig) String() string {
	return fmt.Sprintf("%s:%d auth type:%s, creds: %s:%s",
		s.Host,
		s.Port,
		s.AuthType,
		s.Username,
		s.Password)
}
func (s *SMTPConfig) Addr() string {
	if s.Port == 0 {
		// friends don't let friend's smtp over port 25
		return fmt.Sprintf("%s:%d", s.Host, 587)
	}
	// default
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// Auth creates the appropriate smtp.Auth from the configured AuthType
func (s *SMTPConfig) Auth() smtp.Auth {
	switch s.AuthType {
	case "md5":
		return s.CRAMMD5Auth()
	case "plain":
		return s.PlainAuth()
	default:
		return nil
	}
}

func (s *SMTPConfig) CRAMMD5Auth() smtp.Auth {
	return smtp.CRAMMD5Auth(s.Username, s.Password)
}

func (s *SMTPConfig) PlainAuth() smtp.Auth {
	return smtp.PlainAuth("", s.Username, s.Password, s.Host)
}

type FromAddress struct {
	mail.Address
}

func (f *FromAddress) UnmarshalText(text []byte) error {
	a, err := mail.ParseAddress(string(text))
	if err != nil {
		return err
	}

	f.Address = *a
	return nil
}

func NewMailer(c *SMTPConfig) *Mailer {
	c.Name = "Mailer"
	return &Mailer{c}
}

// methods to conform to EventReporter interface
func (m *Mailer) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (m *Mailer) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (m *Mailer) NewCountStatistic(name string, tags []string) error {
	return nil
}

// TODO: figure out how to goroutine this
func (m *Mailer) NewReapableEvent(r Reapable) error {
	if m.Config.ShouldTriggerFor(r) {
		addr, subject, body, err := r.ReapableEventEmail()
		if err != nil {
			// if this is an unowned error we don't pass it up
			switch t := err.(type) {
			case reapable.UnownedError:
				log.Error(t.Error())
				return nil
			default:
			}
			return err
		}
		return m.Send(addr, subject, body)
	}
	return nil
}

func (e *Mailer) NewBatchReapableEvent(rs []Reapable) error {
	var triggering []Reapable
	for _, r := range rs {
		if e.Config.ShouldTriggerFor(r) {
			triggering = append(triggering, r)
		}
	}
	if len(triggering) == 0 {
		return nil
	}

	buffer := *bytes.NewBuffer(nil)
	owner, _, err := rs[0].ReapableEventEmailShort()
	if err != nil {
		return err
	}
	log.Info("Sending batch Mailer event for %d reapables.", len(triggering))
	subject := fmt.Sprintf("%d AWS Resources you own are going to be reaped!", len(triggering))
	buffer.WriteString(fmt.Sprintf("You are receiving this message because your email, %s, is associated with AWS resources that matched Reaper's filters.\nIf you do not take action they will be terminated!", owner.Address))
	for _, r := range triggering {
		_, body, err := r.ReapableEventEmailShort()
		if err != nil {
			return err
		}
		buffer.ReadFrom(body)
		buffer.WriteString("\n")
	}

	return e.Send(owner, subject, &buffer)
}

// Send an HTML email
func (m *Mailer) Send(to mail.Address, subject string, htmlBody *bytes.Buffer) error {
	log.Debug("Sending email to: \"%s\", from: \"%s\", subject: \"%s\"",
		to.String(),
		m.Config.From.Address.String(),
		subject)

	e := email.NewEmail()
	e.From = m.Config.From.Address.String()
	e.To = []string{to.Address}
	e.Bcc = m.Config.CopyEmailAddresses
	e.Subject = subject
	e.HTML = htmlBody.Bytes()

	return e.Send(m.Config.Addr(), m.Config.Auth())
}
