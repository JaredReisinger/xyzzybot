package main // import "github.com/JaredReisinger/fizmo-slack"

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/fizmo-slack/remglk"
)

func main() {
	logBase := log.StandardLogger()
	logBase.Level = log.DebugLevel
	logger := logBase.WithField("component", "main")

	logger.Info("Starting fizmo-slack...")

	i, err := remglk.NewInterp(logger)
	if err != nil {
		logger.WithError(err).Error("???")
		return
	}

	time.Sleep(time.Second * 1)

	i.SendCommand("look")

	// wait until signal....
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGPIPE, syscall.SIGKILL)
	s := <-c
	logger.WithField("signal", s).Warn("got signal")
	i.Kill()
}
