package interpreter // import "github.com/JaredReisinger/fizmo-slack/interpreter"

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Interpreter represents (may be interface eventually) the command/output
// interaction.
type Interpreter struct {
	// magic   int
	logger  log.FieldLogger
	cmd     *exec.Cmd
	inPipe  io.WriteCloser
	outPipe io.ReadCloser
	errPipe io.ReadCloser

	inputWindow int
	inputGen    int

	// Output should be listened to (perhaps the interpreter should handle
	// listeners added dynamically, so that debugging can be added without
	// complicating things?)
	Output chan *GlkOutput

	// Input for the interpreter should be sent here
	Input chan *Input
}

// The remote-Glk output format separates the window size declarations from the
// output, which seems unnecessarily complicated (although perhaps a better
// reflection of the Z-Machine design).  We use a simplified model for
// interpreter interaction.

// Input represents input to the interpreter
type Input struct {
	WindowID int // really? can we use the zero-value for "default"?
	Type     InputType
	Text     string
}

// InputType ...
type InputType int

const (
	// NoInputType ...
	NoInputType InputType = iota
	// InitInput ...
	InitInput
	// TextInput ...
	TextInput
	// CharacterInput ...
	CharacterInput
)

// NewInterpreter ...
func NewInterpreter(logger log.FieldLogger) (interp *Interpreter, err error) {
	logger = logger.WithField("component", "interpreter")

	// attempt to start a subprocess for the game...
	// exe, err := exec.LookPath("fizmo-remglk")
	// if err != nil {
	// 	logger.WithError(err).Error("cannot find fizmo-remglk")
	// 	return
	// }

	cmd := exec.Command("fizmo-remglk", "-fixmetrics", "-width", "80", "-height", "50", "/Users/jreising/OneDrive/Documents/Interactive Fiction/MiscIFGames/curses.z5")

	inPipe, err := cmd.StdinPipe()
	if err != nil {
		logger.WithError(err).Error("getting stdin")
		return
	}

	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		logger.WithError(err).Error("getting stdout")
		return
	}

	errPipe, err := cmd.StderrPipe()
	if err != nil {
		logger.WithError(err).Error("getting stderr")
		return
	}

	interp = &Interpreter{
		logger:  logger,
		cmd:     cmd,
		inPipe:  inPipe,
		outPipe: outPipe,
		errPipe: errPipe,

		Input:  make(chan *Input, 5),
		Output: make(chan *GlkOutput, 5),
	}

	return
}

// Start starts up the interpreter
func (i *Interpreter) Start() error {
	// Kick off the out/err listeners?
	go i.ProcessRemGlkOutput()
	go i.ProcessInput()

	go i.debugInterpreterOutput()

	err := i.cmd.Start()
	if err != nil {
		i.logger.WithError(err).Error("starting child process")
		return err
	}

	i.logger = i.logger.WithFields(log.Fields{
		"pid": i.cmd.Process.Pid,
		// "game": "????",
		// "channel": "???",
	})

	i.logger.WithField("cmd", i.cmd).Info("running fizmo")
	return nil
}

// DebugInterpreterOutput ...
func (i *Interpreter) debugInterpreterOutput() {
	for {
		output := <-i.Output
		// i.logger.WithField("output", output).Debug("recieved output")
		i.simpleOutput(output)
	}
}

func (i *Interpreter) simpleOutput(output *GlkOutput) {
	sep1 := "============================================================"
	sep2 := "------------------------------------------------------------"
	lines := []string{sep1}

	for _, w := range output.Windows {
		lines = append(lines, w.Content.SlackString())
		lines = append(lines, sep2)
	}

	lines = append(lines, sep1)

	fmt.Println(strings.Join(lines, "\n"))
}
