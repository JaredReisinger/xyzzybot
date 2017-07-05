package main // import "github.com/JaredReisinger/fizmo-slack"

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/fizmo-slack/slack"
)

func main() {
	logBase := log.StandardLogger()
	logBase.Level = log.DebugLevel
	logger := logBase.WithField("component", "main")

	logger.Info("Starting fizmo-slack...")

	c := slack.NewChannel("@jaredreisinger", logger)
	c.StartGame("XXXXX")
	defer c.Kill()

	// wait until signal....
	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGHUP, syscall.SIGINT, syscall.SIGPIPE, syscall.SIGKILL, syscall.SIGTERM)
	sig := <-q
	logger.WithField("signal", sig).Info("got signal")
	// os.Exit(1)
}
