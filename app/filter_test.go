package main

import (
	"testing"

	"github.com/ajaxray/geek-life/model"
)

func TestFilterTasks(t *testing.T) {
	tasks := []model.Task{
		{ID: 1, Completed: false},
		{ID: 2, Completed: true},
		{ID: 3, Completed: false},
		{ID: 4, Completed: true},
	}

	ids := func(ts []model.Task) []int64 {
		out := make([]int64, len(ts))
		for i, t := range ts {
			out[i] = t.ID
		}
		return out
	}
	eq := func(name string, got, want []int64) {
		if len(got) != len(want) {
			t.Fatalf("%s: got %v want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v want %v", name, got, want)
			}
		}
	}

	eq("all", ids(filterTasks(tasks, filterAll)), []int64{1, 2, 3, 4})
	eq("done", ids(filterTasks(tasks, filterDone)), []int64{2, 4})
	eq("notdone", ids(filterTasks(tasks, filterNotDone)), []int64{1, 3})
}
