package main

import (
	"fmt"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
)

// Task priorities follow org-mode: A (highest) through C (lowest), with B the
// default for tasks that carry no explicit priority. Stored as 1=A, 2=B, 3=C;
// 0 means unset and is treated as B.
const (
	priorityHighest = 1 // A
	priorityDefault = 2 // B
	priorityLowest  = 3 // C
)

// effectivePriority returns the task's priority, treating unset (0) as B so a
// task can be promoted above or demoted below its natural default.
func effectivePriority(t model.Task) int64 {
	if t.Priority == 0 {
		return priorityDefault
	}
	return t.Priority
}

// priorityColor returns the display color for a task's priority cookie, or ""
// for the default (B), which has no cookie.
func priorityColor(t model.Task) string {
	switch effectivePriority(t) {
	case priorityHighest:
		return "green"
	case priorityLowest:
		return "orange"
	default:
		return ""
	}
}

// setTaskPriority raises (delta -1) or lowers (delta +1) the task's priority,
// clamped to A..C, and persists it. It returns true if the priority changed.
func setTaskPriority(repo repository.TaskRepository, t *model.Task, delta int) bool {
	next := effectivePriority(*t) + int64(delta)
	if next < priorityHighest {
		next = priorityHighest
	}
	if next > priorityLowest {
		next = priorityLowest
	}
	if next == t.Priority {
		return false
	}

	if err := repo.UpdateField(t, "Priority", next); err != nil {
		statusBar.showForSeconds("[red::]Could not update priority: "+err.Error(), 5)
		return false
	}
	t.Priority = next
	return true
}

// priorityCookie returns the colorized bracketed priority for display — "[A]" in
// green or "[C]" in orange. It returns "" for the default (B), which is implicit
// and shown without a cookie. Brackets are escaped for tview's dynamic-color
// parser (e.g. "[A[]" renders as the literal "[A]").
func priorityCookie(t model.Task) string {
	color := priorityColor(t)
	if color == "" {
		return ""
	}

	letter := "A"
	if effectivePriority(t) == priorityLowest {
		letter = "C"
	}
	return fmt.Sprintf("[%s][%s[]", color, letter)
}
