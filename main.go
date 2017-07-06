package main // import "github.com/JaredReisinger/fizmo-slack"

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/fizmo-slack/slack"
	"github.com/JaredReisinger/fizmo-slack/util"
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

	logger.Info("Starting fizmo-slack...")

	rtm, err := slack.StartRTM(config)
	if err != nil {
		logger.WithError(err).Error("starting slack RTM")
		return
	}

	runUntilSignal(logger)

	rtm.Disconnect()

	if false == false {
		return
	}

	c := slack.NewChannel(config, "@jaredreisinger", logger)
	c.StartGame("curses.z5")
	defer c.Kill()

	runUntilSignal(logger)
}

func runUntilSignal(logger log.FieldLogger) {
	// wait until signal....
	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGHUP, syscall.SIGINT, syscall.SIGPIPE, syscall.SIGKILL, syscall.SIGTERM)
	sig := <-q
	logger.WithField("signal", sig).Info("got signal")
	// os.Exit(1)
}
