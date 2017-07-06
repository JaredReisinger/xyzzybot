package slack

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/fizmo-slack/interpreter"
	"github.com/JaredReisinger/fizmo-slack/util"
)

// Channel represents a Slack channel (or user direct-message) to which we're
// connected.
type Channel struct {
	ID     string
	config *util.Config
	interp *interpreter.Interpreter
	logger log.FieldLogger
}

type channelMap map[string]*Channel

func NewChannel(config *util.Config, id string, logger log.FieldLogger) *Channel {
	return &Channel{
		ID:     id,
		config: config,
		logger: logger.WithFields(log.Fields{
			"component": "slack",
			"channel":   id,
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
	gameFile := path.Join(c.config.GameDirectory, name)
	c.logger.WithField("gameFile", gameFile).Info("starting game")
	i, err := interpreter.NewInterpreter(c.config, gameFile, c.logger)
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

		message := c.createMessage(output)
		b, err := json.Marshal(message)
		if err != nil {
			c.logger.WithError(err).Error("encoding JSON")
			return
		}
		c.logger.Debugf("FOR SLACK:\n%s", string(b))
	}
}

// Message is a Slack message
type Message struct {
	Username    string        `json:"username,omitempty"`
	Channel     string        `json:"channel,omitempty"`
	Text        string        `json:"text"`
	Attachments []*Attachment `json:"attachments"`
}

type Attachment struct {
	Fields []*Field `json:"fields"` // don't omit on empty... to force footer
	Footer string   `json:"footer"`
}

type Field struct {
}

func (c *Channel) createMessage(output *interpreter.GlkOutput) *Message {
	message := &Message{
		// Channel: c.Name,
		Username: "fizmobot",
	}

	lines := []string{}

	for _, w := range output.Windows {
		windowText := FormatWindow(w)

		// If the window looks like a status window, make its text into a footer
		// instead of part of the body.
		if inferStatusWindow(w) {
			message.Attachments = []*Attachment{
				&Attachment{
					Footer: windowText,
				},
			}
		} else {
			lines = append(lines, FormatWindow(w))
		}
	}

	text := strings.Join(lines, "\n")
	leader := ""

	// horribly gross special handling to make Slack pay attention to leading
	// whitespace:
	if len(text) > 0 {
		if text[0] == '\n' {
			leader = "."
		} else if text[0] == ' ' {
			leader = ".\n"
		}
	}

	message.Text = fmt.Sprintf("%s%s", leader, text)

	return message
}

func inferStatusWindow(w *interpreter.GlkWindow) bool {
	return w.Type == interpreter.TextGridWindowType &&
		w.Top == 0 &&
		w.Height <= 5
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
