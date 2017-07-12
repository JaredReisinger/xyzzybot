package slack

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/JaredReisinger/xyzzybot/glk"
)

func formatSpan(span *glk.Span, singleSpan bool) string {
	// log.WithField("span", span).Debug("formatSpan")
	formatted := ""
	format := "%s"
	switch span.Style {
	case glk.NormalSpan:
		format = "%s"
	case glk.EmphasizedSpan:
		format = "_%s_"
	case glk.PreformattedSpan:
		format = "`%s`"
		// If there's leading whitespace, and this is a single span, we
		// probably want to make the leading (and any trailing) whitepace *not*
		// preformatted.
		if singleSpan && span.Text[0] == ' ' {
			// log.Debug("leading-space, single-span preformatted text!")
			mid := strings.TrimLeft(span.Text, " ")
			formatted = fmt.Sprintf("%s`%s`", strings.Repeat(" ", len(span.Text)-len(mid)), mid)
		}
	case glk.HeaderSpan:
		format = "*%s*"
	case glk.SubheaderSpan:
		format = "*%s*"
	case glk.AlertSpan:
		format = "[alert: %s]"
	case glk.NoteSpan:
		format = "[note: %s]"
	case glk.BlockQuoteSpan:
		format = "> %s"
	case glk.InputSpan:
		if singleSpan {
			format = "_command: *%q*_\n"
		} else {
			format = "_command: *%q*_"
		}
	case glk.User1Span:
		format = "%s"
	case glk.User2Span:
		format = "%s"
	default:
		log.WithField("style", span.Style).Warn("unknown style")
	}

	if formatted == "" {
		formatted = fmt.Sprintf(format, span.Text)
	}

	return formatted
}

func formatSpans(spans *glk.Spans) string {
	// log.WithField("spans", spans).Debug("formatSpans")
	if spans == nil {
		return ""
	}

	line := make([]string, 0, len(*spans))
	singleSpan := len(*spans) == 1
	for _, s := range *spans {
		line = append(line, formatSpan(s, singleSpan))
	}

	// Slack can't deal with consecutive-but-not-nested formatting.  For
	// example, ` _this_*does*_not_*work* `.  We can, however, put a
	// zero-width-joiner (\u200d) between each span, and it appears to render
	// correctly: ` _this_\u200d*does*\u200d_work_ `
	return strings.Join(line, "\u200d")
}

func formatTextContent(text *glk.TextContent) string {
	// log.WithField("text", text).Debug("formatTextContent")
	return formatSpans(text.Content)
}

func formatLine(line *glk.Line) string {
	// log.WithField("line", line).Debug("formatLine")
	return formatSpans(line.Content)
}

func formatWindowContent(window *glk.WindowContent) string {
	// log.WithField("window", window).Debug("formatWindowContent")

	// A WindowContent will have *either* Lines or Text... we just let the
	// range operator short-circuit for us when empty.
	lines := make([]string, 0, len(window.Lines)+len(window.Text))
	for _, l := range window.Lines {
		lines = append(lines, formatLine(l))
	}

	for _, t := range window.Text {
		lines = append(lines, formatTextContent(t))
	}

	return strings.Join(lines, "\n")
}

func formatWindow(window *glk.Window) string {
	// log.WithField("window", window).Debug("formatWindow")
	return formatWindowContent(window.Content)
}

func formatOutput(output *glk.Output) string {
	// log.WithField("output", output).Debug("formatOutput")

	// This is where we'd want to infer status windows, etc.

	sep1 := "============================================================"
	sep2 := "------------------------------------------------------------"
	lines := []string{sep1}

	for _, w := range output.Windows {
		lines = append(lines, formatWindow(w))
		lines = append(lines, sep2)
	}

	lines = append(lines, sep1)

	return fmt.Sprintf(strings.Join(lines, "\n"))
}
