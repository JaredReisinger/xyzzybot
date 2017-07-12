package util

import (
	"encoding/json"
	"flag"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

// Config defines the available set of configuration options available for the
// service.
type Config struct {
	GameDirectory string

	Slack struct {
		// ClientID             string
		// ClientSecret         string
		// VerificationToken    string
		// OAuthAccessToken     string
		BotUserOAuthAccessToken string
		BotTokenFile            string
		Admins                  []string
	}

	Logger log.FieldLogger `json:"-"`
}

// ParseConfigFile attempts to load a Config struct, using the data in a JSON
// file.
func ParseConfigFile(configFile string, logger log.FieldLogger) (config *Config, err error) {
	config = &Config{
		// GameDirectory: "(NONE)",
		Logger: logger,
	}

	if len(configFile) == 0 {
		// err = errors.New("configFile must be non-zero string, perhaps -config is missing?")
		return
	}

	logger.WithFields(log.Fields{
		"component": "config",
		"file":      configFile,
	}).Info("loading config")

	byts, err := ioutil.ReadFile(configFile)
	if err != nil {
		return
	}

	err = json.Unmarshal(byts, &config)
	return
}

// AddConfigFlag adds the standard "-config" command-line flag for users of
// this package.
func AddConfigFlag() *string {
	return flag.String("config", "", "The path to the JSON config file to load")
}
