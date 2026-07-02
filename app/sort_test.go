package main

import (
	"testing"

	"github.com/ajaxray/geek-life/model"
)

func TestSortAllTasks(t *testing.T) {
	// day1 is earlier (more overdue) than day2; 0 means no due date.
	const day1, day2 int64 = 100, 200

	tasks := []model.Task{
		{ID: 1, ProjectID: 1, DueDate: day2, Rank: 0}, // A
		{ID: 2, ProjectID: 1, DueDate: day1, Rank: 1}, // B
		{ID: 3, ProjectID: 2, DueDate: day1, Rank: 0}, // C
		{ID: 4, ProjectID: 1, DueDate: 0, Rank: 0},    // D (undated)
		{ID: 5, ProjectID: 1, DueDate: 0, Rank: 1},    // E (undated, after D)
		{ID: 6, ProjectID: 2, DueDate: day1, Rank: 1}, // F (after C in project 2)
	}

	sortAllTasks(tasks)

	got := make([]int64, len(tasks))
	for i := range tasks {
		got[i] = tasks[i].ID
	}

	// Expected:
	//   day1 bucket, grouped by project, natural order within project:
	//     project 1: B(2); project 2: C(3), F(6)  -> 2, 3, 6
	//   day2 bucket:
	//     project 1: A(1)                          -> 1
	//   undated bucket (last), project 1 natural order:
	//     D(4), E(5)                               -> 4, 5
	want := []int64{2, 3, 6, 1, 4, 5}

	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch:\n got  %v\n want %v", got, want)
		}
	}
}

func TestSortAllTasksUndatedLast(t *testing.T) {
	// An undated task must sort after any dated task, even one far in the future.
	tasks := []model.Task{
		{ID: 1, ProjectID: 1, DueDate: 0},
		{ID: 2, ProjectID: 1, DueDate: 9999999999},
	}
	sortAllTasks(tasks)
	if tasks[0].ID != 2 || tasks[1].ID != 1 {
		t.Fatalf("undated task not sorted last: got ids %d,%d", tasks[0].ID, tasks[1].ID)
	}
}
