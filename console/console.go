package console

import (
	"bufio"
	"io"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/xyzzybot/fizmo"
	"github.com/JaredReisinger/xyzzybot/games"
)

// Config ...
type Config struct {
	Logger             log.FieldLogger
	Games              games.Repository
	InterpreterFactory fizmo.InterpreterFactory
	WorkingRoot        string
}

// Console ...
type Console struct {
	config *Config
	logger log.FieldLogger
	quit   chan bool
	interp fizmo.Interpreter
}

// StartConsole ...
func StartConsole(config *Config) (*Console, error) {
	c := &Console{
		config: config,
		logger: config.Logger.WithField("component", "console"),
		quit:   make(chan bool),
	}

	go c.processInput(os.Stdin)

	return c, nil
}

// Disconnect ...
func (c *Console) Disconnect() {
	close(c.quit)
}

func (c *Console) processInput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		input := scanner.Text()
		c.handleInput(input)
	}
}

func (c *Console) handleInput(input string) {
	inGame := c.inGame()
	meta := strings.HasPrefix(input, "!")
	input = strings.TrimPrefix(input, "!")

	if !meta && inGame {
		// send to game!
		c.interp.Send(input)
		return
	}

	words := strings.Fields(input)

	command := ""
	if len(words) > 0 {
		command = words[0]
	}

	switch command {
	// case "space":
	// 	if inGame {
	// 		c.interp.SendChar(' ')
	// 	}
	case "list":
		c.commandList()
	case "play":
		c.commandPlay(words[1])
	default:
		c.logger.WithField("command", command).Error("unknown command")
	}
}

func (c *Console) inGame() bool {
	return c.interp != nil
}

func (c *Console) commandPlay(game string) {
	if c.inGame() {
		c.logger.Warn("already in a game!")
		return
	}

	// Create a working directory for the interpreter...
	workingDir := path.Join(c.config.WorkingRoot, "console")
	err := os.MkdirAll(workingDir, os.FileMode(0755))
	if err != nil {
		c.logger.WithError(err).Error("creating working directory")
		return
	}

	c.logger.WithField("game", game).Info("starting game")
	gameFile, err := c.config.Games.GetGameFile(game)
	if err != nil {
		c.logger.WithError(err).Error("getting game file")
		return
	}

	i, err := c.config.InterpreterFactory.NewInterpreter(gameFile, workingDir, log.Fields{"game": game})
	if err != nil {
		c.logger.WithError(err).Error("creating interpreter")
		return
	}

	go c.processOutput(i.GetOutputChannel())

	err = i.Start()
	if err != nil {
		c.logger.WithError(err).Error("starting interpreter")
		return
	}

	c.interp = i
}

func (c *Console) commandList() {
	games, err := c.config.Games.GetGames()
	if err != nil {
		c.logger.WithError(err).Error("getting games")
		return
	}

	c.logger.WithField("games", games).Info("games")
}

func (c *Console) processOutput(outchan chan *fizmo.Output) {
	c.logger.Info("setting up game output handler")
	for {
		output := <-outchan
		if output == nil {
			c.logger.Warn("game output has been closed")
			c.killGame()
			return
		}
		debugOutput := formatDebugOutput(output)
		c.logger.WithField("output", debugOutput).Debug("recieved output")
	}
}

func (c *Console) killGame() {
	c.logger.Info("recieved killGame request")

	if c.interp != nil {
		// TODO: stop listening to random stuff?
		c.interp.Kill()
		c.interp = nil
	}
}
