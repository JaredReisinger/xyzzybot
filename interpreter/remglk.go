package interpreter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
)

// Output info...

// GlkWindow ...
type GlkWindow struct {
	ID         int
	Type       GlkWindowType // string
	Rock       int
	GridWidth  *int
	GridHeight *int
	Left       int
	Top        int
	Width      int
	Height     int

	// Content is a helper reference so that it's easier to process window
	// content by position/window, rather than having each listener have to map
	// content to window.
	Content *GlkWindowContent `json:"-"`
}

func (w *GlkWindow) String() string {
	return fmt.Sprintf("(WINDOW %d (%s): @%d,%d, %dx%d)", w.ID, w.Type, w.Left, w.Top, w.Width, w.Height)
}

// GlkSpan ...
type GlkSpan struct {
	Style GlkSpanStyle
	Text  string
}

func (t *GlkSpan) String() string {
	return fmt.Sprintf("%s:%q", t.Style, t.Text)
}

// SlackString formats the span using Slack markdown.
func (t *GlkSpan) SlackString() string {
	format := "%s"
	switch t.Style {
	case NormalSpanStyle:
		format = "%s"
	case EmphasizedSpanStyle:
		format = "_%s_"
	case PreformattedSpanStyle:
		format = "`%s`"
	case HeaderSpanStyle:
		format = "*%s*"
	case SubheaderSpanStyle:
		format = "*%s*"
	case AlertSpanStyle:
		format = "[%s]"
	case NoteSpanStyle:
		format = "[%s]"
	case BlockQuoteSpanStyle:
		format = "> %s"
	case InputSpanStyle:
		format = "%s"
	case User1SpanStyle:
		format = "%s"
	case User2SpanStyle:
		format = "%s"
	}

	return fmt.Sprintf(format, t.Text)
}

// GlkSpans ...
type GlkSpans []*GlkSpan

func (s *GlkSpans) String() string {
	if s == nil {
		return ""
	}
	l := make([]string, 0, len(*s))
	for _, c := range *s {
		l = append(l, c.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(l, ", "))
}

// SlackString formats the spans using Slack markdown.
func (s *GlkSpans) SlackString() string {
	if s == nil {
		return ""
	}
	l := make([]string, 0, len(*s))
	for _, c := range *s {
		l = append(l, c.SlackString())
	}
	return strings.Join(l, "")
}

// GlkTextContent ...
type GlkTextContent struct {
	Append  *bool
	Content GlkSpans
}

func (t *GlkTextContent) String() string {
	append := ""
	if t.Append != nil {
		append = "APPEND:"
	}
	return fmt.Sprintf("%s%s", append, t.Content)
}

// SlackString formats the spans using Slack markdown.
func (t *GlkTextContent) SlackString() string {
	return t.Content.SlackString()
}

// GlkLine ...
type GlkLine struct {
	Line    int
	Content GlkSpans
}

func (l *GlkLine) String() string {
	return fmt.Sprintf("line %d: %s", l.Line, l.Content)
}

// SlackString formats the spans using Slack markdown.
func (l *GlkLine) SlackString() string {
	return l.Content.SlackString()
}

// GlkWindowContent ...
type GlkWindowContent struct {
	ID    int
	Clear *bool
	Lines []*GlkLine
	Text  []*GlkTextContent
}

func (w *GlkWindowContent) String() string {
	l := make([]string, 0, 1+len(w.Lines)+len(w.Text))
	l = append(l, fmt.Sprintf("==================== window %d ====================", w.ID))
	for _, c := range w.Lines {
		l = append(l, c.String())
	}
	for _, c := range w.Text {
		l = append(l, c.String())
	}
	return strings.Join(l, "\n")
}

// SlackString formats the spans using Slack markdown.
func (w *GlkWindowContent) SlackString() string {
	l := make([]string, 0, len(w.Lines)+len(w.Text))
	for _, c := range w.Lines {
		l = append(l, c.SlackString())
	}
	for _, c := range w.Text {
		l = append(l, c.SlackString())
	}
	return strings.Join(l, "\n")
}

// GlkInputRequest ...
type GlkInputRequest struct {
	ID     int
	Gen    int
	Type   string
	MaxLen int
}

// ProcessRemGlkOutput ...
func (i *Interpreter) ProcessRemGlkOutput() {
	var windows []*GlkWindow
	decoder := json.NewDecoder(i.outPipe)
	for {
		output := &GlkOutput{}
		err := decoder.Decode(&output)
		if err == io.EOF {
			i.logger.Info("read EOF")
			break
		} else if err != nil {
			i.logger.WithError(err).Error("decoding JSON")
			// skip/eat the error? perhaps we need a way to pass errors along to
			// any listeners?
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
		i.inputWindow = output.Input[0].ID
		i.inputGen = output.Input[0].Gen
	}
}

// windows is a Window sorting helper...
type byPosition []*GlkWindow

func (p byPosition) Len() int      { return len(p) }
func (p byPosition) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p byPosition) Less(i, j int) bool {
	if p[i].Top != p[j].Top {
		return p[i].Top < p[j].Top
	}
	return p[i].Left < p[j].Left
}

// Kill ...
func (i *Interpreter) Kill() {
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

// ProcessInput ...
func (i *Interpreter) ProcessInput() {
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

// InputCommand ...
type InputCommand struct {
	Type   string `json:"type"`
	Gen    int    `json:"gen"`
	Window int    `json:"window"`
	Value  string `json:"value"`
}

// SendCommand ...
func (i *Interpreter) SendCommand(command string) {
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
