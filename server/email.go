package server

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/gomail.v2"
	"time"
)

var emailInstance *EmailSender

// EmailSender - type for sending emails
type EmailSender struct {
	queue  chan *gomail.Message
	config *Config
}

// GetEmailSenderInstance - return free instance of email sender
func GetEmailSenderInstance() *EmailSender {
	return emailInstance
}

// NewEmailSender - return new instance of email sender
func NewEmailSender(config *Config) *EmailSender {
	queue := make(chan *gomail.Message, 1000)
	es := &EmailSender{
		queue:  queue,
		config: config,
	}

	if config.EmailUsername != "" && config.EmailPassword != "" {
		go es.sender()
	}

	return es
}

// GetLatestMessage - return latest message of email queue
func (e *EmailSender) GetLatestMessage() *gomail.Message {
	if len(e.queue) == 0 {
		return nil
	}

	var mess *gomail.Message
	select {
	case mess = <-e.queue:
	case <-time.NewTimer(time.Millisecond * 100).C:
	}
	return mess
}

// SendEmailConfirmation - add email to sending queue
func (e *EmailSender) SendEmailConfirmation(name, email, code string) {
	m := gomail.NewMessage()
	m.SetHeader("From", e.config.EmailFrom)
	m.SetHeader("To", email)
	m.SetHeader("Subject", fmt.Sprintf("Email confirmation for %v", name))
	m.SetBody("text/html", fmt.Sprintf("%s/confirm-email/%v", e.config.ServerURL, code))

	e.queue <- m
}

// sender - routine for sending emails to smtp-server
func (e *EmailSender) sender() {
	d := gomail.NewDialer(e.config.EmailSMTP, e.config.EmailSMTPPort, e.config.EmailUsername, e.config.EmailPassword)
	for {
		m := <-e.queue
		if err := d.DialAndSend(m); err != nil {
			log.Error(err)
		}
	}
}
