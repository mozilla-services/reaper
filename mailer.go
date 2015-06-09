package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/mail"
	"net/smtp"
	"time"
)

// at some point this should probably be configurable
const notifyTemplateSource = `
<html>
<body>
	<p>Your EC2 instance {{ if .Name }}"{{.Name}}" {{ end }}{{.ID}} in {{.Region}} is scheduled to be terminated.</p>

	<p>
		You may ignore this message and your instance will be automatically
		terminated after <strong>{{.TerminateDate}}</strong>.
	</p>

	<p>
		You may also choose to:
		<ul>
			<li><a href="{{.terminateLink}}">Terminate it now</a></li>
			<li><a href="{{.delay1DLink}}">Ignore it for 1 more day</a></li>
			<li><a href="{{.delay3DLink}}">Ignore it for 3 more days</a></li>
			<li><a href="{{.delay7DLink}}">Ignore it for 7 more days</a></li>
		</ul>
	</p>

	<p>
		If you want the Reaper to ignore this instance tag it with REAPER_SPARE_ME with any value.
	</p>
</body>
</html>
`

var (
	notifyTemplate *template.Template
)

func init() {
	notifyTemplate = template.Must(
		template.New("notifyTemplate").Parse(notifyTemplateSource))
}

type Mailer struct {
	conf *Config
}

func NewMailer(conf *Config) *Mailer {
	return &Mailer{conf}
}

// methods to conform to EventReporter interface
func (m *Mailer) NewEvent(title string, text string, fields map[string]string, tags []string) {}
func (m *Mailer) NewStatistic(name string, value float64, tags []string)                      {}
func (m *Mailer) NewCountStatistic(name string, tags []string)                                {}
func (m *Mailer) NewReapableInstanceEvent(i *Instance) {
	// don't send emails if we're on a dry run
	if Conf.DryRun {
		return
	}

	if err := m.Notify(&i.AWSResource); err != nil {
		Log.Error(err.Error())
	}
}

func (m *Mailer) NewReapableASGEvent(a *AutoScalingGroup) {
	// don't send emails if we're on a dry run
	if Conf.DryRun {
		return
	}

	if err := m.Notify(&a.AWSResource); err != nil {
		Log.Error(err.Error())
	}
}

// Send an HTML email
func (m *Mailer) Send(to mail.Address, subject, htmlBody string) error {

	buf := bytes.NewBuffer(nil)
	buf.WriteString("From: " + m.conf.Events.Email.From.Address.String() + "\n")
	buf.WriteString("To: " + to.String() + "\n")
	buf.WriteString("Subject: " + subject + "\n")
	buf.WriteString("MIME-Version: 1.0\n")
	buf.WriteString("Content-Type: text/html; charset=utf-8\n\n")
	buf.WriteString(htmlBody)
	buf.WriteString("\n")

	Log.Debug("Sending email to: \"%s\", from: \"%s\", subject: \"%s\"",
		to.String(),
		m.conf.Events.Email.From.Address.String(),
		subject)

	return smtp.SendMail(
		m.conf.Events.Email.Addr(),
		m.conf.Events.Email.Auth(),
		m.conf.Events.Email.From.Address.Address,
		[]string{to.Address},
		buf.Bytes(),
	)
}

func (m *Mailer) Notify(a *AWSResource) (err error) {
	if a.Owner() == nil {
		Log.Debug("Resource %s has no owner to notify.", a.ID)
		return nil
	}

	terminateDate := a.ReaperState.Until

	var term, delay1, delay3, delay7, whitelist string
	// Token strings

	term, err = MakeTerminateLink(m.conf.TokenSecret,
		m.conf.HTTPApiURL, a.Region, a.ID)

	if err == nil {
		delay1, err = MakeIgnoreLink(m.conf.TokenSecret,
			m.conf.HTTPApiURL, a.Region, a.ID, time.Duration(24*time.Hour))
	}

	if err == nil {
		delay3, err = MakeIgnoreLink(m.conf.TokenSecret,
			m.conf.HTTPApiURL, a.Region, a.ID, time.Duration(3*24*time.Hour))
	}

	if err == nil {
		delay7, err = MakeIgnoreLink(m.conf.TokenSecret,
			m.conf.HTTPApiURL, a.Region, a.ID, time.Duration(7*24*time.Hour))
	}

	if err != nil {
		return err
	}

	mtvLoc, err := time.LoadLocation("PST8PDT")

	if err != nil {
		return err
	}

	dispTime := terminateDate.In(mtvLoc).Truncate(time.Hour).Format(time.RFC1123)
	buf := bytes.NewBuffer(nil)
	err = notifyTemplate.Execute(buf, map[string]string{
		"Id":            a.ID,
		"Name":          a.Name,
		"Host":          m.conf.HTTPApiURL,
		"Region":        a.Region,
		"TerminateDate": dispTime,
		"terminateLink": term,
		"delay1DLink":   delay1,
		"delay3DLink":   delay3,
		"delay7DLink":   delay7,
		"whitelist":     whitelist,
	})

	if err != nil {
		Log.Debug("Template generation error %s", err.Error())
		return err
	}

	iName := a.Name
	if iName == "" {
		iName = "*unnamed*"
	}

	subject := fmt.Sprintf("Your instance %s (%s) will be terminated soon", iName, a.ID)
	return m.Send(*a.Owner(), subject, string(buf.Bytes()))
}
