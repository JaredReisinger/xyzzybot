package remglk

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Interp struct {
	magic  int
	logger log.FieldLogger
	// exe     string
	cmd     *exec.Cmd
	inPipe  io.WriteCloser
	outPipe io.ReadCloser
	errPipe io.ReadCloser

	inputWindow int
	inputGen    int
}

func NewInterp(logger log.FieldLogger) (interp *Interp, err error) {
	logger = logger.WithField("component", "remglk")

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

	err = cmd.Start()
	if err != nil {
		logger.WithError(err).Error("starting child process")
		return
	}

	logger = logger.WithFields(log.Fields{
		"pid": cmd.Process.Pid,
		// "game": "????",
		// "channel": "???",
	})

	logger.WithFields(log.Fields{
		// "exe": exe,
		"cmd": cmd,
	}).Info("running fizmo")

	interp = &Interp{
		logger: logger,
		// exe:     exe,
		cmd:     cmd,
		inPipe:  inPipe,
		outPipe: outPipe,
		errPipe: errPipe,
	}

	// Kick off the out/err listeners?
	go interp.ProcessOutput()
	go interp.ProcessInput()

	return
}

// Output info...
type Window struct {
	ID         int
	Type       string
	Rock       int
	GridWidth  *int
	GridHeight *int
	Left       int
	Top        int
	Width      int
	Height     int
}

func (w *Window) String() string {
	return fmt.Sprintf("(WINDOW %d (%s): @%d,%d, %dx%d)", w.ID, w.Type, w.Left, w.Top, w.Width, w.Height)
}

type TextInfo struct {
	Style string
	Text  string
}

func (t *TextInfo) String() string {
	return fmt.Sprintf("(%s) %q", t.Style, t.Text)
}

type TextInfos []*TextInfo

func (t *TextInfos) String() string {
	if t == nil {
		return ""
	}
	l := make([]string, 0, len(*t))
	for _, c := range *t {
		l = append(l, fmt.Sprintf("[%s]", c.String()))
	}
	return strings.Join(l, "")

}

type TextContent struct {
	Content TextInfos
	Append  *bool
}

func (t *TextContent) String() string {
	append := ""
	if t.Append != nil {
		append = "APPEND:"
	}
	return fmt.Sprintf("%s[%s]", append, t.Content)
}

type Line struct {
	Line    int
	Content TextInfos
}

func (l *Line) String() string {
	return fmt.Sprintf("line %d: [%s]", l.Line, l.Content)
}

type WindowContent struct {
	ID    int
	Clear *bool
	Lines []*Line
	Text  []*TextContent
}

func (w *WindowContent) String() string {
	l := make([]string, 0, 1+len(w.Lines)+len(w.Text))
	l = append(l, fmt.Sprintf("[[window %d]]", w.ID))
	for _, c := range w.Lines {
		l = append(l, c.String())
	}
	for _, c := range w.Text {
		l = append(l, c.String())
	}
	return strings.Join(l, "\n")
}

type InputRequest struct {
	ID     int
	Gen    int
	Type   string
	MaxLen int
}

type Output struct {
	Type    string
	Gen     int
	Windows []*Window
	Content []*WindowContent
	Input   []*InputRequest
}

func (i *Interp) ProcessOutput() {
	// r := bufio.NewReader(i.outPipe)
	// s := r.ReadString("\n")
	decoder := json.NewDecoder(i.outPipe)
	for {
		output := &Output{}
		err := decoder.Decode(&output)
		if err == io.EOF {
			i.logger.Info("read EOF")
			break
		} else if err != nil {
			i.logger.WithError(err).Error("decoding JSON")
			// skip/eat the error?
			// return
		}
		// i.logger.WithField("output", output).Debug("read JSON")
		// i.logger.Debugf("output: %#v", output)

		i.logger.WithFields(log.Fields{
			"type":    output.Type,
			"gen":     output.Gen,
			"windows": output.Windows,
			"input":   output.Input,
		}).Debug("read JSON")

		l := make([]string, 0)

		for _, w := range output.Content {
			// i.logger.WithFields(log.Fields{
			// 	"id":    w.ID,
			// 	"lines": w.Lines,
			// 	"text":  w.Text,
			// }).Debug("window content")
			l = append(l, w.String())
		}
		fmt.Println(strings.Join(l, "\n"))

		// Assume there's only one input?
		// (and always one?)
		i.inputWindow = output.Input[0].ID
		i.inputGen = output.Input[0].Gen
	}
}

func (i *Interp) Kill() {
	err := i.inPipe.Close()
	if err != nil {
		i.logger.WithError(err).Error("closing stdin")
	}
	// err = i.outPipe.Close()
	// if err != nil {
	// 	i.logger.WithError(err).Error("closing stdout")
	// }
	// err = i.errPipe.Close()
	// if err != nil {
	// 	i.logger.WithError(err).Error("closing stderr")
	// }
	b, err := ioutil.ReadAll(i.outPipe)
	if err != nil {
		i.logger.WithError(err).Error("reading output")
	} else {
		// i.logger.WithField("stdout", string(b)).Info("closing...")
		i.logger.Debugf("output: %q", string(b))
	}

	b, err = ioutil.ReadAll(i.errPipe)
	if err != nil {
		i.logger.WithError(err).Error("reading output")
	} else {
		i.logger.WithField("stderr", string(b)).Info("closing...")
	}

	err = i.cmd.Wait()
	if err != nil {
		i.logger.WithError(err).Error("waiting for completion")
		return
	}
}

func (i *Interp) ProcessInput() {
	// read from stdin, and send commands...
	r := bufio.NewReader(os.Stdin)
	for {
		s, err := r.ReadString("\n"[0])
		if err != nil {
			i.logger.WithError(err).Error("reading input")
			return
		}
		i.SendCommand(s[:len(s)-1])
	}
}

type InputCommand struct {
	Type   string `json:"type"`
	Gen    int    `json:"gen"`
	Window int    `json:"window"`
	Value  string `json:"value"`
}

func (i *Interp) SendCommand(command string) {
	i.logger.WithField("command", command).Info("handling command")

	// We need to know the last gen and the correct window...
	c := &InputCommand{
		Type:   "line",
		Gen:    i.inputGen,
		Window: i.inputWindow,
		Value:  command,
	}

	if command == " " {
		c.Type = "char"
	}

	b, err := json.Marshal(c)
	if err != nil {
		i.logger.WithError(err).Error("creating JSON")
		return
	}

	i.inPipe.Write(b)
}
