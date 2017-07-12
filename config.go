package main

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
	BotToken      string
	BotTokenFile  string
	Admins        []string
}

// ParseConfigFile attempts to load a Config struct, using the data in a JSON
// file.
func ParseConfigFile(configFile string, logger log.FieldLogger) (config *Config, err error) {

	if len(configFile) == 0 {
		// return an empty config, rather than nil!
		config = &Config{}
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
	if err != nil {
		return
	}

	return
}

// AddConfigFlag adds the standard "-config" command-line flag for users of
// this package.
func AddConfigFlag() *string {
	return flag.String("config", "", "The path to the JSON config file to load")
}
