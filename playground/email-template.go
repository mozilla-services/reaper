package main

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/mostlygeek/reaper/token"
)

const T_DATA = `
<html>
<body>
	<p>Ahoy</p>,

	<p>Your EC2 instance "{{.Name}}" ({{.Id}}) is scheduled to be terminated.</p>

	<p>
		You may ignore this message and your instance will automatically be
		reaped after <strong>{{.TerminateDate}}</strong>.
	</p>

	<p>
		You may also choose to:
		<ul>
			<li><a href="{{.Host}}/?a=terminate&t={{.TerminateToken | urlquery}}">Terminate it now</a></li>
			<li><a href="{{.Host}}/?a=delay_1dayd&t={{.Delay1DToken | urlquery}}">Ignore it for 1 more day</a></li>
			<li><a href="{{.Host}}/?a=delay_3day&t={{.Delay3DToken | urlquery}}">Ignore it for 3 more days</a></li>
			<li><a href="{{.Host}}/?a=delay_7day&t={{.Delay7DToken | urlquery}}">Ignore it for 7 more days</a></li>
		</ul>
	</p>

	<p>
		If you want the Reaper to always ignore your instance add a tag named REAPER_SPARE_ME to your instance.
	</p>
</body>
</html>
`

const T_PASSWORD = "boom"

var (
	t = template.Must(template.New("email").Parse(T_DATA))
)

func main() {

	id := "i-12345"
	name := "My fake instance"
	region := "us-west-2"

	// Token strings
	term, _ := token.Tokenize(T_PASSWORD, token.NewTerminateJob(region, id))

	delay1, _ := token.Tokenize(T_PASSWORD,
		token.NewDelayJob(region, id,
			time.Now().Add(time.Duration(24*time.Hour))))

	delay3, _ := token.Tokenize(T_PASSWORD,
		token.NewDelayJob(region, id,
			time.Now().Add(time.Duration(3*24*time.Hour))))

	delay7, _ := token.Tokenize(T_PASSWORD,
		token.NewDelayJob(region, id,
			time.Now().Add(time.Duration(7*24*time.Hour))))

	buf := bytes.NewBuffer(nil)
	err := t.Execute(buf, map[string]string{
		"Id":             id,
		"Name":           name,
		"Host":           "https://localhost",
		"TerminateDate":  time.Now().String(),
		"TerminateToken": term,
		"Delay1DToken":   delay1,
		"Delay3DToken":   delay3,
		"Delay7DToken":   delay7,
	})

	if err != nil {
		fmt.Println("ERROR", err)
	} else {
		fmt.Println(string(buf.Bytes()))
	}
}
