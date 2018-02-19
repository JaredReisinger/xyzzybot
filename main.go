package main // import "github.com/JaredReisinger/xyzzybot"

import (
	"bufio"
	"errors"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/xyzzybot/console"
	"github.com/JaredReisinger/xyzzybot/fizmo"
	"github.com/JaredReisinger/xyzzybot/games"
	"github.com/JaredReisinger/xyzzybot/slack"
)

const (
	defaultGameDirectory = "/usr/local/games"
	defaultWorkingRoot   = "/usr/local/var/xyzzybot"
	defaultConfigFile    = "/usr/local/etc/xyzzybot/config.json"
)

func main() {
	logBase := log.StandardLogger()
	logBase.Level = log.DebugLevel
	logger := logBase.WithField("component", "main")

	configParam := AddConfigFlag()

	gameDirParam := flag.String("game-dir", "",
		"directory to use for games")

	workingRootParam := flag.String("working-root", "",
		"directory root for dynamic configuration and game saves/state")

	tokenParam := flag.String("token", "",
		"Slack bot token to use for authentication")

	tokenFileParam := flag.String("token-file", "",
		"file to read for Slack bot token")

	consoleParam := flag.Bool("console", false,
		"use console instead of Slack (for debugging/testing)")

	flag.Parse()

	// if there's no -config flag, we look in a default location
	configFile := *configParam
	requireConfig := true

	if fallback("config file", &configFile, defaultConfigFile, logger) {
		requireConfig = false
	}

	config, err := ParseConfigFile(configFile, logBase)
	if err != nil {
		if requireConfig {
			logger.WithField("file", configFile).WithError(err).Fatal("error parsing config file")
		} else {
			logger.WithField("file", configFile).WithError(err).Debug("error parsing default config file")
		}
	}

	// override any config values with command-line ones...

	overrideParam("game directory", &config.GameDirectory, gameDirParam, logger)

	overrideParam("working root", &config.WorkingRoot, workingRootParam, logger)

	if overrideParam("token file", &config.BotTokenFile, tokenFileParam, logger) {
		// a command-line token file also overrides a token from config
		config.BotToken = ""
	}

	overrideParam("token", &config.BotToken, tokenParam, logger)

	// And one final check... if we have both a token and token file, just
	// use the token.  In this case we also clear out the token file config
	// setting so that they can't disagree.
	if config.BotTokenFile != "" && config.BotToken != "" {
		logger.WithFields(log.Fields{
			"tokenFile": config.BotTokenFile,
			"token":     config.BotToken,
		}).Warn("both token and token file available... using the token!")
		config.BotTokenFile = ""
	}

	// If we *still* have a token file, it means we *don't* have a token... so
	// we need to read the file.  In this case, we *keep* the token file value
	// as well, which helps make it clear that the token *came from* the file.
	if config.BotTokenFile != "" {
		config.BotToken, err = readTokenFile(config.BotTokenFile, logger)
		if err != nil {
			logger.WithError(err).Fatal("unable to read token from file")
		}
	}

	// Fill in any defaults if they're still not set...
	fallback("game directory", &config.GameDirectory, defaultGameDirectory, logger)
	fallback("working root", &config.WorkingRoot, defaultWorkingRoot, logger)

	// If we don't have a bot token, that's a fatal error...
	if config.BotToken == "" {
		logger.Fatal("no bot token found")
	}

	logger.WithField("config", config).Debug("using config")

	// Create components...
	terpFactory := &fizmo.ExternalProcessFactory{
		Logger: logBase,
	}

	gamesFS := &games.FileSys{
		Directory: config.GameDirectory,
		Logger:    logBase,
	}

	logger.Info("Starting xyzzybot...")
	defer logger.Info("xyzzybot exited")

	// wow... slack/console looks interface-able!
	if *consoleParam {
		consoleConfig := &console.Config{
			Logger:             logBase,
			Games:              gamesFS,
			WorkingRoot:        config.WorkingRoot,
			InterpreterFactory: terpFactory,
		}

		c, err := console.StartConsole(consoleConfig)
		if err != nil {
			logger.WithError(err).Error("starting console")
			return
		}
		defer c.Disconnect()
	} else {
		botConfig := &slack.Config{
			BotToken:           config.BotToken,
			Admins:             config.Admins,
			Logger:             logBase,
			Games:              gamesFS,
			WorkingRoot:        config.WorkingRoot,
			InterpreterFactory: terpFactory,
		}

		manager, err := slack.StartManager(botConfig)
		if err != nil {
			logger.WithError(err).Error("starting slack manager")
			return
		}
		defer manager.Disconnect()
	}

	runUntilSignal(logger)

	logger.Info("xyzzybot exiting")
}

func overrideParam(name string, configVal *string, paramVal *string, logger log.FieldLogger) bool {
	if *paramVal == "" {
		return false
	}

	if *configVal != "" {
		logger.WithFields(log.Fields{
			"configVal": *configVal,
			"paramVal":  *paramVal,
		}).Debugf("overriding %s from config", name)
	}

	*configVal = *paramVal
	return true
}

func fallback(name string, val *string, defaultVal string, logger log.FieldLogger) bool {
	if *val != "" {
		return false
	}

	logger.WithField("value", defaultVal).Debugf("falling back to default %s", name)
	*val = defaultVal
	return true
}

func readTokenFile(tokenFile string, logger log.FieldLogger) (token string, err error) {
	logger2 := logger.WithField("file", tokenFile)
	f, err := os.Open(tokenFile)
	if err != nil {
		logger2.WithError(err).Error("unable to open token file")
		return
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			token = line
			break
		}
	}
	if err = s.Err(); err != nil {
		logger2.WithError(err).Error("reading token file")
		return
	}

	if token == "" {
		err = errors.New("No token found in token file")
		logger2.WithError(err).Error("getting token")
	}

	return
}

func runUntilSignal(logger log.FieldLogger) {
	// wait until signal....
	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGHUP, syscall.SIGINT /*syscall.SIGPIPE,*/, syscall.SIGKILL, syscall.SIGTERM)
	sig := <-q
	logger.WithField("signal", sig).Info("got signal")
	// os.Exit(1)
}
