package slack

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/fizmo-slack/interpreter"
)

func FormatSpan(span *interpreter.GlkSpan) string {
	// log.WithField("span", span).Debug("FormatSpan")
	format := "%s"
	switch span.Style {
	case interpreter.NormalSpanStyle:
		format = "%s"
	case interpreter.EmphasizedSpanStyle:
		format = "_%s_"
	case interpreter.PreformattedSpanStyle:
		format = "`%s`"
	case interpreter.HeaderSpanStyle:
		format = "*%s*"
	case interpreter.SubheaderSpanStyle:
		format = "*%s*"
	case interpreter.AlertSpanStyle:
		format = "[%s]"
	case interpreter.NoteSpanStyle:
		format = "[%s]"
	case interpreter.BlockQuoteSpanStyle:
		format = "> %s"
	case interpreter.InputSpanStyle:
		format = "%s"
	case interpreter.User1SpanStyle:
		format = "%s"
	case interpreter.User2SpanStyle:
		format = "%s"
	default:
		log.WithField("style", span.Style).Warn("unknown style")
	}

	return fmt.Sprintf(format, span.Text)
}

func FormatSpans(spans *interpreter.GlkSpans) string {
	// log.WithField("spans", spans).Debug("FormatSpans")
	if spans == nil {
		return ""
	}

	line := make([]string, 0, len(*spans))
	for _, s := range *spans {
		line = append(line, FormatSpan(s))
	}

	return strings.Join(line, "")
}

func FormatTextContent(text *interpreter.GlkTextContent) string {
	// log.WithField("text", text).Debug("FormatTextContent")
	return FormatSpans(text.Content)
}

func FormatLine(line *interpreter.GlkLine) string {
	// log.WithField("line", line).Debug("FormatLine")
	return FormatSpans(line.Content)
}

func FormatWindowContent(window *interpreter.GlkWindowContent) string {
	// log.WithField("window", window).Debug("FormatWindowContent")

	// A GlkWindowContent will have *either* Lines or Text... we just let the
	// range operator short-circuit for us when empty.
	lines := make([]string, 0, len(window.Lines)+len(window.Text))
	for _, l := range window.Lines {
		lines = append(lines, FormatLine(l))
	}

	for _, t := range window.Text {
		lines = append(lines, FormatTextContent(t))
	}

	return strings.Join(lines, "\n")
}

func FormatWindow(window *interpreter.GlkWindow) string {
	// log.WithField("window", window).Debug("FormatWindow")
	return FormatWindowContent(window.Content)
}

func FormatOutput(output *interpreter.GlkOutput) string {
	// log.WithField("output", output).Debug("FormatOutput")

	// This is where we'd want to infer status windows, etc.

	sep1 := "============================================================"
	sep2 := "------------------------------------------------------------"
	lines := []string{sep1}

	for _, w := range output.Windows {
		lines = append(lines, FormatWindow(w))
		lines = append(lines, sep2)
	}

	lines = append(lines, sep1)

	return fmt.Sprintf(strings.Join(lines, "\n"))
}
