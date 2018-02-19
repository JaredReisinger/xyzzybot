package fizmo // import "github.com/JaredReisinger/xyzzybot/fizmo"

import (
	"fmt"
	"strings"
)

// Input ...
type Input struct {
	// Type   InputType `json:"type"`
	Input string `json:"input"`
}

// // InputType ...
// type InputType int
//
// // Values of InputType...
// const (
// 	LineInput InputType = iota // regular commands
// 	CharInput                  // individual keystrokes
// )
//
// func (enum InputType) String() string {
// 	switch enum {
// 	case LineInput:
// 		return "line"
// 	case CharInput:
// 		return "char"
// 	}
//
// 	return ""
// }
//
// // UnmarshalJSON parses Glk input types.
// func (enum *InputType) UnmarshalJSON(data []byte) error {
// 	var s string
// 	if err := json.Unmarshal(data, &s); err != nil {
// 		return fmt.Errorf("InputType should be a string, got %s", data)
// 	}
//
// 	var nameToValue = map[string]InputType{
// 		"line": LineInput,
// 		"char": CharInput,
// 	}
//
// 	v, ok := nameToValue[s]
// 	if !ok {
// 		return fmt.Errorf("invalid InputType %q", s)
// 	}
//
// 	*enum = v
// 	return nil
// }
//
// // MarshalJSON ...
// func (enum InputType) MarshalJSON() ([]byte, error) {
// 	var s string
//
// 	switch enum {
// 	case LineInput:
// 		s = "line"
// 	case CharInput:
// 		s = "char"
// 	default:
// 		return nil, &json.UnsupportedValueError{
// 			Value: reflect.ValueOf(enum),
// 			Str:   "unknown InputType value",
// 		}
// 	}
//
// 	return json.Marshal(s)
// }

// Output ...
type Output struct {
	// Type    OutputType
	Status *Status
	Story  []*Spans
	// Message *string // used for error only
}

// Status ...
type Status struct {
	Columns []*Column
	Lines   []*Spans
}

// Column ...
type Column struct {
	Column int
	Lines  []*Line
}

// Line ...
type Line struct {
	Line int
	Text *Spans
}

// // OutputType ...
// type OutputType int
//
// // It's hard to tell, but I believe the only output types are "update" and
// // "error".
// const (
// 	UpdateOutputType OutputType = iota
// 	ErrorOutputType
// )
//
// func (enum OutputType) String() string {
// 	switch enum {
// 	case UpdateOutputType:
// 		return "update"
// 	case ErrorOutputType:
// 		return "error"
// 	}
//
// 	return ""
// }
//
// // UnmarshalJSON parses Glk output types.
// func (enum *OutputType) UnmarshalJSON(data []byte) error {
// 	var s string
// 	if err := json.Unmarshal(data, &s); err != nil {
// 		return fmt.Errorf("OutputType should be a string, got %s", data)
// 	}
//
// 	var nameToValue = map[string]OutputType{
// 		"update": UpdateOutputType,
// 		"error":  ErrorOutputType,
// 	}
//
// 	v, ok := nameToValue[s]
// 	if !ok {
// 		return fmt.Errorf("invalid OutputType %q", s)
// 	}
//
// 	*enum = v
// 	return nil
// }

// Span ...
type Span struct {
	Bold    bool
	Italic  bool
	Reverse bool
	Fixed   bool
	Text    string
}

func (t *Span) String() string {
	// return fmt.Sprintf("%s:%q", t.Style, t.Text)
	return fmt.Sprintf("%v,%v,%v,%v:%q", t.Bold, t.Italic, t.Reverse, t.Fixed, t.Text)
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
