package femto

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

func visualToCharPos(visualIndex int, lineN int, str string, buf *Buffer, colorscheme Colorscheme, tabsize int) (int, int, *tcell.Style) {
	charPos := 0
	var lineIdx int
	var lastWidth int
	var style *tcell.Style
	var width int
	var rw int
	for i, c := range str {
		// width := StringWidth(str[:i], tabsize)

		if group, ok := buf.Match(lineN)[charPos]; ok {
			s := colorscheme.GetColor(group.String())
			style = &s
		}

		if width >= visualIndex {
			return charPos, visualIndex - lastWidth, style
		}

		if i != 0 {
			charPos++
			lineIdx += rw
		}
		lastWidth = width
		rw = 0
		if c == '\t' {
			rw = tabsize - (lineIdx % tabsize)
			width += rw
		} else {
			rw = runewidth.RuneWidth(c)
			width += rw
		}
	}

	return -1, -1, style
}

type Char struct {
	visualLoc Loc
	realLoc   Loc
	char      rune
	// The actual character that is drawn
	// This is only different from char if it's for example hidden character
	drawChar rune
	style    tcell.Style
	width    int
}

type CellView struct {
	lines [][]*Char
}

func (c *CellView) Draw(buf *Buffer, colorscheme Colorscheme, top, height, left, width, skipRows int) {
	if width <= 0 {
		return
	}

	matchingBrace := Loc{-1, -1}
	// bracePairs is defined in buffer.go
	if buf.Settings["matchbrace"].(bool) {
		for _, bp := range bracePairs {
			curX := buf.Cursor.X
			curLoc := buf.Cursor.Loc
			if buf.Settings["matchbraceleft"].(bool) {
				if curX > 0 {
					curX--
					curLoc = curLoc.Move(-1, buf)
				}
			}

			r := buf.Cursor.RuneUnder(curX)
			if r == bp[0] || r == bp[1] {
				matchingBrace = buf.FindMatchingBrace(bp, curLoc)
			}
		}
	}

	tabsize := int(buf.Settings["tabsize"].(float64))
	softwrap := buf.Settings["softwrap"].(bool)
	indentrunes := []rune(buf.Settings["indentchar"].(string))
	// if empty indentchar settings, use space
	if indentrunes == nil || len(indentrunes) == 0 {
		indentrunes = []rune{' '}
	}
	indentchar := indentrunes[0]

	start := buf.Cursor.Y
	if buf.Settings["syntax"].(bool) && buf.syntaxDef != nil {
		if start > 0 && buf.lines[start-1].rehighlight {
			buf.highlighter.ReHighlightLine(buf, start-1)
			buf.lines[start-1].rehighlight = false
		}

		buf.highlighter.ReHighlightStates(buf, start)

		buf.highlighter.HighlightMatches(buf, top, top+height)
	}

	c.lines = make([][]*Char, 0)

	viewLine := 0
	lineN := top

	curStyle := defStyle
	for viewLine < height {
		if lineN >= len(buf.lines) {
			break
		}

		lineStr := buf.Line(lineN)
		line := []rune(lineStr)

		// Prose (word) wrapping: when soft wrap is on and the logical line is
		// wider than the view, wrap it on whitespace boundaries rather than in
		// the middle of a word, and drop the whitespace a wrap lands on so it
		// never leads a continuation row. This reflows correctly when the
		// terminal is resized because it runs on every draw.
		if softwrap && width > 0 && StringWidth(lineStr, tabsize) > width {
			// The top line may be scrolled partway in (sub-line scrolling): skip
			// its first skipRows visual rows.
			skip := 0
			if lineN == top {
				skip = skipRows
			}
			viewLine, curStyle = c.drawWrappedLine(
				buf, colorscheme, line, lineN, viewLine, height, width, tabsize, matchingBrace, indentchar, curStyle, skip,
			)
			lineN++
			continue
		}

		colN, startOffset, startStyle := visualToCharPos(left, lineN, lineStr, buf, colorscheme, tabsize)
		if colN < 0 {
			colN = len(line)
		}
		viewCol := -startOffset
		if startStyle != nil {
			curStyle = *startStyle
		}

		// We'll either draw the length of the line, or the width of the screen
		// whichever is smaller
		lineLength := min(StringWidth(lineStr, tabsize), width)
		c.lines = append(c.lines, make([]*Char, lineLength))

		wrap := false
		// We only need to wrap if the length of the line is greater than the width of the terminal screen
		if softwrap && StringWidth(lineStr, tabsize) > width {
			wrap = true
			// We're going to draw the entire line now
			lineLength = StringWidth(lineStr, tabsize)
		}

		for viewCol < lineLength {
			if colN >= len(line) {
				break
			}
			if group, ok := buf.Match(lineN)[colN]; ok {
				curStyle = colorscheme.GetColor(group.String())
			}

			char := line[colN]

			if viewCol >= 0 {
				st := curStyle
				if colN == matchingBrace.X && lineN == matchingBrace.Y && !buf.Cursor.HasSelection() {
					st = curStyle.Reverse(true)
				}
				if viewCol < len(c.lines[viewLine]) {
					c.lines[viewLine][viewCol] = &Char{Loc{viewCol, viewLine}, Loc{colN, lineN}, char, char, st, 1}
				}
			}
			if char == '\t' {
				charWidth := tabsize - (viewCol+left)%tabsize
				if viewCol >= 0 {
					c.lines[viewLine][viewCol].drawChar = indentchar
					c.lines[viewLine][viewCol].width = charWidth

					indentStyle := curStyle
					ch := buf.Settings["indentchar"].(string)
					if group, ok := colorscheme["indent-char"]; ok && !IsStrWhitespace(ch) && ch != "" {
						indentStyle = group
					}

					c.lines[viewLine][viewCol].style = indentStyle
				}

				for i := 1; i < charWidth; i++ {
					viewCol++
					if viewCol >= 0 && viewCol < lineLength && viewCol < len(c.lines[viewLine]) {
						c.lines[viewLine][viewCol] = &Char{Loc{viewCol, viewLine}, Loc{colN, lineN}, char, ' ', curStyle, 1}
					}
				}
				viewCol++
			} else if runewidth.RuneWidth(char) > 1 {
				charWidth := runewidth.RuneWidth(char)
				if viewCol >= 0 {
					c.lines[viewLine][viewCol].width = charWidth
				}
				for i := 1; i < charWidth; i++ {
					viewCol++
					if viewCol >= 0 && viewCol < lineLength && viewCol < len(c.lines[viewLine]) {
						c.lines[viewLine][viewCol] = &Char{Loc{viewCol, viewLine}, Loc{colN, lineN}, char, ' ', curStyle, 1}
					}
				}
				viewCol++
			} else {
				viewCol++
			}
			colN++

			if wrap && viewCol >= width {
				viewLine++

				// If we go too far soft wrapping we have to cut off
				if viewLine >= height {
					break
				}

				nextLine := line[colN:]
				lineLength := min(StringWidth(string(nextLine), tabsize), width)
				c.lines = append(c.lines, make([]*Char, lineLength))

				viewCol = 0
			}

		}
		if group, ok := buf.Match(lineN)[len(line)]; ok {
			curStyle = colorscheme.GetColor(group.String())
		}

		// newline
		viewLine++
		lineN++
	}

	for i := top; i < top+height; i++ {
		if i >= buf.NumLines {
			break
		}
		buf.SetMatch(i, nil)
	}
}

