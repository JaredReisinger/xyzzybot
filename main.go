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

	"github.com/JaredReisinger/xyzzybot/games"
	"github.com/JaredReisinger/xyzzybot/glk"
	"github.com/JaredReisinger/xyzzybot/slack"
)

func main() {
	logBase := log.StandardLogger()
	logBase.Level = log.DebugLevel
	logger := logBase.WithField("component", "main")

	configParam := AddConfigFlag()

	gameDirParam := flag.String("game-dir", "",
		"directory to use for games")

	tokenParam := flag.String("token", "",
		"Slack bot token to use for authentication")

	tokenFileParam := flag.String("token-file", "",
		"file to read for Slack bot token")

	flag.Parse()

	config, err := ParseConfigFile(*configParam, logBase)
	if err != nil {
		logger.WithField("file", *configParam).WithError(err).Fatal("error parsing config file")
	}

	// override any config values with command-line ones...

	if *gameDirParam != "" {
		if config.GameDirectory != "" {
			logger.WithFields(log.Fields{
				"configVal": config.GameDirectory,
				"paramVal":  *gameDirParam,
			}).Debug("overriding game directory from config")
		}
		config.GameDirectory = *gameDirParam
	}

	if *tokenFileParam != "" {
		if config.BotTokenFile != "" || config.BotToken != "" {
			logger.WithFields(log.Fields{
				"configVal":   config.BotTokenFile,
				"configToken": config.BotToken,
				"paramVal":    *tokenFileParam,
			}).Debug("token file overriding token info from config")
		}
		config.BotTokenFile = *tokenFileParam
		// a command-line token file overrides a token from the config file
		config.BotToken = ""
	}

	if *tokenParam != "" {
		if config.BotToken != "" {
			logger.WithFields(log.Fields{
				"configVal": config.BotToken,
				"paramVal":  *tokenParam,
			}).Debug("token overriding token from config")
		}
		config.BotToken = *tokenParam
	}

	if config.BotTokenFile != "" && config.BotToken != "" {
		logger.WithFields(log.Fields{
			"tokenFile": config.BotTokenFile,
			"token":     config.BotToken,
		}).Warn("both token and token file available... using the token!")
		config.BotTokenFile = ""
	}

	if config.BotTokenFile != "" {
		config.BotToken, err = readTokenFile(config.BotTokenFile, logger)
		if err != nil {
			logger.WithError(err).Fatal("unable to read token from file")
		}
	}

	logger.WithField("config", config).Debug("using config")

	// Create components...
	terpFactory := &glk.RemGlkFactory{
		Logger: logBase,
	}

	gamesFS := &games.FileSys{
		Directory: config.GameDirectory,
		Logger:    logBase,
	}

	botConfig := &slack.Config{
		BotToken:           config.BotToken,
		Admins:             config.Admins,
		Logger:             logBase,
		Games:              gamesFS,
		InterpreterFactory: terpFactory,
	}

	logger.Info("Starting xyzzybot...")
	defer logger.Info("xyzzybot exited")

	manager, err := slack.StartManager(botConfig)
	if err != nil {
		logger.WithError(err).Error("starting slack manager")
		return
	}
	defer manager.Disconnect()

	runUntilSignal(logger)

	logger.Info("xyzzybot exiting")
}

func readTokenFile(tokenFile string, logger log.FieldLogger) (token string, err error) {
	logger2 := logger.WithField("file", tokenFile)
	f, err := os.Open(tokenFile)
	if err != nil {
		logger2.WithError(err).Error("unable to open token file")
		return
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for token == "" {
		token, err = r.ReadString('\n')
		if err != nil {
			logger2.WithError(err).Error("reading token file")
			return
		}

		token = strings.TrimSpace(token)

		// treat #-prefixed lines as blank...
		if strings.HasPrefix(token, "#") {
			token = ""
		}
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
