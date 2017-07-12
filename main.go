package main // import "github.com/JaredReisinger/xyzzybot"

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/xyzzybot/games"
	"github.com/JaredReisinger/xyzzybot/glk"
	"github.com/JaredReisinger/xyzzybot/slack"
	"github.com/JaredReisinger/xyzzybot/util"
)

func main() {
	logBase := log.StandardLogger()
	logBase.Level = log.DebugLevel
	logger := logBase.WithField("component", "main")

	configParam := util.AddConfigFlag()
	flag.Parse()

	config, err := util.ParseConfigFile(*configParam, logBase)
	if err != nil {
		logger.Fatalf("error parsing config file: %#v\n", err)
	}

	// Create components...
	terpFactory := &glk.RemGlkFactory{
		Logger: logBase,
	}

	gamesFS := &games.FileSys{
		Directory: config.GameDirectory,
		Logger:    logBase,
	}

	botConfig := &slack.Config{
		BotToken:           config.Slack.BotUserOAuthAccessToken,
		Admins:             config.Slack.Admins,
		Logger:             logBase,
		Games:              gamesFS,
		InterpreterFactory: terpFactory,
	}

	logger.Info("Starting xyzzybot...")

	rtm, err := slack.StartRTM(botConfig)
	if err != nil {
		logger.WithError(err).Error("starting slack RTM")
		return
	}

	runUntilSignal(logger)

	rtm.Disconnect()
}

func runUntilSignal(logger log.FieldLogger) {
	// wait until signal....
	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGHUP, syscall.SIGINT /*syscall.SIGPIPE,*/, syscall.SIGKILL, syscall.SIGTERM)
	sig := <-q
	logger.WithField("signal", sig).Info("got signal")
	// os.Exit(1)
}
