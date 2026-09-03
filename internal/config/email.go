// pkg/config/email.go
package config

type EmailConfig struct {
	Provider       string
	FromAddress    string
	FromName       string
	SMTPHost       string
	SMTPPort       int
	SMTPUsername   string
	SMTPPassword   string
	SendGridAPIKey string
	AWSRegion      string
}

type SMSConfig struct {
	Provider         string
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string
	AWSRegion        string
}

func loadEmailConfig() EmailConfig {
	return EmailConfig{
		Provider:       getEnv("EMAIL_PROVIDER", "smtp"),
		FromAddress:    getEnv("EMAIL_FROM_ADDRESS", "noreply@manifesto.com"),
		FromName:       getEnv("EMAIL_FROM_NAME", "Manifesto"),
		SMTPHost:       getEnv("SMTP_HOST", ""),
		SMTPPort:       getEnvInt("SMTP_PORT", 587),
		SMTPUsername:   getEnv("SMTP_USERNAME", ""),
		SMTPPassword:   getEnv("SMTP_PASSWORD", ""),
		SendGridAPIKey: getEnv("SENDGRID_API_KEY", ""),
		AWSRegion:      getEnv("AWS_REGION", "us-east-1"),
	}
}

func loadSMSConfig() SMSConfig {
	return SMSConfig{
		Provider:         getEnv("SMS_PROVIDER", "twilio"),
		TwilioAccountSID: getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:  getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromNumber: getEnv("TWILIO_FROM_NUMBER", ""),
		AWSRegion:        getEnv("AWS_REGION", "us-east-1"),
	}
}