// runeVisualWidth returns the number of cells a rune occupies when drawn
// starting at the given visual column (tabs expand to the next tab stop).
func runeVisualWidth(r rune, col, tabsize int) int {
	if r == '\t' {
		return tabsize - (col % tabsize)
	}
	return runewidth.RuneWidth(r)
}

// wrapLineIndices splits one logical line into visual rows for prose (word)
// wrapping. Each returned row is the ordered list of rune indices from `line`
// to render on that row (always a contiguous span). When a wrap falls on a run
// of whitespace, that whitespace is dropped so no continuation row begins with
// wrapped whitespace; indentation the author typed at the very start of the
// logical line is preserved. A single word wider than the view is hard-broken
// at the view width.
func wrapLineIndices(line []rune, tabsize, width int) [][]int {
	n := len(line)
	if width <= 0 || n == 0 {
		row := make([]int, n)
		for i := range line {
			row[i] = i
		}
		return [][]int{row}
	}

	var rows [][]int
	i := 0
	for i < n {
		rowStart := i
		col := 0
		// wrapAt is the rune index of the first whitespace in the trailing
		// whitespace run we can break on (-1 until one is seen on this row).
		wrapAt := -1

		for i < n {
			r := line[i]
			rw := runeVisualWidth(r, col, tabsize)
			if col+rw > width && i > rowStart {
				break
			}
			if IsWhitespace(r) && (i == rowStart || !IsWhitespace(line[i-1])) {
				wrapAt = i
			}
			col += rw
			i++
		}

		end := i // exclusive end of this row's span
		if i < n {
			if wrapAt > rowStart {
				// Break on whitespace: end the row at the last word and skip
				// the whole whitespace run so it does not lead the next row.
				end = wrapAt
				i = wrapAt
				for i < n && IsWhitespace(line[i]) {
					i++
				}
			}
			// Otherwise (no usable whitespace) hard-break before line[i]; end
			// is already i and the next row resumes there.
		}

		row := make([]int, 0, end-rowStart)
		for k := rowStart; k < end; k++ {
			row = append(row, k)
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		rows = append(rows, []int{})
	}
	return rows
}

// visualColInRow returns the visual column, relative to the start of the display
// row, of rune index x. If x is at or past the row's end the full row width is
// returned.
func visualColInRow(line []rune, row []int, x, tabsize int) int {
	col := 0
	for _, idx := range row {
		if idx >= x {
			break
		}
		col += runeVisualWidth(line[idx], col, tabsize)
	}
	return col
}

// charIndexForCol returns the rune index in `line` that the cursor lands on for
// the given visual column of a display row. Columns at or past the row's end map
// to the position just after the row's last rune.
func charIndexForCol(line []rune, row []int, target, tabsize int) int {
	if len(row) == 0 {
		return 0
	}
	col := 0
	for _, idx := range row {
		w := runeVisualWidth(line[idx], col, tabsize)
		if col+w > target {
			return idx
		}
		col += w
	}
	return row[len(row)-1] + 1
}

// wrapRowContaining returns the index of the display row that owns rune index x.
// A row owns the span from its first rune up to (but not including) the next
// row's first rune; the final row owns everything through the end of the line.
func wrapRowContaining(rows [][]int, x, n int) int {
	for i := range rows {
		start := 0
		if len(rows[i]) > 0 {
			start = rows[i][0]
		}
		next := n
		if i+1 < len(rows) && len(rows[i+1]) > 0 {
			next = rows[i+1][0]
		}
		if x >= start && x < next {
			return i
		}
	}
	return len(rows) - 1
}

// moveCursorVisual moves the cursor one display (soft-wrapped) row up (dir < 0)
// or down (dir > 0), following the wrapped rows as shown on screen and keeping a
// sticky visual column across a run of vertical moves. It returns false when
// soft wrapping is not in effect so the caller can fall back to logical-line
// movement.
func (v *View) moveCursorVisual(dir int) bool {
	if v.Buf == nil {
		return false
	}
	if sw, ok := v.Buf.Settings["softwrap"].(bool); !ok || !sw {
		return false
	}
	width := v.width - v.lineNumOffset
	if width <= 0 {
		return false
	}

	tabsize := int(v.Buf.Settings["tabsize"].(float64))
	c := v.Cursor

	line := []rune(v.Buf.Line(c.Y))
	rows := wrapLineIndices(line, tabsize, width)
	rowIdx := wrapRowContaining(rows, c.X, len(line))

	// Recompute the sticky column only when we are starting a fresh run of
	// vertical moves (the previous event was something else).
	if !v.prevVertical {
		v.wrapStickyCol = visualColInRow(line, rows[rowIdx], c.X, tabsize)
	}
	v.curVertical = true
	target := v.wrapStickyCol

	if dir < 0 {
		switch {
		case rowIdx > 0:
			c.X = charIndexForCol(line, rows[rowIdx-1], target, tabsize)
		case c.Y > 0:
			py := c.Y - 1
			pline := []rune(v.Buf.Line(py))
			prows := wrapLineIndices(pline, tabsize, width)
			c.Y = py
			c.X = charIndexForCol(pline, prows[len(prows)-1], target, tabsize)
		default:
			return true // already on the first display row of the buffer
		}
	} else {
		switch {
		case rowIdx < len(rows)-1:
			c.X = charIndexForCol(line, rows[rowIdx+1], target, tabsize)
		case c.Y < v.Buf.NumLines-1:
			ny := c.Y + 1
			nline := []rune(v.Buf.Line(ny))
			nrows := wrapLineIndices(nline, tabsize, width)
			c.Y = ny
			c.X = charIndexForCol(nline, nrows[0], target, tabsize)
		default:
			return true // already on the last display row of the buffer
		}
	}

	return true
}

// drawWrappedLine renders a logical line (lineN) that is wider than the view,
// wrapping it on word boundaries. It appends one entry to c.lines per visual
// row and returns the next viewLine along with the running style (carried
// across logical lines for multi-line syntax highlighting).
// The `skip` argument drops the first `skip` visual rows of this line (used for
// sub-line scrolling of the top line); their style changes are still processed
// so highlighting stays continuous.
func (c *CellView) drawWrappedLine(
	buf *Buffer, colorscheme Colorscheme, line []rune, lineN, viewLine, height, width, tabsize int,
	matchingBrace Loc, indentchar rune, curStyle tcell.Style, skip int,
) (int, tcell.Style) {
	indentSetting := buf.Settings["indentchar"].(string)

	for i, row := range wrapLineIndices(line, tabsize, width) {
		rowChars := make([]*Char, 0, len(row))
		viewCol := 0
		for _, colN := range row {
			if group, ok := buf.Match(lineN)[colN]; ok {
				curStyle = colorscheme.GetColor(group.String())
			}

			char := line[colN]
			st := curStyle
			if colN == matchingBrace.X && lineN == matchingBrace.Y && !buf.Cursor.HasSelection() {
				st = curStyle.Reverse(true)
			}

			switch {
			case char == '\t':
				charWidth := tabsize - (viewCol % tabsize)
				indentStyle := st
				if group, ok := colorscheme["indent-char"]; ok && !IsStrWhitespace(indentSetting) && indentSetting != "" {
					indentStyle = group
				}
				rowChars = append(rowChars, &Char{Loc{viewCol, viewLine}, Loc{colN, lineN}, char, indentchar, indentStyle, charWidth})
				for i := 1; i < charWidth; i++ {
					rowChars = append(rowChars, &Char{Loc{viewCol + i, viewLine}, Loc{colN, lineN}, char, ' ', st, 1})
				}
				viewCol += charWidth
			case runewidth.RuneWidth(char) > 1:
				charWidth := runewidth.RuneWidth(char)
				rowChars = append(rowChars, &Char{Loc{viewCol, viewLine}, Loc{colN, lineN}, char, char, st, charWidth})
				for i := 1; i < charWidth; i++ {
					rowChars = append(rowChars, &Char{Loc{viewCol + i, viewLine}, Loc{colN, lineN}, char, ' ', st, 1})
				}
				viewCol += charWidth
			default:
				rowChars = append(rowChars, &Char{Loc{viewCol, viewLine}, Loc{colN, lineN}, char, char, st, 1})
				viewCol++
			}
		}

		if i < skip {
			continue // scrolled above the viewport; not emitted
		}
		if viewLine >= height {
			return viewLine, curStyle
		}

		c.lines = append(c.lines, rowChars)
		viewLine++
	}

	if group, ok := buf.Match(lineN)[len(line)]; ok {
		curStyle = colorscheme.GetColor(group.String())
	}

	return viewLine, curStyle
}
