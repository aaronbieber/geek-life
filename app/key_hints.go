package main

import (
	"fmt"
	"strings"
)

// keyHint is a single "<key>: <operation>" entry shown in the status bar.
//
// The status bar only carries keys that have no on-screen affordance. Keys that
// the UI already advertises with an underlined letter (button labels, pane
// titles) or a dedicated hint (e.g. "<space> to toggle") are intentionally left
// out, so this row complements — rather than duplicates — those hints.
type keyHint struct {
	key string
	op  string
}

// quitHint is available in every context (tview stops the app on Ctrl+C).
var quitHint = keyHint{"ctrl+c", "Quit"}

// renderHints joins hints as "<key>: <operation>" (key colorized), separated by
// two spaces.
func renderHints(hints []keyHint) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, fmt.Sprintf("[yellow]%s[-]: %s", h.key, h.op))
	}

	return strings.Join(parts, "  ")
}

// formatKeyHints renders hints left-aligned for the status bar.
func formatKeyHints(hints []keyHint) string {
	return " " + renderHints(hints)
}

// keyHintText is the full status-bar text for the current state: the filter
// chord sub-menu when it is active, otherwise the focused context's hints.
func keyHintText() string {
	if deleteConfirmActive {
		return " Deleting " + deleteConfirmName + ", are you sure? " + renderHints(deleteConfirmOptions)
	}
	if filterChordActive {
		return " [::b]Filter:[::-] " + renderHints(filterOptions)
	}

	return formatKeyHints(currentKeyHints())
}

// currentKeyHints resolves the hints for the currently focused context.
func currentKeyHints() []keyHint {
	return append(contextKeyHints(), quitHint)
}

// contextKeyHints returns the context-specific hints (excluding the always-on
// quit hint) based on what currently has focus.
//
// Focus is probed with each primitive's HasFocus() rather than app.GetFocus():
// this runs inside the before-draw handler while the Application write-lock is
// held, and app.GetFocus() would try to take that same lock and deadlock.
// The specific input fields are checked before their containing panes, since a
// pane reports HasFocus() when any of its children is focused.
func contextKeyHints() []keyHint {
	if helpShowing {
		return []keyHint{{"esc", "Close help"}}
	}

	switch {
	// Text-entry contexts: navigation and "new" are unavailable while typing, so
	// only the field's own actions apply.
	case projectPane.newProject.HasFocus():
		return []keyHint{{"enter", "Create project"}, {"esc", "Cancel"}}
	case taskPane.newTask.HasFocus():
		return []keyHint{{"enter", "Create task"}, {"esc", "Cancel"}}
	case taskDetailPane.taskDate.HasFocus():
		return []keyHint{{"enter", "Set date"}, {"esc", "Cancel"}}
	case taskDetailPane.header.renameText.HasFocus():
		return []keyHint{{"enter", "Rename task"}, {"esc", "Cancel"}}
	case taskDetailPane.taskDetailView.HasFocus():
		// The note editor shows "Esc to save changes" inline while editing, so
		// there is nothing left for the bottom row but the global quit.
		return nil

	case projectPane.HasFocus():
		return projectPane.keyHints()
	case taskPane.HasFocus():
		return taskPane.keyHints()
	case taskDetailPane.HasFocus():
		return taskDetailPane.keyHints()
	}

	return nil
}

// keyHints returns the context hints when the Projects pane list is focused.
// The p/t pane-switch keys are advertised by the underlined letters in the pane
// titles, so they are not repeated here.
func (pane *ProjectPane) keyHints() []keyHint {
	return []keyHint{
		{"j,k", "Up/down"},
		{"enter", "Open"},
		{"n", "New project"},
		{"d", "Delete project"},
	}
}

// keyHints returns the context hints when the Tasks pane list is focused. "New
// task" and task reordering are only meaningful inside a real project (dynamic
// lists have no project to add to and cannot be reordered).
func (pane *TaskPane) keyHints() []keyHint {
	hints := []keyHint{{"j,k", "Up/down"}}

	inProject := projectPane.GetActiveProject() != nil
	if inProject {
		hints = append(hints, keyHint{"J/K", "Move task"})
	}
	hints = append(hints, keyHint{"enter", "Open"})
	hints = append(hints, keyHint{"d", "Toggle done"})
	if inProject {
		hints = append(hints, keyHint{"n", "New task"})
	}
	hints = append(hints, keyHint{"f", "Filter"})
	hints = append(hints, keyHint{"esc", "Back"})

	return hints
}

// keyHints returns the context hints when the Task Detail pane is focused. The
// edit/rename/export/date/day actions are all shown on-screen via underlined
// buttons and the toggle hint, so only the un-advertised keys appear here.
func (pane *TaskDetailPane) keyHints() []keyHint {
	return []keyHint{
		{"↑,↓", "Scroll note"},
		{"esc", "Back"},
	}
}
