package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/mail"
	"net/smtp"
	"time"

	. "github.com/tj/go-debug"
)

var (
	debugMailer = Debug("reaper:mailer")
)

// at some point this should probably be configurable
const notifyTemplateSource = `
<html>
<body>
	<p>Your EC2 instance "{{.Name}}" ({{.Id}}) in {{.Region}} is scheduled to be terminated.</p>

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
	conf Config
}

func NewMailer(conf Config) *Mailer {
	return &Mailer{conf}
}

// Send an HTML email
func (m *Mailer) Send(to mail.Address, subject, htmlBody string) error {

	buf := bytes.NewBuffer(nil)
	buf.WriteString("From: " + m.conf.SMTP.From.Address.String() + "\n")
	buf.WriteString("To: " + to.String() + "\n")
	buf.WriteString("Subject: " + subject + "\n")
	buf.WriteString("MIME-Version: 1.0\n")
	buf.WriteString("Content-Type: text/html; charset=utf-8\n\n")
	buf.WriteString(htmlBody)
	buf.WriteString("\n")

	debugMailer("Sending email to:%s, from:%s, subject:%s",
		to.String(),
		m.conf.SMTP.From.Address.String(),
		subject)

	return smtp.SendMail(
		m.conf.SMTP.Addr(),
		m.conf.SMTP.Auth(),
		m.conf.SMTP.From.Address.Address,
		[]string{to.Address},
		buf.Bytes(),
	)
}

func (m *Mailer) Notify(notifyNum int, i *Instance) (err error) {

	if i.Owner() == nil {
		return fmt.Errorf("instance %i has no owner to notify", i.Id())
	}

	terminateDate := time.Now()

	switch notifyNum {
	case 1:
		terminateDate.
			Add(m.conf.Reaper.SecondNotification.Duration).
			Add(m.conf.Reaper.Terminate.Duration)
	case 2:
		terminateDate.
			Add(m.conf.Reaper.Terminate.Duration)
	default:
		return fmt.Errorf("%d not a valid notification num", notifyNum)
	}

	var term, delay1, delay3, delay7 string
	// Token strings

	term, err = MakeTerminateLink(m.conf.TokenSecret,
		m.conf.HTTPApiURL, i.Region(), i.Id())

	debugMailer("creating links for %s:%s", i.Id(), i.Region())

	if err == nil {
		delay1, err = MakeIgnoreLink(m.conf.TokenSecret,
			m.conf.HTTPApiURL, i.Region(), i.Id(), time.Duration(24*time.Hour))
	}

	if err == nil {
		delay3, err = MakeIgnoreLink(m.conf.TokenSecret,
			m.conf.HTTPApiURL, i.Region(), i.Id(), time.Duration(3*24*time.Hour))
	}

	if err == nil {
		delay7, err = MakeIgnoreLink(m.conf.TokenSecret,
			m.conf.HTTPApiURL, i.Region(), i.Id(), time.Duration(7*24*time.Hour))
	}

	if err != nil {
		return err
	}

	mtvLoc, errt := time.LoadLocation("PST8PDT")

	if errt != nil {
		return errt
	}

	dispTime := terminateDate.In(mtvLoc).Truncate(time.Hour).Format(time.RFC1123)
	buf := bytes.NewBuffer(nil)
	err = notifyTemplate.Execute(buf, map[string]string{
		"Id":            i.Id(),
		"Name":          i.Name(),
		"Host":          m.conf.HTTPApiURL,
		"Region":        i.Region(),
		"TerminateDate": dispTime,
		"terminateLink": term,
		"delay1DLink":   delay1,
		"delay3DLink":   delay3,
		"delay7DLink":   delay7,
	})

	if err != nil {
		debugMailer("Template generation error %s", err.Error())
		return err
	}

	iName := i.Name()
	if iName == "" {
		iName = "*unnamed*"
	}

	subject := fmt.Sprintf("Your instance %s (%s) will be terminated soon", iName, i.Id())
	return m.Send(*i.Owner(), subject, string(buf.Bytes()))
}
