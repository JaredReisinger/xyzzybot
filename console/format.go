package console

import (
	"fmt"
	"strings"

	"github.com/JaredReisinger/xyzzybot/fizmo"
)

func formatSpan(span *fizmo.Span, singleSpan bool) string {
	// log.WithField("span", span).Debug("formatSpan")
	text := span.Text
	leading := ""
	trailing := ""

	// We sometimes get single span lines with leading/trailing whitespace
	// but formatting.  This looks bad, so we move the formatting codes to
	// "inside" the whitespace.
	if singleSpan {
		trimLeft := strings.TrimLeft(text, " ")
		trimRight := strings.TrimRight(text, " ")
		leading = strings.Repeat(" ", len(text)-len(trimLeft))
		trailing = strings.Repeat(" ", len(text)-len(trimRight))
		text = strings.Trim(text, " ")
	}

	boldFmt := ""
	italicFmt := ""
	fixedFmt := ""

	if span.Bold {
		boldFmt = "*"
	}

	if span.Italic {
		italicFmt = "_"
	}

	if span.Fixed {
		fixedFmt = "`"
	}

	formatted := fmt.Sprintf("%s%s%s%s%s%s%s%s%s", leading, italicFmt, boldFmt, fixedFmt, text, fixedFmt, boldFmt, italicFmt, trailing)

	return formatted
}

func formatSpans(spans *fizmo.Spans) string {
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

func formatLine(line *fizmo.Line) string {
	return formatSpans(line.Text)
}

func formatColumn(column *fizmo.Column) string {
	parts := make([]string, 0)
	for _, line := range column.Lines {
		parts = append(parts, formatLine(line))
	}
	return strings.Join(parts, " / ")
}

func formatStatus(status *fizmo.Status) string {
	lines := make([]string, 0)
	for _, col := range status.Columns {
		lines = append(lines, formatColumn(col))
	}

	return strings.Join(lines, "\n")
}

func formatDebugOutput(output *fizmo.Output) string {
	// This is where we'd want to infer status windows, etc.
	sep1 := strings.Repeat("=", 79)
	sep2 := strings.Repeat("-", 79)
	lines := []string{"", sep1}

	// lines = append(lines, fmt.Sprintf("type: %s, gen: %d", output.Type, output.Gen))
	for _, spans := range output.Story {
		lines = append(lines, formatSpans(spans))
	}

	lines = append(lines, sep2)
	lines = append(lines, formatStatus(output.Status))

	// if output.Message != nil {
	// 	lines = append(lines, sep2)
	// 	lines = append(lines, fmt.Sprintf("message: %s", *output.Message))
	// }

	lines = append(lines, sep1)

	return fmt.Sprintf(strings.Join(lines, "\n"))
}
