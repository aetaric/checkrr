package notifications

import (
	"fmt"
	"github.com/aetaric/checkrr/logging"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"

	//"github.com/emersion/go-smtp"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	mail "github.com/xhit/go-simple-mail/v2"
)

type SMTPNotifs struct {
	host          string
	port          string
	authEnabled   bool
	user          string
	pass          string
	from          string
	to            string
	starttls      bool
	ssl           bool
	client        *mail.SMTPClient
	config        koanf.Koanf
	AllowedNotifs []string
	Log           *logging.Log
	Localizer     *i18n.Localizer
}

func (s *SMTPNotifs) FromConfig(config koanf.Koanf) {
	authconfig := config.Cut("auth")

	s.config = config
	s.host = config.String("host")
	s.port = config.String("port")
	s.from = config.String("from")
	s.to = config.String("to")
	s.user = authconfig.String("user")
	s.pass = authconfig.String("pass")
	s.authEnabled = authconfig.Bool("enabled")
	s.starttls = config.Bool("starttls")
	s.ssl = config.Bool("starttls")
	s.AllowedNotifs = config.Strings("notificationtypes")
}

func (s *SMTPNotifs) Connect() bool {
	if s.host != "" && s.port != "" {
		port, err := strconv.Atoi(s.port)
		if err != nil {
			message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsSMTPPort",
			})
			s.Log.WithFields(log.Fields{"Startup": true, "SMTP Connected": false}).Warn(message)
			return false
		}

		server := mail.NewSMTPClient()
		server.Host = s.host
		server.Port = port

		if s.user != "" && s.pass != "" {
			server.Username = s.user
			server.Password = s.pass
		}

		if s.starttls {
			server.Encryption = mail.EncryptionSTARTTLS
		} else if s.ssl {
			server.Encryption = mail.EncryptionSSL
		} else {
			server.Encryption = mail.EncryptionNone
		}

		server.KeepAlive = true
		server.ConnectTimeout = 10 * time.Second
		server.SendTimeout = 10 * time.Second

		smtpClient, err := server.Connect()
		if err != nil {
			message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsSMTPPort",
			})
			s.Log.WithFields(log.Fields{"Startup": true, "SMTP Connected": false}).Warn(message)
		}

		s.client = smtpClient
		message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsSMTPConnected",
		})
		s.Log.WithFields(log.Fields{"Startup": true, "SMTP Connected": true}).Info(message)
		return true
	} else {
		return false
	}
}

func (s SMTPNotifs) Notify(title string, description string, notifType string, path string) bool {
	var allowed bool = false
	for _, notif := range s.AllowedNotifs {
		if notif == notifType {
			allowed = true
		}
	}
	if allowed {
		email := mail.NewMSG()
		email.SetFrom(s.from).AddTo(s.to).SetSubject(fmt.Sprintf("Checkrr Notification: %s", title))
		if path != "" {
			email.SetBody(mail.TextPlain, fmt.Sprintf("%s: %s", description, path))
		} else {
			email.SetBody(mail.TextPlain, fmt.Sprintf("%s", description))
		}

		if email.Error != nil {
			s.Log.Error(email.Error)
		}

		err := email.Send(s.client)
		if err != nil {
			message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsSMTPErrorSend",
			})
			s.Log.WithFields(log.Fields{"Notifications": "SMTP"}).Warn(message)
		} else {
			message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsSMTPSent",
			})
			s.Log.WithFields(log.Fields{"Notifications": "SMTP"}).Warn(message)
		}

		return true
	}
	return false
}
