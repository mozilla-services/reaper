package main

// Some example code demonistrating creating and sending of emails via SMTP

import (
	"bytes"
	"flag"
	"fmt"
	"net/smtp"
	"os"
	"text/template"

	"github.com/mostlygeek/reaper"
	. "github.com/tj/go-debug"
)

var (
	log   = &reaper.Logger{"EC2"}
	Conf  *reaper.Config
	debug = Debug("reaper:EC2")
	from  = ""
	to    = ""
)

type EmailData struct {
	From    string
	To      string
	Subject string
	Body    string
}

const emailTemplate = `From: {{.From}}
To: {{.To}}
Subject: {{.Subject}}

{{.Body}}
`

func init() {
	var configFile string

	flag.StringVar(&configFile, "conf", "", "path to config file")
	flag.StringVar(&from, "from", "somebody@stage.mozaws.net", "email sender")
	flag.StringVar(&to, "to", "nobody@localhost", "email recipient")

	flag.Parse()

	if configFile == "" {
		log.Err("Config file required", configFile)
		os.Exit(1)
	}

	if c, err := reaper.LoadConfig(configFile); err == nil {
		Conf = c
		log.Info("Configuration loaded from", configFile)
		debug("SMTP Config: %s", Conf.SMTP.String())
	} else {
		log.Err("toml", err)
		os.Exit(1)
	}

}

func main() {
	context := EmailData{
		From:    from,
		To:      to,
		Subject: "Reaper notice",
		Body:    "Hello...",
	}

	t := template.Must(template.New("email").Parse(emailTemplate))
	var buf bytes.Buffer
	if err := t.Execute(&buf, context); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err := smtp.SendMail(Conf.SMTP.Addr(), Conf.SMTP.Auth(), from, []string{to}, buf.Bytes())
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("OK! Email sent")
		fmt.Println("--------------")
		fmt.Println(string(buf.Bytes()))
	}
}
