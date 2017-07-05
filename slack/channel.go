package slack

import (
	"errors"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/fizmo-slack/interpreter"
)

// Channel represents a Slack channel (or user direct-message) to which we're
// connected.
type Channel struct {
	Name   string
	interp *interpreter.Interpreter
	logger log.FieldLogger
}

func NewChannel(name string, logger log.FieldLogger) *Channel {
	return &Channel{
		Name: name,
		logger: logger.WithFields(log.Fields{
			"component": "slack",
			"channel":   name,
		}),
	}
}

func (c *Channel) StartGame(name string) error {
	if c.interp != nil {
		err := errors.New("game already in progress, ignoring start-game request")
		c.logger.WithError(err).Error("starting game")
		return err
	}

	// Create a new interpreter for the requested game...
	i, err := interpreter.NewInterpreter(c.logger)
	if err != nil {
		c.logger.WithError(err).Error("starting interpreter")
		return err
	}

	go c.ListenForGameOutput(i.Output)

	err = i.Start()
	if err != nil {
		c.logger.WithError(err).Error("starting interpreter")
		return err
	}

	c.interp = i
	return nil
}

func (c *Channel) ListenForGameOutput(outchan chan *interpreter.GlkOutput) {
	c.logger.Info("setting up game output handler")
	for {
		output := <-outchan
		if output == nil {
			c.logger.Warn("game output has been closed")
			return
		}
		debugOutput := c.debugFormat(output)
		c.logger.WithField("output", debugOutput).Debug("recieved output")
	}
}

func (c *Channel) debugFormat(output *interpreter.GlkOutput) string {
	sep1 := "============================================================"
	sep2 := "------------------------------------------------------------"
	lines := []string{sep1}

	for _, w := range output.Windows {
		lines = append(lines, FormatWindow(w))
		lines = append(lines, sep2)
	}

	lines = append(lines, sep1)

	return strings.Join(lines, "\n")
}

func (c *Channel) Kill() {
	c.logger.Info("recieved kill request")
	// TODO: stop listening to random stuff?
	if c.interp != nil {
		c.interp.Kill()
	}
}
