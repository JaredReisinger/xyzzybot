package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"path"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// Config defines the available set of configuration options available for the
// service.
type Config struct {
	GameDirectory string
	WorkingRoot   string
	BotToken      string
	BotTokenFile  string
	Admins        []string
}

// ParseConfigFile attempts to load a Config struct, using the data in a JSON
// file.
func ParseConfigFile(configFile string, logger log.FieldLogger) (config *Config, err error) {
	// always return an empty config, rather than nil!
	config = &Config{}

	if len(configFile) == 0 {
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

	// Make sure any file paths are resolved relative to the config file.
	// These need to be absolute paths if possible, because we will sometimes
	// run executables (the game interpreter) from different directories
	absConfig, err := filepath.Abs(configFile)
	if err != nil {
		logger.WithError(err).Error("getting config file absolute path")
		return
	}
	configDir := filepath.Dir(absConfig)
	absolutize(configDir, &config.GameDirectory)
	absolutize(configDir, &config.WorkingRoot)
	absolutize(configDir, &config.BotTokenFile)

	return
}

func absolutize(baseDir string, relative *string) {
	if relative != nil && *relative != "" && !path.IsAbs(*relative) {
		*relative = path.Join(baseDir, *relative)
	}
}

// AddConfigFlag adds the standard "-config" command-line flag for users of
// this package.
func AddConfigFlag() *string {
	return flag.String("config", "", "The path to the JSON config file to load")
}
