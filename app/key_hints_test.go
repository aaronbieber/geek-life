package main

import (
	"testing"

	"github.com/ajaxray/geek-life/model"
)

func TestFormatKeyHints(t *testing.T) {
	got := formatKeyHints([]keyHint{
		{"j,k", "Navigate list"},
		{"n", "New project"},
	})
	// Left-padded, "<key>: <op>" with a colorized key, separated by two spaces.
	want := " [yellow]j,k[-]: Navigate list  [yellow]n[-]: New project"
	if got != want {
		t.Fatalf("formatKeyHints:\n got  %q\n want %q", got, want)
	}
}

func hintKeys(hints []keyHint) []string {
	keys := make([]string, len(hints))
	for i, h := range hints {
		keys[i] = h.key
	}
	return keys
}

func eqStrs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestTaskPaneKeyHintsInProject(t *testing.T) {
	// Stub the global project pane with an active project.
	projectPane = &ProjectPane{activeProject: &model.Project{}}
	defer func() { projectPane = nil }()

	pane := &TaskPane{}
	// With a project active: reorder and "new task" are available.
	eqStrs(t, hintKeys(pane.keyHints()),
		[]string{"j,k", "J/K", "enter", "d", "=,-", "n", "f", "esc"})
}

func TestTaskPaneKeyHintsDynamicList(t *testing.T) {
	// No active project (a dynamic list is showing).
	projectPane = &ProjectPane{activeProject: nil}
	defer func() { projectPane = nil }()

	pane := &TaskPane{}
	// No project: no reorder, no "new task".
	eqStrs(t, hintKeys(pane.keyHints()),
		[]string{"j,k", "enter", "d", "=,-", "f", "esc"})
}
