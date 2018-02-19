package fizmo // import "github.com/JaredReisinger/xyzzybot/fizmo"

import (
	"bufio"
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// ExternalProcessFactory ...
type ExternalProcessFactory struct {
	Logger log.FieldLogger
}

// NewInterpreter ...
func (f *ExternalProcessFactory) NewInterpreter(gameFile string, workingDir string,
	fields log.Fields) (i Interpreter, err error) {
	logger := f.Logger.WithField("component", "fizmocmd").WithFields(fields)

	// TODO: implement autosave/autorestore, and ensure regular save/restore
	// is unavailable.

	cmd := exec.Command("fizmo-json",
		// "--trace-level", "1",
		// // "-savegame-path", "/Users/jreising/.savegames",
		// "-autosave-filename", "autosave",
		// "-restore", "autosave.glksave",
		gameFile)

	// Set the working directory so that game saves and other incidentals
	// happen in the correct location.
	cmd.Dir = workingDir

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

	i = &interpreter{
		logger:  logger,
		cmd:     cmd,
		inPipe:  inPipe,
		outPipe: outPipe,
		errPipe: errPipe,

		Output: make(chan *Output, 5),
	}

	return
}

// interpreter represents the interpreter command/output interaction.
type interpreter struct {
	// magic   int
	logger  log.FieldLogger
	cmd     *exec.Cmd
	inPipe  io.WriteCloser
	outPipe io.ReadCloser
	errPipe io.ReadCloser
	killing bool

	// inputGen    int

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
func (i *interpreter) GetOutputChannel() chan *Output {
	return i.Output
}

// Start starts up the interpreter
func (i *interpreter) Start() error {
	// Kick off the out/err listeners?
	go i.ProcessOutput()
	go i.ProcessErr()

	// go i.debugOutput()

	err := i.cmd.Start()
	if err != nil {
		i.logger.WithError(err).Error("starting child process")
		i.Kill()
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
// func (i *interpreter) debugOutput() {
// 	for {
// 		output := <-i.Output
// 		i.logger.WithField("output", output).Debug("recieved output")
// 	}
// }

// ProcessOutput ...
func (i *interpreter) ProcessOutput() {
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
			i.logger.WithError(err).Error("decoding JSON X")
			// skip/eat the error? perhaps we need a way to pass errors along to
			// any listeners?
			remaining := decoder.Buffered()
			b, _ := ioutil.ReadAll(remaining)
			i.logger.WithField("remaining", string(b)).Debug("ate remaining buffer")
			continue
		}

		// Send the output (TODO: to multiple listeners?)
		i.Output <- output

		// i.logger.WithField("output", output).Debug("read JSON")
	}
}

// ProcessErr ...
func (i *interpreter) ProcessErr() {

	scanner := bufio.NewScanner(i.errPipe)

	for scanner.Scan() {
		t := scanner.Text()
		i.logger.WithField("stderr", t).Debug("reading errPipe")
	}
}

// Kill ...
func (i *interpreter) Kill() {
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

func (i *interpreter) Send(input string) error {
	i.logger.WithFields(log.Fields{
		"input": input,
	}).Info("sending")

	b, err := json.Marshal(&Input{Input: input})
	if err != nil {
		i.logger.WithError(err).Error("creating JSON")
		return err
	}

	_, err = i.inPipe.Write(b)
	return err

}
