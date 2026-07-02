package femto

import "testing"

// wordCursor builds a buffer from content and returns its cursor positioned at
// rune index x on line y.
func wordCursor(content string, y, x int) *Cursor {
	buf := NewBufferFromString(content, "")
	buf.Cursor.X = x
	buf.Cursor.Y = y
	return &buf.Cursor
}

func TestWordStartRight(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		startX       int
		wantX, wantY int
	}{
		{"from word start", "foo bar baz", 0, 4, 0},
		{"from mid word", "foo bar baz", 1, 4, 0},
		{"across punctuation", "foo, bar", 0, 5, 0},
		{"lands past trailing spaces", "foo    bar", 0, 7, 0},
		{"crosses to next line", "foo\nbar", 1, 0, 1},
		{"stops at buffer end", "foo", 1, 3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := wordCursor(tc.content, 0, tc.startX)
			c.WordStartRight()
			if c.X != tc.wantX || c.Y != tc.wantY {
				t.Fatalf("got (%d,%d), want (%d,%d)", c.Y, c.X, tc.wantY, tc.wantX)
			}
		})
	}
}

func TestWordLeftLandsOnWordStart(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		startX       int
		wantX, wantY int
	}{
		{"from next word start", "foo bar", 4, 0, 0},
		{"from mid word to its start", "foobar", 3, 0, 0},
		{"across punctuation", "foo, bar", 5, 0, 0},
		{"crosses to previous line", "foo\nbar", 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// place cursor on the correct line for multi-line content
			y := 0
			if tc.content == "foo\nbar" {
				y = 1
			}
			c := wordCursor(tc.content, y, tc.startX)
			c.WordLeft()
			if c.X != tc.wantX || c.Y != tc.wantY {
				t.Fatalf("got (%d,%d), want (%d,%d)", c.Y, c.X, tc.wantY, tc.wantX)
			}
		})
	}
}
