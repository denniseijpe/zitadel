package messages

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/zitadel/zitadel/internal/eventstore"
	"github.com/zitadel/zitadel/internal/notification/channels"
)

var (
	isHTMLRgx = regexp.MustCompile(`.*<html.*>.*`)
	lineBreak = "\r\n"
)

var _ channels.Message = (*Email)(nil)

type Email struct {
	Recipients      []string
	BCC             []string
	CC              []string
	SenderEmail     string
	SenderName      string
	Subject         string
	Content         string
	TriggeringEvent eventstore.Event
}

func (msg *Email) GetContent() (string, error) {
	headers := make(map[string]string)
	from := msg.SenderEmail
	if msg.SenderName != "" {
		from = fmt.Sprintf("%s <%s>", msg.SenderName, msg.SenderEmail)
	}
	headers["From"] = from
	headers["Return-Path"] = msg.SenderEmail
	headers["To"] = strings.Join(msg.Recipients, ", ")
	headers["Cc"] = strings.Join(msg.CC, ", ")
	headers["Date"] = time.Now().Format(time.RFC1123Z)

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s"+lineBreak, k, v)
	}

	//default mime-type is html
	mime := "MIME-version: 1.0;" + lineBreak + "Content-Type: text/html; charset=\"UTF-8\";" + lineBreak + lineBreak
	if !isHTML(msg.Content) {
		mime = "MIME-version: 1.0;" + lineBreak + "Content-Type: text/plain; charset=\"UTF-8\";" + lineBreak + lineBreak
	}
	subject := "Subject: " + msg.Subject + lineBreak
	message += subject + mime + lineBreak + msg.Content

	return message, nil
}

func (msg *Email) GetTriggeringEvent() eventstore.Event {
	return msg.TriggeringEvent
}

func isHTML(input string) bool {
	return isHTMLRgx.MatchString(input)
}
