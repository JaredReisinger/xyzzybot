package glk

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"sort"

	log "github.com/sirupsen/logrus"
)

// RemGlkFactory ...
type RemGlkFactory struct {
	// GameDirectory string
	Logger log.FieldLogger
}

// NewInterpreter ...
func (f *RemGlkFactory) NewInterpreter(gameFile string, fields log.Fields) (i Interpreter, err error) {
	logger := f.Logger.WithField("component", "remglk").WithFields(fields)

	cmd := exec.Command("fizmo-remglk", "-fixmetrics", "-width", "80", "-height", "50", gameFile)

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

	i = &RemGlk{
		logger:  logger,
		cmd:     cmd,
		inPipe:  inPipe,
		outPipe: outPipe,
		errPipe: errPipe,

		Output: make(chan *Output, 5),
	}

	return
}

// RemGlk represents (may be interface eventually) the command/output
// interaction.
type RemGlk struct {
	// magic   int
	logger  log.FieldLogger
	cmd     *exec.Cmd
	inPipe  io.WriteCloser
	outPipe io.ReadCloser
	errPipe io.ReadCloser
	killing bool

	inputWindow int
	inputGen    int

	// Output should be listened to (perhaps the interpreter should handle
	// listeners added dynamically, so that debugging can be added without
	// complicating things?)
	Output chan *Output
}

// The remote-Glk output format separates the window size declarations from the
// output, which seems unnecessarily complicated (although perhaps a better
// reflection of the Z-Machine design).  We use a simplified model for
// interpreter interaction.

// GetOutputChannel ...
func (i *RemGlk) GetOutputChannel() chan *Output {
	return i.Output
}

// Start starts up the interpreter
func (i *RemGlk) Start() error {
	// Kick off the out/err listeners?
	go i.ProcessRemOutput()
	// go i.ProcessInput()

	// go i.debugOutput()

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

	i.logger.WithField("cmd", i.cmd).Info("running interpreter")
	return nil
}

// // debugOutput ...
// func (i *RemGlk) debugOutput() {
// 	for {
// 		output := <-i.Output
// 		i.logger.WithField("output", output).Debug("recieved output")
// 	}
// }

// ProcessRemOutput ...
func (i *RemGlk) ProcessRemOutput() {
	var windows []*Window
	decoder := json.NewDecoder(i.outPipe)
	for {
		output := &Output{}
		err := decoder.Decode(&output)
		if i.killing {
			i.logger.Info("killing the interpreter")
			close(i.Output)
			return
		}
		if err == io.EOF {
			i.logger.Info("read EOF")
			close(i.Output)
			return
		} else if err != nil {
			i.logger.WithError(err).Error("decoding JSON")
			// skip/eat the error? perhaps we need a way to pass errors along to
			// any listeners?
			continue
		}

		// The Glk specification says that values (like window specifications)
		// only have to be sent when things are changed.  I haven't seen this
		// yet, but we need to ensure that *if* any information is missing, we
		// re-create it so that listeners don't need to worry about it.

		// If/when remglk sends window information, it sends it for *all* of the
		// windows, so we don't need to deal with deltas here, but we do need to
		// deal with entirely-missing info.
		if output.Windows != nil {
			sort.Sort(byPosition(output.Windows))
			windows = output.Windows
		} else {
			i.logger.Debug("using cached windows")
			output.Windows = windows
		}

		// Map all window content to the appropriate window...
		for _, c := range output.Content {
			mapped := false
			for _, w := range output.Windows {
				if w.ID == c.ID {
					w.Content = c
					mapped = true
					break
				}
			}
			if !mapped {
				i.logger.WithField("content", c).Warn("could not map content to window")
			}
		}

		// Send the output (TODO: to multiple listeners?)
		i.Output <- output

		// i.logger.WithField("output", output).Debug("read JSON")

		// Assume there's only one input? (and always one?)  (This should live
		// in the interpreter proper, rather than the remglk layer)
		if len(output.Input) > 0 {
			i.inputWindow = output.Input[0].ID
			i.inputGen = output.Input[0].Gen
		}
	}
}

// windows is a Window sorting helper...
type byPosition []*Window

func (p byPosition) Len() int      { return len(p) }
func (p byPosition) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p byPosition) Less(i, j int) bool {
	if p[i].Top != p[j].Top {
		return p[i].Top < p[j].Top
	}
	return p[i].Left < p[j].Left
}

// Kill ...
func (i *RemGlk) Kill() {
	i.logger.Info("received kill request")
	i.killing = true
	err := i.inPipe.Close()
	if err != nil {
		i.logger.WithError(err).Error("closing stdin")
	}

	b, err := ioutil.ReadAll(i.outPipe)
	if err != nil {
		i.logger.WithError(err).Error("clearing stdout")
	} else {
		i.logger.WithField("stdout", string(b)).Debug("clearing stdout")
	}

	b, err = ioutil.ReadAll(i.errPipe)
	if err != nil {
		i.logger.WithError(err).Error("clearing stderr")
	} else {
		i.logger.WithField("stderr", string(b)).Debug("clearing stderr")
	}

	err = i.cmd.Wait()
	if err != nil {
		// Note... it's not at all surprising that force-killing the subprocess
		// results in an error (any non-zero exit code).  Perhaps we shouldn't
		// report this as an error?
		i.logger.WithError(err).Error("waiting for completion")
		return
	}
}

func (i *RemGlk) sendCommand(command string, commandType string) {
	i.logger.WithField("command", command).Info("handling command")

	// We need to know the last gen and the correct window...
	c := &Input{
		Type:   LineInput, //commandType,
		Gen:    i.inputGen,
		Window: i.inputWindow,
		Value:  command,
	}

	i.SendInput(c)
}

// SendLine ...
func (i *RemGlk) SendLine(line string) error {
	return i.SendInput(&Input{
		Type:   LineInput,
		Gen:    i.inputGen,
		Window: i.inputWindow,
		Value:  line,
	})
}

// SendChar ...
func (i *RemGlk) SendChar(char rune) error {
	return i.SendInput(&Input{
		Type:   CharInput,
		Gen:    i.inputGen,
		Window: i.inputWindow,
		Value:  string(char),
	})
}

// SendInput ...
func (i *RemGlk) SendInput(input *Input) (err error) {
	b, err := json.Marshal(input)
	if err != nil {
		i.logger.WithError(err).Error("creating JSON")
		return
	}

	_, err = i.inPipe.Write(b)
	return err

}
