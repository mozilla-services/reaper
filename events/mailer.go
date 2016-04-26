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

// Mailer implements ReapableEventReporter, sends email
// uses godspeed, requires dd-agent running
type Mailer struct {
	Config *MailerConfig
}

// HTTPConfig is the configuration for the HTTP server
// probably shouldn't be in Events, but it would need its own package + circular imports...
type HTTPConfig struct {
	TokenSecret string
	APIURL      string
	Listen      string
	Token       string
	Action      string
}

// MailerConfig is the configuration for a Mailer
type MailerConfig struct {
	HTTPConfig
	*eventReporterConfig

	CopyEmailAddresses []string

	Host     string
	Port     int
	AuthType string
	Username string
	Password string
	From     FromAddress
}

// SetDryRun is a method of ReapableEventReporter
func (e *Mailer) SetDryRun(b bool) {
	e.Config.DryRun = b
}

// String representation of MailerConfig
func (c *MailerConfig) String() string {
	return fmt.Sprintf("%s:%d auth type:%s, creds: %s:%s",
		c.Host,
		c.Port,
		c.AuthType,
		c.Username,
		c.Password)
}

// Addr returns the string representation of the MailerConfig's address
func (c *MailerConfig) Addr() string {
	if c.Port == 0 {
		// friends don't let friends smtp over port 25
		return fmt.Sprintf("%s:%d", c.Host, 587)
	}
	// default
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Auth creates the appropriate smtp.Auth from the configured AuthType
func (c *MailerConfig) Auth() smtp.Auth {
	switch c.AuthType {
	case "md5":
		return c.CRAMMD5Auth()
	case "plain":
		return c.PlainAuth()
	default:
		return nil
	}
}

// CRAMMD5Auth configures CRAMMD5Auth for MailerConfig
func (c *MailerConfig) CRAMMD5Auth() smtp.Auth {
	return smtp.CRAMMD5Auth(c.Username, c.Password)
}

// PlainAuth configures PlainAuth for MailerConfig
func (c *MailerConfig) PlainAuth() smtp.Auth {
	return smtp.PlainAuth("", c.Username, c.Password, c.Host)
}

// FromAddress is an alias for mail.Address
type FromAddress mail.Address

// UnmarshalText parses []byte -> Address string
func (f *FromAddress) UnmarshalText(text []byte) error {
	a, err := mail.ParseAddress(string(text))
	if err != nil {
		return err
	}
	f.Address = a.Address
	f.Name = a.Name
	return nil
}

// NewMailer is a constructor for Mailers
func NewMailer(c *MailerConfig) *Mailer {
	c.Name = "Mailer"
	return &Mailer{c}
}

// NewReapableEvent is a method of ReapableEventReporter
func (e *Mailer) NewReapableEvent(r Reapable, tags []string) error {
	if e.Config.shouldTriggerFor(r) {
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
		return e.send(addr, subject, body)
	}
	return nil
}

// NewBatchReapableEvent is a method of ReapableEventReporter
func (e *Mailer) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	var triggering []Reapable
	for _, r := range rs {
		if e.Config.shouldTriggerFor(r) {
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

	return e.send(owner, subject, &buffer)
}

// Send an HTML email
func (e *Mailer) send(to mail.Address, subject string, htmlBody *bytes.Buffer) error {
	log.Debug("Sending email to: \"%s\", from: \"%s\", subject: \"%s\"",
		to.String(),
		e.Config.From.Address,
		subject)

	m := email.NewEmail()
	m.From = e.Config.From.Address
	m.To = []string{to.Address}
	m.Bcc = e.Config.CopyEmailAddresses
	m.Subject = subject
	m.HTML = htmlBody.Bytes()

	return m.Send(e.Config.Addr(), e.Config.Auth())
}
