package main

import (
	"unicode"

	"github.com/gdamore/tcell/v2"

	"github.com/ajaxray/geek-life/model"
)

// taskFilter selects which tasks a list displays.
type taskFilter int

const (
	filterAll taskFilter = iota // no filter
	filterDone
	filterNotDone
)

// name returns the display name of an active filter, or "" for no filter.
func (f taskFilter) name() string {
	switch f {
	case filterDone:
		return "Done"
	case filterNotDone:
		return "Not done"
	default:
		return ""
	}
}

// filterChordActive is true while the filter key-chord sub-menu is showing —
// i.e. after "f" is pressed and before an option (or Esc) is chosen.
var filterChordActive bool

// filterOptions are the selectable options shown after the filter chord entry.
var filterOptions = []keyHint{
	{"d", "Done"},
	{"u", "Not done"},
	{"a", "All (no filter)"},
}

// filterTasks returns the subset of tasks matching the given filter, preserving
// order.
func filterTasks(tasks []model.Task, f taskFilter) []model.Task {
	if f == filterAll {
		return tasks
	}

	filtered := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		if (f == filterDone) == t.Completed {
			filtered = append(filtered, t)
		}
	}

	return filtered
}

// handleFilterChord resolves a keypress while the filter chord is active. Any
// key ends the chord; d/u/a apply the corresponding filter, anything else (Esc
// included) simply cancels.
func handleFilterChord(event *tcell.EventKey) *tcell.EventKey {
	filterChordActive = false

	switch unicode.ToLower(event.Rune()) {
	case 'd':
		taskPane.applyFilter(filterDone)
	case 'u':
		taskPane.applyFilter(filterNotDone)
	case 'a':
		taskPane.applyFilter(filterAll)
	}

	return nil
}
