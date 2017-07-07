package slack

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/xyzzybot/interpreter"
	"github.com/JaredReisinger/xyzzybot/util"
)

// Channel represents a Slack channel (or user direct-message) to which we're
// connected.
type Channel struct {
	ID     string
	config *util.Config
	rtm    *RTM
	interp *interpreter.Interpreter
	logger log.FieldLogger
}

type channelMap map[string]*Channel

func NewChannel(config *util.Config, rtm *RTM, id string, logger log.FieldLogger) *Channel {
	return &Channel{
		ID:     id,
		config: config,
		rtm:    rtm,
		logger: logger.WithFields(log.Fields{
			"component": "slack",
			"channel":   id,
		}),
	}
}

func (c *Channel) StartGame(name string) error {
	if c.gameInProgress() {
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

	go c.listenForGameOutput(i.Output)

	err = i.Start()
	if err != nil {
		c.logger.WithError(err).Error("starting interpreter")
		return err
	}

	c.interp = i
	return nil
}

func (c *Channel) listenForGameOutput(outchan chan *interpreter.GlkOutput) {
	c.logger.Info("setting up game output handler")
	for {
		output := <-outchan
		if output == nil {
			c.logger.Warn("game output has been closed")
			c.killGame()
			return
		}
		debugOutput := c.debugFormat(output)
		c.logger.WithField("output", debugOutput).Debug("recieved output")

		c.sendOutputMessage(output)
	}
}

func (c *Channel) sendOutputMessage(output *interpreter.GlkOutput) {
	lines := []string{}
	status := ""

	for _, w := range output.Windows {
		windowText := formatWindow(w)

		// If the window looks like a status window, save its text separately as
		// status.
		if inferStatusWindow(w) {
			status = windowText
		} else {
			lines = append(lines, formatWindow(w))
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

	msg := fmt.Sprintf("%s%s", leader, text)
	c.sendMessageWithNameContext(msg, status, "game output")
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
		lines = append(lines, formatWindow(w))
		lines = append(lines, sep2)
	}

	lines = append(lines, sep1)

	return strings.Join(lines, "\n")
}

func (c *Channel) killGame() {
	c.logger.Info("recieved killGame request")

	if c.interp != nil {
		// TODO: stop listening to random stuff?
		c.interp.Kill()
		c.interp = nil
	}
}

func (c *Channel) sendIntro(initialStartup bool) {
	var format string

	if initialStartup {
		format = "Hi, everyone!  I’ve been asleep for a bit, but I’m awake again.  Just as a reminder, you can address me directly to get more help: `@%s help`"
	} else {
		format = "Hi, everyone!  Thanks for inviting me to the channel!  You can address me directly to get more help: `@%s help`"
	}

	msg := fmt.Sprintf(format, c.rtm.authInfo.User)
	c.sendMessage(msg)
}

func (c *Channel) sendMessage(text string) {
	c.rtm.sendMessage(c.ID, text)
}

// func (c *Channel) sendMessageWithStatus(text string, status string) {
// 	c.rtm.sendMessageWithStatus(c.ID, text, status)
// }

func (c *Channel) sendMessageWithNameContext(text string, status string, nameContext string) {
	c.rtm.sendMessageWithNameContext(c.ID, text, status, nameContext)
}

type commandHandler func(command string, args ...string)

func (c *Channel) handleCommand(command string) {
	// If we have an interpreter, it gets the command.  Otherwise (or if there's
	// a leading `/`), it's a meta-command.

	if c.gameInProgress() && !strings.HasPrefix(command, "/") {
		c.interp.SendCommand(command)
		return
	}

	command = strings.TrimPrefix(command, "/")

	// right now, we only do super-simple command parsing...
	words := strings.Fields(command)
	dispatch := map[string]commandHandler{
		"help":   c.commandHelp,
		"status": c.commandStatus,
		"list":   c.commandList,
		"play":   c.commandPlay,
		"kill":   c.commandKill,
	}

	handler, ok := dispatch[words[0]]
	if !ok {
		handler = c.commandUnknown
	}

	handler(words[0], words[1:]...)
}

func (c *Channel) commandHelp(command string, args ...string) {
	msg := fmt.Sprintf("Hi!  I’m %[1]s, and I exist to help you experience the world of interactive fiction.\n\nWhen there’s a game in progress, I’ll assume that any comments directed my way are actually meant for the game, and I’ll pass them along.  If you really want to reach me directly, slap a `/` at the begining, like `@%[1]s /help` to see this message again.\n\nWhen there’s _not_ a game underway, or if your `/`-prefix your message, I can help with the following:\n* `help` - this message\n* `status` - operational status about myself\n* `list` - list the available games\n* `play game-name` - start _game-name_\n* `kill` - kill an in-progress game", c.rtm.authInfo.User)
	c.sendMessage(msg)
}

func (c *Channel) commandStatus(command string, args ...string) {
	var inProgress string
	if c.gameInProgress() {
		inProgress = "There *is* currently a game in progress in this channel."
	} else {
		inProgress = "There *is not* currently a game in progress in this channel."
	}

	// TODO: better feedback here!
	channelIds := make([]string, 0, len(c.rtm.channels))
	for k := range c.rtm.channels {
		channelIds = append(channelIds, k)
	}

	msg := fmt.Sprintf("I am participating in the following channels: %s\n%s", strings.Join(channelIds, ", "), inProgress)
	c.sendMessage(msg)
}

func (c *Channel) commandList(command string, args ...string) {
	dir, err := os.Open(c.config.GameDirectory)
	if err != nil {
		c.logger.WithField("path", c.config.GameDirectory).WithError(err).Error("unable to open game directory")
		c.sendMessage("I’m sorry, I wasn’t able to get to the list of games.  Please let XXX know something needs to be tweaked!")
		return
	}

	infos, err := dir.Readdir(-1)
	if err != nil {
		c.logger.WithField("path", c.config.GameDirectory).WithError(err).Error("unable to open game directory")
		c.sendMessage("I’m sorry, I wasn’t able to get the list of games.  Please let XXX know something needs to be tweaked!")
		return
	}

	files := make([]string, 0, len(infos))

	for _, info := range infos {
		if info.Mode().IsRegular() {
			files = append(files, info.Name())
		}
	}

	warning := ""

	if c.gameInProgress() {
		warning = "\n\n_Do note that there's currently a game in progress; you’ll need to finish or `/kill` it before you can start a new game._"
	}

	msg := fmt.Sprintf("The following games are currently available:\n* `%s`\n\nYou can start a game using `@%s play game-name`%s", strings.Join(files, "`\n* `"), c.rtm.authInfo.User, warning)
	c.sendMessage(msg)
}

func (c *Channel) commandPlay(command string, args ...string) {
	if c.gameInProgress() {
		c.sendMessage("_There's currently a game in progress; you’ll need to finish or `/kill` it before you can start a new game._")
		return
	}

	c.StartGame(args[0])
	// gameFile := path.Join(c.config.GameDirectory, args[0])
	//
	// i, err := interpreter.NewInterpreter(c.config, gameFile, c.logger)
	// if err != nil {
	// 	c.logger.WithField("path", gameFile).WithError(err).Error("unable to start interpreter")
	// 	c.sendMessage(fmt.Sprintf("I’m sorry, I wasn’t able to start the game `%s`.", args[0]))
	// 	return
	// }
	// c.interp = i
}

func (c *Channel) commandKill(command string, args ...string) {
	if !c.gameInProgress() {
		c.sendMessage("There's _not_ currently a game in progress!")
		return
	}

	c.killGame()
}

func (c *Channel) commandUnknown(command string, args ...string) {
	c.logger.WithField("command", command).Debug("unknown command")
	c.sendMessage(fmt.Sprintf("I’m sorry, I don’t know how to `%s`.", command))
}

func (c *Channel) gameInProgress() bool {
	// Should we also check to see that the underlying process is really
	// working?  (This could/should be exposed as a helper on Interpreter
	// itself.)
	return c.interp != nil
}
