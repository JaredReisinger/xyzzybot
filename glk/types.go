package glk // import "github.com/JaredReisinger/xyzzybot/glk"

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Input ...
type Input struct {
	Type   InputType `json:"type"`
	Gen    int       `json:"gen"`
	Window int       `json:"window"`
	Value  string    `json:"value"`
}

// InputType ...
type InputType int

// Values of InputType...
const (
	LineInput InputType = iota // regular commands
	CharInput                  // individual keystrokes
)

func (enum InputType) String() string {
	switch enum {
	case LineInput:
		return "line"
	case CharInput:
		return "char"
	}

	return ""
}

// UnmarshalJSON parses Glk input types.
func (enum *InputType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("InputType should be a string, got %s", data)
	}

	var nameToValue = map[string]InputType{
		"line": LineInput,
		"char": CharInput,
	}

	v, ok := nameToValue[s]
	if !ok {
		return fmt.Errorf("invalid InputType %q", s)
	}

	*enum = v
	return nil
}

// MarshalJSON ...
func (enum InputType) MarshalJSON() ([]byte, error) {
	var s string

	switch enum {
	case LineInput:
		s = "line"
	case CharInput:
		s = "char"
	default:
		return nil, &json.UnsupportedValueError{
			Value: reflect.ValueOf(enum),
			Str:   "unknown InputType value",
		}
	}

	return json.Marshal(s)
}

// Output ...
type Output struct {
	Type    OutputType
	Gen     int
	Windows []*Window
	Content []*WindowContent
	Input   []*InputRequest
	Message *string // used for error only
}

// OutputType ...
type OutputType int

// It's hard to tell, but I believe the only output types are "update" and
// "error".
const (
	UpdateOutputType OutputType = iota
	ErrorOutputType
)

func (enum OutputType) String() string {
	switch enum {
	case UpdateOutputType:
		return "update"
	case ErrorOutputType:
		return "error"
	}

	return ""
}

// UnmarshalJSON parses Glk output types.
func (enum *OutputType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("OutputType should be a string, got %s", data)
	}

	var nameToValue = map[string]OutputType{
		"update": UpdateOutputType,
		"error":  ErrorOutputType,
	}

	v, ok := nameToValue[s]
	if !ok {
		return fmt.Errorf("invalid OutputType %q", s)
	}

	*enum = v
	return nil
}

// Window ...
type Window struct {
	ID         int
	Type       WindowType // string
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
	Content *WindowContent `json:"-"`
}

func (w *Window) String() string {
	return fmt.Sprintf("(WINDOW %d (%s): @%d,%d, %dx%d)", w.ID, w.Type, w.Left, w.Top, w.Width, w.Height)
}

// Span ...
type Span struct {
	Style SpanStyle
	Text  string
}

func (t *Span) String() string {
	return fmt.Sprintf("%s:%q", t.Style, t.Text)
}

// Spans ...
type Spans []*Span

