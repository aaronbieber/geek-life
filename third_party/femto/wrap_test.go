package femto

import (
	"strings"
	"testing"
)

// render turns wrapLineIndices output back into visible strings per row, so we
// can assert wrapping behavior directly.
func render(line string, width int) []string {
	runes := []rune(line)
	rows := wrapLineIndices(runes, 4, width)
	out := make([]string, len(rows))
	for i, row := range rows {
		var b strings.Builder
		for _, idx := range row {
			b.WriteRune(runes[idx])
		}
		out[i] = b.String()
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("row count: got %d %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("row %d: got %q, want %q (all: %q)", i, got[i], want[i], got)
		}
	}
}

func TestWrapWordBoundary(t *testing.T) {
	// "hello world foo" at width 8 must break on whitespace, not mid-word.
	eq(t, render("hello world foo", 8), []string{"hello", "world", "foo"})
}

func TestWrapNoLeadingWhitespace(t *testing.T) {
	// The space a wrap lands on must not appear at the start of the next row.
	for _, row := range render("aaaa bbbb cccc dddd", 10) {
		if strings.HasPrefix(row, " ") {
			t.Fatalf("row starts with whitespace: %q", row)
		}
	}
}

func TestWrapCollapsesMultipleSpaces(t *testing.T) {
	// Multiple spaces at the break are all consumed.
	eq(t, render("hello    world", 8), []string{"hello", "world"})
}

func TestWrapPreservesLeadingIndent(t *testing.T) {
	// Author-typed indentation at the very start of the line is preserved.
	rows := render("    a long enough sentence here", 12)
	if !strings.HasPrefix(rows[0], "    ") {
		t.Fatalf("leading indent lost: %q", rows[0])
	}
}

func TestWrapLongWordHardBreak(t *testing.T) {
	// A single word wider than the view is hard-broken at width.
	eq(t, render("abcdefghij", 4), []string{"abcd", "efgh", "ij"})
}

func TestWrapFitsOnOneRow(t *testing.T) {
	eq(t, render("short", 80), []string{"short"})
}

func TestWrapWordThenLongWord(t *testing.T) {
	// A normal word followed by an over-long word: break on the space, then
	// hard-break the long word.
	eq(t, render("hi abcdefghij", 4), []string{"hi", "abcd", "efgh", "ij"})
}

func TestWrapRowContaining(t *testing.T) {
	// "aaaa bbbb cccc" at width 6 wraps to ["aaaa","bbbb","cccc"].
	line := []rune("aaaa bbbb cccc")
	rows := wrapLineIndices(line, 4, 6)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d: %v", len(rows), rows)
	}
	n := len(line)
	cases := []struct {
		x, want int
	}{
		{0, 0}, {3, 0},
		{4, 0},  // the dropped space after "aaaa" belongs to row 0
		{5, 1},  // 'b'
		{8, 1},  //
		{10, 2}, // 'c'
		{14, 2}, // end of line
	}
	for _, tc := range cases {
		if got := wrapRowContaining(rows, tc.x, n); got != tc.want {
			t.Errorf("wrapRowContaining(x=%d) = %d, want %d", tc.x, got, tc.want)
		}
	}
}

func TestVisualColRoundTrip(t *testing.T) {
	// Moving from a column on one row to the same column on another row should
	// land on the analogous character.
	line := []rune("aaaa bbbb cccc")
	rows := wrapLineIndices(line, 4, 6) // ["aaaa","bbbb","cccc"]

	// Cursor on the 3rd char of row 1 ("bb|bb", rune index 7) is at visual col 2.
	col := visualColInRow(line, rows[1], 7, 4)
	if col != 2 {
		t.Fatalf("visualColInRow = %d, want 2", col)
	}
	// Same column on row 2 ("cccc") should be rune index 12 ('c' at col 2).
	if got := charIndexForCol(line, rows[2], col, 4); got != 12 {
		t.Fatalf("charIndexForCol = %d, want 12", got)
	}
	// Same column on row 0 ("aaaa") should be rune index 2.
	if got := charIndexForCol(line, rows[0], col, 4); got != 2 {
		t.Fatalf("charIndexForCol = %d, want 2", got)
	}
}

func TestCharIndexForColPastEnd(t *testing.T) {
	line := []rune("aaaa bbbb cccc")
	rows := wrapLineIndices(line, 4, 6)
	// A column past the end of a wrapped (non-final) row maps to just after its
	// last rune (the dropped whitespace position), never beyond the line.
	got := charIndexForCol(line, rows[0], 99, 4)
	if got != 4 {
		t.Fatalf("charIndexForCol past end = %d, want 4", got)
	}
	// Past the end of the final row maps to end-of-line.
	if got := charIndexForCol(line, rows[2], 99, 4); got != len(line) {
		t.Fatalf("charIndexForCol final past end = %d, want %d", got, len(line))
	}
}
