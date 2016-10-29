package server

import (
	"fmt"
	"github.com/golang/glog"
	"gopkg.in/njern/gonexmo.v1"
	"math/rand"
	"time"
)

// SMSGateway - type for providing sms functionality
type SMSGateway struct {
	client   *nexmo.Client
	messages chan *nexmo.SMSMessage
}

var smsGatewayInstance *SMSGateway

const codeBytes = "1234567890"

// NewSMSGateway - create new sms-gateway
func NewSMSGateway(apiKey, apiSecret string) *SMSGateway {
	var err error
	rand.Seed(time.Now().UnixNano())
	smsGw := &SMSGateway{}

	smsGw.messages = make(chan *nexmo.SMSMessage, 100)

	if apiKey != "" && apiSecret != "" {
		smsGw.client, err = nexmo.NewClientFromAPI(apiKey, apiSecret)
		if err != nil {
			glog.Error(err)
		}

		go smsGw.worker()
	}
	return smsGw
}

// GetSMSGateway - get instance of free sms-gateway
func GetSMSGateway() *SMSGateway {
	return smsGatewayInstance
}

// GetLatestMessage - return latest added sms-message
func (s *SMSGateway) GetLatestMessage() *nexmo.SMSMessage {
	len := len(s.messages)

	if len == 0 {
		return nil
	}

	var mess *nexmo.SMSMessage
	select {
	case mess = <-s.messages:
	case <-time.NewTimer(time.Millisecond * 100).C:
	}

	return mess
}

// SendSMSMessage - add sms-message to sending queue
func (s *SMSGateway) SendSMSMessage(phone, code string) error {
	message := &nexmo.SMSMessage{
		From:  "simplendi",
		To:    phone,
		Title: "",
		Type:  nexmo.Text,
		Text:  fmt.Sprintf("Your verification code is: %s", code),
	}

	//Todo: add fixed size for messages. Check possible overflow
	s.messages <- message
	return nil
}

// GenerateRandomCode - generate random numeric code
func (s *SMSGateway) GenerateRandomCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = codeBytes[rand.Intn(len(codeBytes))]
	}
	return string(b)
}

func (s *SMSGateway) worker() {
	glog.Info("Run SMS-sender process")
	for {
		mess := <-s.messages
		resp, err := s.client.SMS.Send(mess)
		for _, respMess := range resp.Messages {
			if respMess.Status != nexmo.ResponseSuccess {
				glog.Error(fmt.Errorf("Error with deliver text to %v: %v", respMess.To, respMess.ErrorText))
			}
		}

		if err != nil {
			// add fallback mechanism
			glog.Error(err)
		}
	}
}
