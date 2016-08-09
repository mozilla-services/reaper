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

// Mailer implements EventReporter, sends email
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
	*EventReporterConfig

	CopyEmailAddresses []string

	Host     string
	Port     int
	AuthType string
	Username string
	Password string
	From     FromAddress
}

// SetDryRun is a method of EventReporter
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

// NewReapableEvent is a method of EventReporter
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

// NewBatchReapableEvent is a method of EventReporter
func (e *Mailer) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	errorStrings := []string{}
	buffer := new(bytes.Buffer)

	// owner is the same for all of these resources
	owner, _, err := rs[0].ReapableEventEmailShort()
	if err != nil {
		return fmt.Errorf("Error getting resource owner with ReapableEventEmailShort: %s", err)
	}

	subject := fmt.Sprintf("AWS Resources you own are going to be reaped!")
	buffer.WriteString(
		fmt.Sprintf("You are receiving this message because your email, "+
			"%s, is associated with AWS resources that matched Reaper's filters.\n"+
			"If you do not take action they will be stopped and then terminated!\n", owner.Address))

	// if none of these resources should trigger, we shouldn't send an email
	triggering := false
	for _, r := range rs {
		if !e.Config.shouldTriggerFor(r) {
			continue
		}
		triggering = true
		_, body, err := r.ReapableEventEmailShort()
		errorStrings = append(errorStrings, fmt.Sprintf("ReapableEventEmailShort: %s", err))
		buffer.ReadFrom(body)
		buffer.WriteString("\n")
	}
	if triggering {
		return e.send(owner, subject, buffer)
	}
	return nil
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

// GetConfig is a method of EventReporter
func (e *Mailer) GetConfig() EventReporterConfig {
	return *e.Config.EventReporterConfig
}

// NewCountStatistic is a method of EventReporter
func (e *Mailer) NewCountStatistic(string, []string) error {
	return nil
}

// NewStatistic is a method of EventReporter
func (e *Mailer) NewStatistic(string, float64, []string) error {
	return nil
}

// NewEvent is a method of EventReporter
func (e *Mailer) NewEvent(string, string, map[string]string, []string) error {
	return nil
}