func (s *Spans) String() string {
	if s == nil {
		return ""
	}
	l := make([]string, 0, len(*s))
	for _, c := range *s {
		l = append(l, c.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(l, ", "))
}

// TextContent ...
type TextContent struct {
	Append  *bool
	Content *Spans
}

func (t *TextContent) String() string {
	append := ""
	if t.Append != nil {
		append = "APPEND:"
	}
	return fmt.Sprintf("%s%s", append, t.Content)
}

// Line ...
type Line struct {
	Line    int
	Content *Spans
}

func (l *Line) String() string {
	return fmt.Sprintf("line %d: %s", l.Line, l.Content)
}

// WindowContent ...
type WindowContent struct {
	ID    int
	Clear *bool
	Lines []*Line
	Text  []*TextContent
}

func (w *WindowContent) String() string {
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

// InputRequest ...
type InputRequest struct {
	ID     int
	Gen    int
	Type   string
	MaxLen int
}

// WindowType defines the type/handling for an interpreter window.  Note that
// this is a "physical" description, and we may attempt to infer semantic
// meaning from it.  For example, a TextGridWindow at position 0,0 and of
// relatively few lines is likely a status window.
type WindowType int

// These types happen to align in value with the wintype_* values listed in
// glk.h
const (
	_ WindowType = iota // AllWindows
	PairWindow
	BlankWindow
	TextBufferWindow
	TextGridWindow
	GraphicsWindow
)

func (enum WindowType) String() string {
	switch enum {
	case PairWindow:
		return "pair"
	case BlankWindow:
		return "blank"
	case TextBufferWindow:
		return "buffer"
	case TextGridWindow:
		return "grid"
	case GraphicsWindow:
		return "graphics"
	}

	return ""
}

// As much as it would be nice to use `go:generate jsonenums` to auto-create the
// MarshalJSON and UnmarshalJSON implementations, it requires the string
// representation to be the constant itself.  Ideally, it would find/use an
// override representation from a comment or somesuch.  (But that's a project
// for another time.)  Also, we only implement the Unmarshaler interface,
// because that's the only one we need at present.

// UnmarshalJSON parses Glk window types.  Note that remglk only ever sends
// values for "buffer", "grid", and "graphics" (see remglk/rgdata.c
// data_window_print(), lines 1461-1474).  The pair and blank types don't have a
// known string representation.
func (enum *WindowType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("WindowType should be a string, got %s", data)
	}

	var nameToValue = map[string]WindowType{
		// "(all)":    AllWindows, // we should never parse this value
		// "pair":     PairWindow,
		// "blank":    BlankWindow, //
		"buffer":   TextBufferWindow,
		"grid":     TextGridWindow,
		"graphics": GraphicsWindow,
	}

	v, ok := nameToValue[s]
	if !ok {
		return fmt.Errorf("invalid WindowType %q", s)
	}

	*enum = v
	return nil
}

// SpanStyle ...
type SpanStyle int

// SpanStyle values, see glk.h, lines 126-136 (style_*), and string
// representations in remglk/rgdata.c name_for_style(), lines 119-147
const (
	NormalSpan SpanStyle = iota
	EmphasizedSpan
	PreformattedSpan
	HeaderSpan
	SubheaderSpan
	AlertSpan
	NoteSpan
	BlockQuoteSpan
	InputSpan
	User1Span
	User2Span

	UnknownSpan SpanStyle = -1
)

func (enum SpanStyle) String() string {
	switch enum {
	case NormalSpan:
		return "normal"
	case EmphasizedSpan:
		return "emphasized"
	case PreformattedSpan:
		return "preformatted"
	case HeaderSpan:
		return "header"
	case SubheaderSpan:
		return "subheader"
	case AlertSpan:
		return "alert"
	case NoteSpan:
		return "note"
	case BlockQuoteSpan:
		return "blockquote"
	case InputSpan:
		return "input"
	case User1Span:
		return "user1"
	case User2Span:
		return "user2"
	}
	return ""
}

// UnmarshalJSON ...
func (enum *SpanStyle) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("SpanStyle should be a string, got %s", data)
	}

	var nameToValue = map[string]SpanStyle{
		"normal":       NormalSpan,
		"emphasized":   EmphasizedSpan,
		"preformatted": PreformattedSpan,
		"header":       HeaderSpan,
		"subheader":    SubheaderSpan,
		"alert":        AlertSpan,
		"note":         NoteSpan,
		"blockquote":   BlockQuoteSpan,
		"input":        InputSpan,
		"user1":        User1Span,
		"user2":        User2Span,
	}

	// remglk might encounter unanticipated span styles, and it will send the
	// string "unknown".  Rather than including it in the list, we handle *any*
	// unexpected/unforseen value as unknown.
	v, ok := nameToValue[s]
	if !ok {
		// return fmt.Errorf("invalid SpanStyle %q", s)
		v = UnknownSpan
	}

	*enum = v
	return nil
}
