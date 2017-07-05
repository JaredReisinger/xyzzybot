package main // import "github.com/JaredReisinger/fizmo-slack"

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/fizmo-slack/interpreter"
	"github.com/JaredReisinger/fizmo-slack/slack"
)

func main() {
	logBase := log.StandardLogger()
	logBase.Level = log.DebugLevel
	logger := logBase.WithField("component", "main")

	logger.Info("Starting fizmo-slack...")

	s := slack.NewSender("@jaredreisinger", logger)

	i, err := interpreter.NewInterpreter(logger)
	if err != nil {
		logger.WithError(err).Error("creating interpreter")
		return
	}

	go s.Listen(i.Output)

	err = i.Start()
	if err != nil {
		logger.WithError(err).Error("starting interpreter")
		return
	}

	// // send a "look" command in a bit...
	// go func() {
	// 	time.Sleep(time.Second * 1)
	// 	i.SendCommand("look")
	// }()

	// wait until signal....
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGPIPE, syscall.SIGKILL)
	sig := <-c
	logger.WithField("signal", sig).Warn("got signal")
	i.Kill()
}
