package interpreter // import "github.com/JaredReisinger/xyzzybot/interpreter"

import (
	"encoding/json"
	"fmt"
)

// GlkOutput ...
type GlkOutput struct {
	Type    GlkOutputType
	Gen     int
	Windows []*GlkWindow
	Content []*GlkWindowContent
	Input   []*GlkInputRequest
	Message *string // used for error only
}

// GlkOutputType ...
type GlkOutputType int

//go:generate stringer -type=GlkOutputType

// It's hard to tell, but I believe the only output types are "update" and
// "error".
const (
	UpdateOutputType GlkOutputType = iota
	ErrorOutputType
)

// UnmarshalJSON parses Glk output types.
func (enum *GlkOutputType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("GlkOutputType should be a string, got %s", data)
	}

	var nameToValue = map[string]GlkOutputType{
		"update": UpdateOutputType,
		"error":  ErrorOutputType,
	}

	v, ok := nameToValue[s]
	if !ok {
		return fmt.Errorf("invalid GlkOutputType %q", s)
	}

	*enum = v
	return nil
}

// GlkWindowType defines the type/handling for an interpreter window.  Note that
// this is a "physical" description, and we may attempt to infer semantic
// meaning from it.  For example, a TextGridWindowType at position 0,0 and of
// relatively few lines is likely a status window.
type GlkWindowType int

//go:generate stringer -type=GlkWindowType

// These types happen to align in value with the wintype_* values listed in
// glk.h
const (
	_ GlkWindowType = iota // AllWindowTypes
	PairWindowType
	BlankWindowType
	TextBufferWindowType
	TextGridWindowType
	GraphicsWindowType
)

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
func (enum *GlkWindowType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("GlkWindowType should be a string, got %s", data)
	}

	var nameToValue = map[string]GlkWindowType{
		// "(all)":    AllWindowTypes, // we should never parse this value
		// "pair":     PairWindowType,
		// "blank":    BlankWindowType, //
		"buffer":   TextBufferWindowType,
		"grid":     TextGridWindowType,
		"graphics": GraphicsWindowType,
	}

	v, ok := nameToValue[s]
	if !ok {
		return fmt.Errorf("invalid GlkWindowType %q", s)
	}

	*enum = v
	return nil
}

// GlkSpanStyle ...
type GlkSpanStyle int

//go:generate stringer -type=GlkSpanStyle

// GlkSpanStyle values, see glk.h, lines 126-136 (style_*), and string
// representations in remglk/rgdata.c name_for_style(), lines 119-147
const (
	NormalSpanStyle GlkSpanStyle = iota
	EmphasizedSpanStyle
	PreformattedSpanStyle
	HeaderSpanStyle
	SubheaderSpanStyle
	AlertSpanStyle
	NoteSpanStyle
	BlockQuoteSpanStyle
	InputSpanStyle
	User1SpanStyle
	User2SpanStyle

	UnknownSpanStyle GlkSpanStyle = -1
)

// UnmarshalJSON ...
func (enum *GlkSpanStyle) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("GlkSpanStyle should be a string, got %s", data)
	}

	var nameToValue = map[string]GlkSpanStyle{
		"normal":       NormalSpanStyle,
		"emphasized":   EmphasizedSpanStyle,
		"preformatted": PreformattedSpanStyle,
		"header":       HeaderSpanStyle,
		"subheader":    SubheaderSpanStyle,
		"alert":        AlertSpanStyle,
		"note":         NoteSpanStyle,
		"blockquote":   BlockQuoteSpanStyle,
		"input":        InputSpanStyle,
		"user1":        User1SpanStyle,
		"user2":        User2SpanStyle,
	}

	// remglk might encounter unanticipated span styles, and it will send the
	// string "unknown".  Rather than including it in the list, we handle *any*
	// unexpected/unforseen value as unknown.
	v, ok := nameToValue[s]
	if !ok {
		// return fmt.Errorf("invalid GlkSpanStyle %q", s)
		v = UnknownSpanStyle
	}

	*enum = v
	return nil
}
