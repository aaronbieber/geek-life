package main

import (
	"github.com/rivo/tview"
)

// Help view state, remembered so Help can restore the previous layout/focus.
var (
	helpShowing     bool
	helpReturnFocus tview.Primitive
	helpSavedThird  tview.Primitive
)

// NewHelpPane builds the scrollable, left/top-aligned Help view.
func NewHelpPane() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true)

	tv.SetText(helpText)
	tv.SetBorder(true).SetTitle("Help")

	return tv
}

// showHelp hides the Tasks area (and any open detail pane) and shows Help.
func showHelp() {
	if helpShowing {
		return
	}
	helpShowing = true
	helpReturnFocus = app.GetFocus()
	helpSavedThird = thirdCol

	removeThirdCol()
	contents.RemoveItem(taskPane)
	contents.AddItem(helpPane, 0, 2, true)
	helpPane.ScrollToBeginning()
	app.SetFocus(helpPane)
}

// hideHelp restores the layout, detail pane, and focus present when Help opened.
func hideHelp() {
	if !helpShowing {
		return
	}
	helpShowing = false

	contents.RemoveItem(helpPane)
	contents.AddItem(taskPane, 0, 2, false)

	if helpSavedThird == taskDetailPane {
		contents.AddItem(taskDetailPane, 0, 3, false)
		thirdCol = taskDetailPane
	}

	if helpReturnFocus != nil {
		app.SetFocus(helpReturnFocus)
	}
}

// helpText is shown in the Help pane. It uses tview dynamic-color tags; keys are
// highlighted and shortcut letters are underlined to mirror the UI.
const helpText = `[yellow::b]Geek-life — Help[-::-]

Scroll this page with ↑/↓, j/k, PgUp/PgDn, or g/G.  Press [yellow]←[-] or [yellow]Esc[-] to close help.

A keyboard-driven task manager. This page explains the philosophy, the
layout, and every shortcut. Read top to bottom if you're new.

[yellow::b]How geek-life works[-::-]
Geek-life is a lightweight task manager for tracking simple, related lists
of tasks — not a heavyweight planner.
  • A [::b]project[::-] is a container for a set of related tasks.
  • A task is either done or not done; toggle it as you work.
  • A task list can be filtered to show only done or only not-done tasks.
  • When every task in a project is done, you're finished with it — delete
    the whole project rather than clearing tasks one by one.

[yellow::b]The layout[-::-]
The screen has up to three columns, left to right:
  • [::b]Projects[::-] (left) — your dynamic lists and projects.
  • [::b]Tasks[::-] (middle) — the tasks in the selected list.
  • [::b]Task Detail[::-] (right) — the selected task's note, due date and priority.
One column is focused at a time; the focused column has a brighter border.

[yellow::b]Finding the shortcuts[-::-]
You rarely need to memorize keys — the screen shows them two ways:
  • [::b]Underlined letters[::-] — an underlined letter in a title or button is its
    shortcut key. "[::u]P[::-]rojects" means press [yellow]p[-]; "[::u]e[::-]dit" means press [yellow]e[-].
  • [::b]The bottom row[::-] — the very bottom line lists the other available keys for
    whatever is focused, written as "key: action". It changes as you move
    around, and it hides keys during typing.

[yellow::b]Getting around[-::-]
  • Move up / down a list:   [yellow]j[-] [yellow]k[-]   or   [yellow]↑[-] [yellow]↓[-]
  • Go right / open:         [yellow]→[-]   or   [yellow]Enter[-]
       From Projects, opens the selected list into the Tasks column.
       From Tasks, opens the selected task's detail.
  • Go left / back:          [yellow]←[-]   or   [yellow]Esc[-]
       From Task Detail, closes it back to the task list.
       From Tasks, returns to Projects (the [yellow]h[-] key does this too).
  • Jump straight to a column: [yellow]p[-] for Projects, [yellow]t[-] for Tasks.

[yellow::b]Projects and dynamic lists[-::-]
The Projects column has two groups:
  • [::b]Dynamic Lists[::-] — automatic views you don't edit directly:
       [::b]All[::-] every task, [::b]Today[::-] due today or overdue, [::b]Tomorrow[::-],
       [::b]Upcoming[::-] the next 7 days, and [::b]Unscheduled[::-] tasks with no due date.
  • [::b]Projects[::-] — lists you create; every task lives in a project.
Select any item and press [yellow]→[-] / [yellow]Enter[-] to load its tasks. In dynamic lists a
task is shown with its project name as a prefix.
  • New project:                 [yellow]n[-]
  • Delete the selected project:  [yellow]d[-], then confirm [yellow]y[-] (yes) or [yellow]n[-] (no)
  • Jump to the first project:    [yellow]Shift+J[-]
  • Jump to the Dynamic Lists:    [yellow]Shift+K[-]

[yellow::b]Working with tasks[-::-]
With a list loaded in the Tasks column:
  • New task (in a project):     [yellow]n[-]
  • Open a task:                 [yellow]→[-] / [yellow]Enter[-]
  • Toggle done / not done:      [yellow]d[-]
  • Delete a task:               [yellow]Shift+D[-], then confirm [yellow]y[-] / [yellow]n[-]
  • Move the selected task up/down (projects only): [yellow]Shift+K[-] / [yellow]Shift+J[-]
Completed tasks are shown in green; tasks due today are orange and overdue
tasks are red.

[yellow::b]Priorities[-::-]
Tasks can be [green][A[][-] (high) or [orange][C[][-] (low). The default is B, which is
implicit and shown without a cookie — so most tasks have no bracket. Raise a
task to A or lower it to C:
  • Raise priority (toward A):  [yellow]Shift+↑[-]
  • Lower priority (toward C):  [yellow]Shift+↓[-]
These work in both the Tasks list and the task detail. Lists are sorted by
priority first, then due date, then your manual order.

[yellow::b]Filtering[-::-]
Press [yellow]f[-] to start a filter, then choose:
  [yellow]d[-] Done      [yellow]u[-] Not done      [yellow]a[-] All (no filter)
The active filter is shown in the Tasks title, e.g. "Tasks (Not done)", and
applies to both projects and dynamic lists.

[yellow::b]The task detail[-::-]
Open a task to view and change it:
  • Edit the note here:            [yellow]e[-]
  • Edit the note in your editor:  [yellow]v[-]   (uses $EDITOR)
  • Rename the task:               [yellow]r[-]
  • Mark complete / not complete:  [yellow]Space[-]
  • Scroll the note:               [yellow]↑[-] [yellow]↓[-]
  • Copy the task to the clipboard: [yellow]x[-]
  • Close:                         [yellow]←[-] / [yellow]Esc[-]

[yellow::b]Due dates[-::-] (in the task detail)
  • Edit the date field: [yellow]d[-], type a date as YYYY-MM-DD, then [yellow]Enter[-]
  • Set to today:        [yellow]o[-]
  • Next / previous day: [yellow]+[-] / [yellow]-[-]
  • Clear the due date:  [yellow]u[-]

[yellow::b]Editing a note[-::-]
While the note editor is active:
  • Move by word:          [yellow]Ctrl+←[-] / [yellow]Ctrl+→[-]
  • Delete the word left:   [yellow]Ctrl+W[-] or [yellow]Ctrl+Backspace[-]
  • Save and leave editing: [yellow]Esc[-]
Notes are stored as plain paragraphs and wrap to the window, so they reflow
when the window resizes; long notes scroll as you move the cursor.

[yellow::b]Anywhere[-::-]
  • This help:  [yellow]?[-]
  • Quit:       [yellow]Ctrl+C[-]`
