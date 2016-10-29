package main

import (
	"flag"
	"fmt"
	"git.simplendi.com/FirmQ/frontend-server/server"
	"github.com/golang/glog"
	"github.com/spf13/viper"
)

func main() {
	flag.Parse()

	viper.SetEnvPrefix("simplendi")
	viper.AutomaticEnv()

	config := &server.Config{
		NexmoAPIKey:          viper.GetString("nexmo_api_key"),
		NexmoSecretKey:       viper.GetString("nexmo_secret_key"),
		Port:                 viper.GetString("port"),
		EmailUsername:        viper.GetString("email_username"),
		EmailPassword:        viper.GetString("email_password"),
		EmailFrom:            viper.GetString("email_from"),
		ServerURL:            viper.GetString("server_url"),
		EmailSMTP:            viper.GetString("email_smtp"),
		EmailSMTPPort:        viper.GetInt("email_smtp_port"),
		EmailConfirmationTTL: viper.GetDuration("email_confirmation_ttl"),
		SMSConfirmationTTL:   viper.GetDuration("sms_confirmation_ttl"),
	}

	fmt.Printf("%+v\n", config)

	srv, err := server.NewServer(config)
	if err != nil {
		glog.Error(err)
	}

	flag.Set("alsologtostderr", "true")
	flag.Set("v", "5")

	if err := srv.RunServer(); err != nil {
		glog.Error(err)
	}
}
