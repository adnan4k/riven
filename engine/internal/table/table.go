// Package table renders plain-text tables in the same box style as Laravel's
// $this->table() (Symfony Console's default style), so engine output lines up
// with what users see from php artisan.
package table

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Render writes headers and rows as:
//
//	+-----+------+
//	| URI | Name |
//	+-----+------+
//	| /   | home |
//	+-----+------+
//
// Rows shorter than headers are padded with empty cells.
func Render(w io.Writer, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = width(h)
	}
	for _, row := range rows {
		for i := range headers {
			if i < len(row) {
				widths[i] = max(widths[i], width(row[i]))
			}
		}
	}

	var b strings.Builder
	border := separator(widths)

	b.WriteString(border)
	writeRow(&b, widths, headers)
	b.WriteString(border)
	for _, row := range rows {
		writeRow(&b, widths, row)
	}
	b.WriteString(border)

	_, err := io.WriteString(w, b.String())
	return err
}

func separator(widths []int) string {
	var b strings.Builder
	for _, w := range widths {
		b.WriteString("+" + strings.Repeat("-", w+2))
	}
	b.WriteString("+\n")
	return b.String()
}

func writeRow(b *strings.Builder, widths []int, cells []string) {
	for i, w := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		fmt.Fprintf(b, "| %s%s ", cell, strings.Repeat(" ", w-width(cell)))
	}
	b.WriteString("|\n")
}

// width counts runes, not bytes, so non-ASCII names don't break alignment.
// (Wide CJK characters would still be off by one column each.)
func width(s string) int {
	return utf8.RuneCountInString(s)
}
