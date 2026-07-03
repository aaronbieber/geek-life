package femto

import (
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The View struct stores information about a view into a buffer.
// It stores information about the cursor, and the viewport
// that the user sees the buffer from.
type View struct {
	*tview.Box

	// A pointer to the buffer's cursor for ease of access
	Cursor *Cursor

	// The topmost line, used for vertical scrolling
	Topline int
	// The visual (soft-wrapped) sub-row within Topline to start rendering from.
	// Enables sub-line scrolling so a wrapped line taller than the viewport can
	// still be scrolled through. Only meaningful when soft wrap is enabled.
	topRow int
	// The leftmost column, used for horizontal scrolling
	leftCol int

	// Specifies whether or not this view is readonly
	Readonly bool

	// Actual width and height
	width  int
	height int

	// Where this view is located
	x, y int

	// How much to offset because of line numbers
	lineNumOffset int

	// The buffer
	Buf *Buffer

	// We need to keep track of insert key press toggle
	isOverwriteMode bool
	lastLoc         Loc

	// lastCutTime stores when the last ctrl+k was issued.
	// It is used for clearing the clipboard to replace it with fresh cut lines.
	lastCutTime time.Time

	// freshClip returns true if the clipboard has never been pasted.
	freshClip bool

	// Soft-wrap vertical navigation: when the cursor moves up/down through a
	// soft-wrapped line, it should follow the displayed (visual) rows rather
	// than jumping over whole logical lines. wrapStickyCol remembers the target
	// visual column within a display row so a run of up/down keeps its column;
	// prevVertical/curVertical track whether the previous/current event was such
	// a vertical move so the sticky column is only recomputed after other edits.
	wrapStickyCol int
	prevVertical  bool
	curVertical   bool

	// The cellview used for displaying and syntax highlighting
	cellview *CellView

	// The scrollbar
	scrollbar *ScrollBar

	// The keybindings
	bindings KeyBindings

	// The colorscheme
	colorscheme Colorscheme

	// The runtime files
	runtimeFiles *RuntimeFiles
}

// NewView returns a new view with the specified buffer.
func NewView(buf *Buffer) *View {
	v := new(View)

	v.Box = tview.NewBox()

	v.x, v.y, v.width, v.height = 0, 0, 0, 0

	v.cellview = new(CellView)

	v.OpenBuffer(buf)

	v.scrollbar = &ScrollBar{
		view: v,
	}

	v.bindings = DefaultKeyBindings

	return v
}

// SetRect sets a new position for the view.
func (v *View) SetRect(x, y, width, height int) {
	v.Box.SetRect(x, y, width, height)
	v.x, v.y, v.width, v.height = v.Box.GetInnerRect()
}

// InputHandler returns a handler which received key events when this view has focus,
func (v *View) InputHandler() func(event *tcell.EventKey, _ func(p tview.Primitive)) {
	return v.WrapInputHandler(func(event *tcell.EventKey, _ func(p tview.Primitive)) {
		v.HandleEvent(event)
	})
}

// GetKeyBindings gets the keybindings for this view.
func (v *View) GetKeybindings() KeyBindings {
	return v.bindings
}

// SetKeybindings sets the keybindings for this view.
func (v *View) SetKeybindings(bindings KeyBindings) {
	v.bindings = bindings
}

// SetColorscheme sets the colorscheme for this view.
func (v *View) SetColorscheme(colorscheme Colorscheme) {
	v.colorscheme = colorscheme
	v.Buf.updateRules(v.runtimeFiles)
}

// SetRuntimeFiles sets the runtime files for this view.
func (v *View) SetRuntimeFiles(runtimeFiles *RuntimeFiles) {
	v.runtimeFiles = runtimeFiles
	v.Buf.updateRules(v.runtimeFiles)
}

func (v *View) paste(clip string) {
	if v.Buf.Settings["smartpaste"].(bool) {
		if v.Cursor.X > 0 && GetLeadingWhitespace(strings.TrimLeft(clip, "\r\n")) == "" {
			leadingWS := GetLeadingWhitespace(v.Buf.Line(v.Cursor.Y))
			clip = strings.Replace(clip, "\n", "\n"+leadingWS, -1)
		}
	}

	if v.Cursor.HasSelection() {
		v.Cursor.DeleteSelection()
		v.Cursor.ResetSelection()
	}

	v.Buf.Insert(v.Cursor.Loc, clip)
	// v.Cursor.Loc = v.Cursor.Loc.Move(Count(clip), v.Buf)
	v.freshClip = false
}

// ScrollUp scrolls the view up n lines (if possible)
func (v *View) ScrollUp(n int) {
	// With soft wrap, scroll by visual (wrapped) rows so long paragraphs scroll
	// smoothly one displayed row at a time.
	if v.Buf.Settings["softwrap"].(bool) {
		for i := 0; i < n; i++ {
			v.scrollUpOneRow()
		}
		return
	}

	// Try to scroll by n but if it would overflow, scroll by 1
	if v.Topline-n >= 0 {
		v.Topline -= n
	} else if v.Topline > 0 {
		v.Topline--
	}
}

// ScrollDown scrolls the view down n lines (if possible)
func (v *View) ScrollDown(n int) {
	if v.Buf.Settings["softwrap"].(bool) {
		for i := 0; i < n; i++ {
			v.scrollDownOneRow()
		}
		return
	}

	// Try to scroll by n but if it would overflow, scroll by 1
	if v.Topline+n <= v.Buf.NumLines {
		v.Topline += n
	} else if v.Topline < v.Buf.NumLines-1 {
		v.Topline++
	}
}

// OpenBuffer opens a new buffer in this view.
// This resets the topline, event handler and cursor.
func (v *View) OpenBuffer(buf *Buffer) {
	v.Buf = buf
	v.Cursor = &buf.Cursor
	v.Topline = 0
	v.leftCol = 0
	v.Cursor.ResetSelection()
	v.Relocate()
	v.Center()

	// Set isOverwriteMode to false, because we assume we are in the default mode when editor
	// is opened
	v.isOverwriteMode = false
}

// Bottomline returns the line number of the lowest line in the view
// You might think that this is obviously just v.Topline + v.height
// but if softwrap is enabled things get complicated since one buffer
// line can take up multiple lines in the view
func (v *View) Bottomline() int {
	if !v.Buf.Settings["softwrap"].(bool) {
		return v.Topline + v.height
	}

	screenX, screenY := 0, 0
	numLines := 0
	for lineN := v.Topline; lineN < v.Topline+v.height; lineN++ {
		line := v.Buf.Line(lineN)

		colN := 0
		for _, ch := range line {
			if screenX >= v.width-v.lineNumOffset {
				screenX = 0
				screenY++
			}

			if ch == '\t' {
				screenX += int(v.Buf.Settings["tabsize"].(float64)) - 1
			}

			screenX++
			colN++
		}
		screenX = 0
		screenY++
		numLines++

		if screenY >= v.height {
			break
		}
	}
	return numLines + v.Topline
}

// Relocate moves the view window so that the cursor is in view
// This is useful if the user has scrolled far away, and then starts typing
func (v *View) Relocate() bool {
	// Soft wrap needs scrolling measured in visual (wrapped) rows; the
	// buffer-line math below cannot keep the cursor in view when a wrapped line
	// spans more rows than the viewport.
	if v.Buf.Settings["softwrap"].(bool) {
		return v.relocateSoftwrap()
	}

	height := v.Bottomline() - v.Topline
	ret := false
	cy := v.Cursor.Y
	scrollmargin := int(v.Buf.Settings["scrollmargin"].(float64))
	if cy < v.Topline+scrollmargin && cy > scrollmargin-1 {
		v.Topline = cy - scrollmargin
		ret = true
	} else if cy < v.Topline {
		v.Topline = cy
		ret = true
	}
	if cy > v.Topline+height-1-scrollmargin && cy < v.Buf.NumLines-scrollmargin {
		v.Topline = cy - height + 1 + scrollmargin
		ret = true
	} else if cy >= v.Buf.NumLines-scrollmargin && cy >= height {
		v.Topline = v.Buf.NumLines - height
		ret = true
	}

	if !v.Buf.Settings["softwrap"].(bool) {
		cx := v.Cursor.GetVisualX()
		if cx < v.leftCol {
			v.leftCol = cx
			ret = true
		}
		if cx+v.lineNumOffset+1 > v.leftCol+v.width && v.width > cx+v.lineNumOffset+1 {
			v.leftCol = cx - v.width + v.lineNumOffset + 1
			ret = true
		}
	}
	return ret
}

// relocateSoftwrap keeps the cursor within the viewport when soft wrapping is on.
// It measures position in visual (wrapped) rows and scrolls (including partway
// into a wrapped line via topRow) only as far as needed to bring the cursor into
// view, so the viewport otherwise stays put (traditional text-box behavior).
func (v *View) relocateSoftwrap() bool {
	if v.width-v.lineNumOffset <= 0 || v.height <= 0 {
		return false
	}

	v.clampTopRow()
	ret := false

	for v.cursorOffsetFromTop() < 0 {
		if v.Topline == 0 && v.topRow == 0 {
			break
		}
		v.scrollUpOneRow()
		ret = true
	}
	for v.cursorOffsetFromTop() > v.height-1 {
		prevTop, prevRow := v.Topline, v.topRow
		v.scrollDownOneRow()
		if v.Topline == prevTop && v.topRow == prevRow {
			break // cannot scroll any further
		}
		ret = true
	}

	return ret
}

// lineVisualRows returns how many wrapped visual rows buffer line ln occupies at
// the current view width.
func (v *View) lineVisualRows(ln int) int {
	width := v.width - v.lineNumOffset
	if width <= 0 || ln < 0 || ln >= v.Buf.NumLines {
		return 1
	}
	tabsize := int(v.Buf.Settings["tabsize"].(float64))
	return len(wrapLineIndices([]rune(v.Buf.Line(ln)), tabsize, width))
}

// cursorSubRow returns the visual row within its own line that the cursor sits on.
func (v *View) cursorSubRow() int {
	width := v.width - v.lineNumOffset
	if width <= 0 {
		return 0
	}
	tabsize := int(v.Buf.Settings["tabsize"].(float64))
	line := []rune(v.Buf.Line(v.Cursor.Y))
	return wrapRowContaining(wrapLineIndices(line, tabsize, width), v.Cursor.X, len(line))
}

// cursorOffsetFromTop returns how many visual rows the cursor is below the top of
// the viewport (negative if it is above it).
func (v *View) cursorOffsetFromTop() int {
	cy := v.Cursor.Y
	rows := 0
	if cy >= v.Topline {
		for ln := v.Topline; ln < cy; ln++ {
			rows += v.lineVisualRows(ln)
		}
	} else {
		for ln := cy; ln < v.Topline; ln++ {
			rows -= v.lineVisualRows(ln)
		}
	}
	return rows + v.cursorSubRow() - v.topRow
}

// clampTopRow keeps Topline/topRow within valid ranges (e.g. after Topline was
// set directly elsewhere).
func (v *View) clampTopRow() {
	if v.Topline < 0 {
		v.Topline = 0
	}
	if v.Topline > v.Buf.NumLines-1 {
		v.Topline = v.Buf.NumLines - 1
	}
	if v.topRow < 0 {
		v.topRow = 0
	}
	if max := v.lineVisualRows(v.Topline) - 1; v.topRow > max {
		v.topRow = max
	}
}

// scrollDownOneRow scrolls the viewport down by one visual row.
func (v *View) scrollDownOneRow() {
	if v.topRow < v.lineVisualRows(v.Topline)-1 {
		v.topRow++
	} else if v.Topline < v.Buf.NumLines-1 {
		v.Topline++
		v.topRow = 0
	}
}

// scrollUpOneRow scrolls the viewport up by one visual row.
func (v *View) scrollUpOneRow() {
	if v.topRow > 0 {
		v.topRow--
	} else if v.Topline > 0 {
		v.Topline--
		v.topRow = v.lineVisualRows(v.Topline) - 1
	}
}

// Execute actions executes the supplied actions
func (v *View) ExecuteActions(actions []func(*View) bool) bool {
	relocate := false
	readonlyBindingsList := []string{"Delete", "Insert", "Backspace", "Cut", "Play", "Paste", "Move", "Add", "DuplicateLine", "Macro"}
	for _, action := range actions {
		readonlyBindingsResult := false
		funcName := ShortFuncName(action)
		if v.Readonly == true {
			// check for readonly and if true only let key bindings get called if they do not change the contents.
			for _, readonlyBindings := range readonlyBindingsList {
				if strings.Contains(funcName, readonlyBindings) {
					readonlyBindingsResult = true
				}
			}
		}
		if !readonlyBindingsResult {
			// call the key binding
			relocate = action(v) || relocate
		}
	}

	return relocate
}

// SetCursor sets the view's and buffer's cursor
func (v *View) SetCursor(c *Cursor) bool {
	if c == nil {
		return false
	}
	v.Cursor = c
	v.Buf.curCursor = c.Num

	return true
}

// HandleEvent handles an event passed by the main loop
func (v *View) HandleEvent(event tcell.Event) {
	// This bool determines whether the view is relocated at the end of the function
	// By default it's true because most events should cause a relocate
	relocate := true

	// Reset per-event vertical-move tracking. CursorUp/CursorDown set
	// curVertical, so any other event breaks a run of vertical moves and causes
	// the soft-wrap sticky column to be recomputed on the next up/down.
	v.curVertical = false
	defer func() { v.prevVertical = v.curVertical }()

	switch e := event.(type) {
	case *tcell.EventKey:
		// Check first if input is a key binding, if it is we 'eat' the input and don't insert a rune
		isBinding := false
		for key, actions := range v.bindings {

			if e.Key() == key.keyCode {
				if e.Key() == tcell.KeyRune {
					if e.Rune() != key.r {
						continue
					}
				}
				if e.Modifiers() == key.modifiers {
					for _, c := range v.Buf.cursors {
						ok := v.SetCursor(c)
						if !ok {
							break
						}
						relocate = false
						isBinding = true
						relocate = v.ExecuteActions(actions) || relocate
					}
					v.SetCursor(&v.Buf.Cursor)
					v.Buf.MergeCursors()
					break
				}
			}
		}

		if !isBinding && e.Key() == tcell.KeyRune {
			// Check viewtype if readonly don't insert a rune (readonly help and log view etc.)
			if v.Readonly == false {
				for _, c := range v.Buf.cursors {
					v.SetCursor(c)

					// Insert a character
					if v.Cursor.HasSelection() {
						v.Cursor.DeleteSelection()
						v.Cursor.ResetSelection()
					}

					if v.isOverwriteMode {
						next := v.Cursor.Loc
						next.X++
						v.Buf.Replace(v.Cursor.Loc, next, string(e.Rune()))
					} else {
						v.Buf.Insert(v.Cursor.Loc, string(e.Rune()))
					}
				}
				v.SetCursor(&v.Buf.Cursor)
			}
		}
	}

	if relocate {
		v.Relocate()
		// We run relocate again because there's a bug with relocating with softwrap
		// when for example you jump to the bottom of the buffer and it tries to
		// calculate where to put the topline so that the bottom line is at the bottom
		// of the terminal and it runs into problems with visual lines vs real lines.
		// This is (hopefully) a temporary solution
		v.Relocate()
	}
}

func (v *View) mainCursor() bool {
	return v.Buf.curCursor == len(v.Buf.cursors)-1
}

// displayView draws the view to the screen
func (v *View) displayView(screen tcell.Screen) {
	if v.Buf.Settings["softwrap"].(bool) && v.leftCol != 0 {
		v.leftCol = 0
	}

	// We need to know the string length of the largest line number
	// so we can pad appropriately when displaying line numbers
	maxLineNumLength := len(strconv.Itoa(v.Buf.NumLines))

	if v.Buf.Settings["ruler"] == true {
		// + 1 for the little space after the line number
		v.lineNumOffset = maxLineNumLength + 1
	} else {
		v.lineNumOffset = 0
	}

	xOffset := v.x + v.lineNumOffset
	yOffset := v.y

	height := v.height
	width := v.width
	left := v.leftCol
	top := v.Topline

	skipRows := 0
	if v.Buf.Settings["softwrap"].(bool) {
		v.clampTopRow()
		skipRows = v.topRow
	}
	v.cellview.Draw(v.Buf, v.colorscheme, top, height, left, width-v.lineNumOffset, skipRows)

	screenX := v.x
	realLineN := top - 1
	visualLineN := 0
	var line []*Char
	for visualLineN, line = range v.cellview.lines {
		var firstChar *Char
		if len(line) > 0 {
			firstChar = line[0]
		}

		var softwrapped bool
		if firstChar != nil {
			if firstChar.realLoc.Y == realLineN {
				softwrapped = true
			}
			realLineN = firstChar.realLoc.Y
		} else {
			realLineN++
		}

		colorcolumn := int(v.Buf.Settings["colorcolumn"].(float64))
		if colorcolumn != 0 && xOffset+colorcolumn-v.leftCol < v.width {
			style := v.colorscheme.GetColor("color-column")
			fg, _, _ := style.Decompose()
			st := defStyle.Background(fg)
			screen.SetContent(xOffset+colorcolumn-v.leftCol, yOffset+visualLineN, ' ', nil, st)
		}

		screenX = v.x

		lineNumStyle := defStyle
		if v.Buf.Settings["ruler"] == true {
			// Write the line number
			if style, ok := v.colorscheme["line-number"]; ok {
				lineNumStyle = style
			}
			if style, ok := v.colorscheme["current-line-number"]; ok {
				if realLineN == v.Cursor.Y && !v.Cursor.HasSelection() {
					lineNumStyle = style
				}
			}

			lineNum := strconv.Itoa(realLineN + 1)

			// Write the spaces before the line number if necessary
			for i := 0; i < maxLineNumLength-len(lineNum); i++ {
				screen.SetContent(screenX, yOffset+visualLineN, ' ', nil, lineNumStyle)
				screenX++
			}
			// The very first visible row is a wrapped continuation when the top
			// line is scrolled partway in (topRow > 0), so pad instead of
			// printing its number.
			continuation := (softwrapped && visualLineN != 0) || (visualLineN == 0 && v.topRow > 0)
			if continuation {
				// Pad without the line number because it was written on the visual line before
				for range lineNum {
					screen.SetContent(screenX, yOffset+visualLineN, ' ', nil, lineNumStyle)
					screenX++
				}
			} else {
				// Write the actual line number
				for _, ch := range lineNum {
					screen.SetContent(screenX, yOffset+visualLineN, ch, nil, lineNumStyle)
					screenX++
				}
			}

			// Write the extra space
			screen.SetContent(screenX, yOffset+visualLineN, ' ', nil, lineNumStyle)
			screenX++
		}

		var lastChar *Char
		cursorSet := false
		for _, char := range line {
			if char != nil {
				lineStyle := char.style

				colorcolumn := int(v.Buf.Settings["colorcolumn"].(float64))
				if colorcolumn != 0 && char.visualLoc.X == colorcolumn {
					style := v.colorscheme.GetColor("color-column")
					fg, _, _ := style.Decompose()
					lineStyle = lineStyle.Background(fg)
				}

				charLoc := char.realLoc
				for _, c := range v.Buf.cursors {
					v.SetCursor(c)
					if v.Cursor.HasSelection() &&
						(charLoc.GreaterEqual(v.Cursor.CurSelection[0]) && charLoc.LessThan(v.Cursor.CurSelection[1]) ||
							charLoc.LessThan(v.Cursor.CurSelection[0]) && charLoc.GreaterEqual(v.Cursor.CurSelection[1])) {
						// The current character is selected
						lineStyle = defStyle.Reverse(true)

						if style, ok := v.colorscheme["selection"]; ok {
							lineStyle = style
						}
					}
				}
				v.SetCursor(&v.Buf.Cursor)

				if v.Buf.Settings["cursorline"].(bool) &&
					!v.Cursor.HasSelection() && v.Cursor.Y == realLineN {
					style := v.colorscheme.GetColor("cursor-line")
					fg, _, _ := style.Decompose()
					lineStyle = lineStyle.Background(fg)
				}

				screen.SetContent(xOffset+char.visualLoc.X, yOffset+char.visualLoc.Y, char.drawChar, nil, lineStyle)

				for i, c := range v.Buf.cursors {
					v.SetCursor(c)
					if !v.Cursor.HasSelection() &&
						v.Cursor.Y == char.realLoc.Y && v.Cursor.X == char.realLoc.X && (!cursorSet || i != 0) {
						ShowMultiCursor(screen, xOffset+char.visualLoc.X, yOffset+char.visualLoc.Y, i)
						cursorSet = true
					}
				}
				v.SetCursor(&v.Buf.Cursor)

				lastChar = char
			}
		}

		lastX := 0
		var realLoc Loc
		var visualLoc Loc
		var cx, cy int
		if lastChar != nil {
			lastX = xOffset + lastChar.visualLoc.X + lastChar.width
			for i, c := range v.Buf.cursors {
				v.SetCursor(c)
				if !v.Cursor.HasSelection() &&
					v.Cursor.Y == lastChar.realLoc.Y && v.Cursor.X == lastChar.realLoc.X+1 {
					ShowMultiCursor(screen, lastX, yOffset+lastChar.visualLoc.Y, i)
					cx, cy = lastX, yOffset+lastChar.visualLoc.Y
				}
			}
			v.SetCursor(&v.Buf.Cursor)
			realLoc = Loc{lastChar.realLoc.X + 1, realLineN}
			visualLoc = Loc{lastX - xOffset, lastChar.visualLoc.Y}
		} else if len(line) == 0 {
			for i, c := range v.Buf.cursors {
				v.SetCursor(c)
				if !v.Cursor.HasSelection() &&
					v.Cursor.Y == realLineN {
					ShowMultiCursor(screen, xOffset, yOffset+visualLineN, i)
					cx, cy = xOffset, yOffset+visualLineN
				}
			}
			v.SetCursor(&v.Buf.Cursor)
			lastX = xOffset
			realLoc = Loc{0, realLineN}
			visualLoc = Loc{0, visualLineN}
		}

		if v.Cursor.HasSelection() &&
			(realLoc.GreaterEqual(v.Cursor.CurSelection[0]) && realLoc.LessThan(v.Cursor.CurSelection[1]) ||
				realLoc.LessThan(v.Cursor.CurSelection[0]) && realLoc.GreaterEqual(v.Cursor.CurSelection[1])) {
			// The current character is selected
			selectStyle := defStyle.Reverse(true)

			if style, ok := v.colorscheme["selection"]; ok {
				selectStyle = style
			}
			screen.SetContent(xOffset+visualLoc.X, yOffset+visualLoc.Y, ' ', nil, selectStyle)
		}

		if v.Buf.Settings["cursorline"].(bool) &&
			!v.Cursor.HasSelection() && v.Cursor.Y == realLineN {
			for i := lastX; i < xOffset+v.width-v.lineNumOffset; i++ {
				style := v.colorscheme.GetColor("cursor-line")
				fg, _, _ := style.Decompose()
				style = style.Background(fg)
				if !(!v.Cursor.HasSelection() && i == cx && yOffset+visualLineN == cy) {
					screen.SetContent(i, yOffset+visualLineN, ' ', nil, style)
				}
			}
		}
	}
}

// ShowMultiCursor will display a cursor at a location
// If i == 0 then the terminal cursor will be used
// Otherwise a fake cursor will be drawn at the position
func ShowMultiCursor(screen tcell.Screen, x, y, i int) {
	if i == 0 {
		screen.ShowCursor(x, y)
	} else {
		r, _, _, _ := screen.GetContent(x, y)
		screen.SetContent(x, y, r, nil, defStyle.Reverse(true))
	}
}

// Draw renders the view and the cursor
func (v *View) Draw(screen tcell.Screen) {
	v.Box.Draw(screen)
	v.x, v.y, v.width, v.height = v.Box.GetInnerRect()

	// TODO(pdg): just clear from the last line down.
	for y := v.y; y < v.y+v.height; y++ {
		for x := v.x; x < v.x+v.width; x++ {
			screen.SetContent(x, y, ' ', nil, defStyle)
		}
	}

	v.displayView(screen)

	// Don't draw the cursor if it is out of the viewport or if it has a selection
	hideCursor := v.Cursor.HasSelection()
	if v.Buf.Settings["softwrap"].(bool) {
		off := v.cursorOffsetFromTop()
		if off < 0 || off > v.height-1 {
			hideCursor = true
		}
	} else if v.Cursor.Y-v.Topline < 0 || v.Cursor.Y-v.Topline > v.height-1 {
		hideCursor = true
	}
	if hideCursor {
		screen.HideCursor()
	}

	if v.Buf.Settings["scrollbar"].(bool) {
		v.scrollbar.Display(screen)
	}
}
