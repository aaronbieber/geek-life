package main

import (
	"sort"
	"time"
	"unicode"

	"github.com/asdine/storm/v3"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
)

// TaskPane displays tasks of current TaskList or Project
type TaskPane struct {
	*tview.Flex
	list       *tview.List
	tasks      []model.Task
	activeTask *model.Task

	newTask     *tview.InputField
	projectRepo repository.ProjectRepository
	taskRepo    repository.TaskRepository
	hint        *tview.TextView

	// filter selects which tasks the current list displays (done/not done/all).
	// It persists across lists and is re-applied whenever a list is loaded.
	filter taskFilter
	// reloadList re-runs the loader for the currently displayed list (project or
	// dynamic), so a filter change can re-render it. Set on every list load.
	reloadList func()
	// showingSplash is true while the pane displays only the splash/help text
	// (no list loaded). In that state the pane refuses focus so a stray click or
	// "t" can't quietly move focus off the Projects pane.
	showingSplash bool
}

// NewTaskPane initializes and configures a TaskPane
func NewTaskPane(projectRepo repository.ProjectRepository, taskRepo repository.TaskRepository) *TaskPane {
	pane := TaskPane{
		Flex:        tview.NewFlex().SetDirection(tview.FlexRow),
		list:        tview.NewList().ShowSecondaryText(false),
		newTask:     makeLightTextInput("+[New Task]"),
		projectRepo: projectRepo,
		taskRepo:    taskRepo,
		hint:        tview.NewTextView().SetTextColor(tcell.ColorYellow).SetTextAlign(tview.AlignCenter),
	}

	pane.list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)
	pane.list.SetDoneFunc(func() {
		app.SetFocus(projectPane)
	})

	pane.newTask.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			name := pane.newTask.GetText()
			if len(name) < 3 {
				statusBar.showForSeconds("[red::]Task title should be at least 3 character", 5)
				return
			}

			task, err := taskRepo.Create(*projectPane.GetActiveProject(), name, "", "", 0)
			if err != nil {
				statusBar.showForSeconds("[red::]Could not create Task:"+err.Error(), 5)
				return
			}

			pane.tasks = append(pane.tasks, task)
			pane.addTaskToList(len(pane.tasks) - 1)
			pane.RemoveItem(pane.hint) // clear the "No tasks" message now that one exists
			pane.newTask.SetText("")
			statusBar.showForSeconds("[yellow::]Task created. Add another task or press Esc.", 5)
		case tcell.KeyEsc:
			pane.hideNewTaskInput()
		}
	})

	pane.
		AddItem(pane.list, 0, 1, true).
		AddItem(pane.hint, 0, 1, false)

	pane.SetBorder(true)
	pane.updateTitle()
	pane.setHintMessage()
	pane.showingSplash = true // the splash is shown until a list is loaded

	return &pane
}

// MouseHandler refuses mouse focus while only the splash is shown, so clicking
// the empty Tasks area doesn't silently take focus from the Projects pane.
func (pane *TaskPane) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if pane.showingSplash {
			return false, nil
		}
		return pane.Flex.MouseHandler()(action, event, setFocus)
	}
}

// ClearList removes all items from TaskPane
func (pane *TaskPane) ClearList() {
	pane.list.Clear()
	pane.tasks = nil
	pane.activeTask = nil

	pane.RemoveItem(pane.newTask)
}

// SetList Sets a list of tasks to be displayed, honoring the active filter.
func (pane *TaskPane) SetList(tasks []model.Task) {
	pane.ClearList()
	pane.tasks = filterTasks(tasks, pane.filter)

	for i := range pane.tasks {
		pane.addTaskToList(i)
	}
}

// applyFilter records the chosen filter and re-renders the current list.
func (pane *TaskPane) applyFilter(f taskFilter) {
	pane.filter = f
	pane.updateTitle()
	if pane.reloadList != nil {
		pane.reloadList()
	}
}

// updateTitle sets the pane title, appending the active filter's name in
// parentheses (e.g. "Tasks (Not done)"); no suffix when no filter is active.
func (pane *TaskPane) updateTitle() {
	title := "[::u]T[::-]asks"
	if name := pane.filter.name(); name != "" {
		title += " (" + name + ")"
	}
	pane.SetTitle(title)
}

func (pane *TaskPane) addTaskToList(i int) *tview.List {
	return pane.list.AddItem(makeTaskListingTitle(pane.tasks[i]), "", 0, func(taskidx int) func() {
		return func() { taskPane.ActivateTask(taskidx) }
	}(i))
}

// MoveTask moves the currently selected task by the given offset (-1 = up,
// +1 = down) and persists the new order. Reordering is only meaningful within
// a single project's list, so it is a no-op for the dynamic (date-based) views.
func (pane *TaskPane) MoveTask(offset int) {
	if projectPane.GetActiveProject() == nil {
		statusBar.showForSeconds("[yellow::]Tasks can only be reordered within a project", 5)
		return
	}

	from := pane.list.GetCurrentItem()
	to := from + offset
	if from < 0 || to < 0 || to >= len(pane.tasks) {
		return
	}

	pane.tasks[from], pane.tasks[to] = pane.tasks[to], pane.tasks[from]
	pane.persistTaskOrder()

	pane.list.SetItemText(from, makeTaskListingTitle(pane.tasks[from]), "")
	pane.list.SetItemText(to, makeTaskListingTitle(pane.tasks[to]), "")
	pane.list.SetCurrentItem(to)
}

// persistTaskOrder rewrites the Rank of any task whose position in the list has
// changed, so the current in-memory order survives a reload.
func (pane *TaskPane) persistTaskOrder() {
	for i := range pane.tasks {
		rank := int64(i)
		if pane.tasks[i].Rank == rank {
			continue
		}

		pane.tasks[i].Rank = rank
		if err := pane.taskRepo.UpdateField(&pane.tasks[i], "Rank", rank); err != nil {
			statusBar.showForSeconds("[red::]Could not save task order: "+err.Error(), 5)
		}
	}
}

func (pane *TaskPane) handleShortcuts(event *tcell.EventKey) *tcell.EventKey {
	// Esc and Left move focus back to the projects pane. Handled here (not via
	// the list's done func) so it works even when the pane was focused by a
	// mouse click rather than keyboard navigation.
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyLeft:
		app.SetFocus(projectPane)
		return nil
	case tcell.KeyRight:
		// Mimic Enter: open the highlighted task.
		return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	}

	// Shift+J / Shift+K reorder the selected task; = / + raise and - lowers the
	// priority. Checked as raw runes before the case-insensitive switch below.
	switch event.Rune() {
	case '=', '+':
		pane.changeSelectedTaskPriority(-1)
		return nil
	case '-':
		pane.changeSelectedTaskPriority(1)
		return nil
	case 'J':
		pane.MoveTask(1)
		return nil
	case 'K':
		pane.MoveTask(-1)
		return nil
	case 'D':
		pane.startDeleteSelectedTask()
		return nil
	}

	switch unicode.ToLower(event.Rune()) {
	case 'j':
		pane.list.SetCurrentItem(pane.list.GetCurrentItem() + 1)
		return nil
	case 'k':
		pane.list.SetCurrentItem(pane.list.GetCurrentItem() - 1)
		return nil
	case 'h':
		app.SetFocus(projectPane)
		return nil
	case 'l':
		// vim-style right: open the highlighted task (mimic Enter).
		return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	case 'n':
		pane.showNewTaskInput()
		return nil
	case 'f':
		filterChordActive = true
		return nil
	case 'd':
		pane.toggleSelectedTaskDone()
		return nil
	}

	return event
}

// LoadProjectTasks loads tasks of a project in taskPane
func (pane *TaskPane) LoadProjectTasks(project model.Project) {
	pane.reloadList = func() { pane.LoadProjectTasks(project) }
	pane.showingSplash = false

	tasks, err := taskRepo.GetAllByProject(project)
	if err != nil && err != storm.ErrNotFound {
		statusBar.showForSeconds("[red::]Error: "+err.Error(), 5)
		return
	}

	sortTasks(tasks)
	pane.SetList(tasks)

	if len(pane.tasks) == 0 {
		pane.showListMessage("No tasks")
	} else {
		pane.RemoveItem(pane.hint)
	}
}

// showNewTaskInput reveals the new-task input at the bottom of the pane and
// focuses it. Tasks can only be added within a project, so it is a no-op for the
// dynamic (date-based) lists, which have no project to add to.
func (pane *TaskPane) showNewTaskInput() {
	if projectPane.GetActiveProject() == nil {
		return
	}
	pane.newTask.SetText("")
	pane.RemoveItem(pane.newTask) // avoid duplicating if already shown
	pane.AddItem(pane.newTask, 1, 0, false)
	app.SetFocus(pane.newTask)
}

// hideNewTaskInput removes the new-task input and returns focus to the task list.
func (pane *TaskPane) hideNewTaskInput() {
	pane.RemoveItem(pane.newTask)
	app.SetFocus(pane)
}

// LoadDynamicList loads tasks based on logic key
func (pane *TaskPane) LoadDynamicList(logic string) {
	pane.reloadList = func() { pane.LoadDynamicList(logic) }
	pane.showingSplash = false

	var tasks []model.Task
	var err error

	today := toDate(time.Now())
	zeroTime := time.Time{}

	switch logic {
	case "all":
		tasks, err = pane.taskRepo.GetAll()

	case "today":
		tasks, err = pane.taskRepo.GetAllByDateRange(zeroTime, today)

	case "tomorrow":
		tomorrow := today.AddDate(0, 0, 1)
		tasks, err = pane.taskRepo.GetAllByDate(tomorrow)

	case "upcoming":
		week := today.Add(7 * 24 * time.Hour)
		tasks, err = pane.taskRepo.GetAllByDateRange(today, week)

	case "unscheduled":
		tasks, err = pane.taskRepo.GetAllByDate(zeroTime)
	}

	projectPane.activeProject = nil
	taskPane.ClearList()
	removeThirdCol()

	// storm reports ErrNotFound for an empty result, which is not a real error
	// here - it just means the list has no tasks.
	if err != nil && err != storm.ErrNotFound {
		statusBar.showForSeconds("[red]Error: "+err.Error(), 5)
		return
	}

	sortTasks(tasks)

	pane.SetList(tasks)
	app.SetFocus(taskPane)

	if len(tasks) == 0 {
		pane.showListMessage("No tasks")
	} else {
		pane.RemoveItem(pane.hint)
	}
}

// showListMessage displays a centered message in the task list area, in the same
// place as the initial splash/hint text. Used when a list has no tasks to show.
func (pane *TaskPane) showListMessage(message string) {
	pane.hint.SetText(message)
	pane.RemoveItem(pane.hint) // avoid duplicating if already present
	pane.AddItem(pane.hint, 0, 1, false)
}

// ShowSplash clears the pane and displays the initial splash/help text. Used when
// no task list is displayed, e.g. after deleting a project.
func (pane *TaskPane) ShowSplash() {
	pane.ClearList()
	pane.setHintMessage()
	pane.RemoveItem(pane.hint) // avoid duplicating if already present
	pane.AddItem(pane.hint, 0, 1, false)
	pane.showingSplash = true
}

// sortTasks orders any task list: by priority first (A→B→C, unset counting as
// B), then by due date (most overdue first, undated last), then grouped by
// project, and finally by each task's rank (its user-defined order) then ID.
// Rank is the lowest-precedence key, so it can never override priority or dates.
func sortTasks(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]

		// 1. Priority (A highest).
		if pa, pb := effectivePriority(a), effectivePriority(b); pa != pb {
			return pa < pb
		}
		// 2. Due date: dated before undated, earliest first.
		aDated, bDated := a.DueDate != 0, b.DueDate != 0
		if aDated != bDated {
			return aDated
		}
		if aDated && a.DueDate != b.DueDate {
			return a.DueDate < b.DueDate
		}
		// 3. Group by project.
		if a.ProjectID != b.ProjectID {
			return a.ProjectID < b.ProjectID
		}
		// 4. Natural per-project order (rank), then ID.
		if a.Rank != b.Rank {
			return a.Rank < b.Rank
		}
		return a.ID < b.ID
	})
}

// ActivateTask marks a task as currently active and loads in TaskDetailPane
func (pane *TaskPane) ActivateTask(idx int) {
	removeThirdCol()
	pane.activeTask = &pane.tasks[idx]
	taskDetailPane.SetTask(pane.activeTask)

	contents.AddItem(taskDetailPane, 0, 3, false)
	thirdCol = taskDetailPane
}

// startDeleteSelectedTask shows a status-bar confirmation for deleting the task
// under the cursor.
func (pane *TaskPane) startDeleteSelectedTask() {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}

	startDeleteConfirm(pane.tasks[idx].Title,
		pane.deleteSelectedTask,
		func() { app.SetFocus(taskPane) })
}

// deleteSelectedTask removes the task under the cursor and reloads the list,
// keeping the selection near the deleted position.
func (pane *TaskPane) deleteSelectedTask() {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}

	if err := pane.taskRepo.Delete(&pane.tasks[idx]); err != nil {
		statusBar.showForSeconds("[red::]Could not delete task: "+err.Error(), 5)
		return
	}

	if pane.reloadList != nil {
		pane.reloadList()
	}
	if n := len(pane.tasks); n > 0 {
		if idx >= n {
			idx = n - 1
		}
		pane.list.SetCurrentItem(idx)
	}
	app.SetFocus(taskPane)
}

// ReloadCurrentTask Loads the current task - in Task details and listing
func (pane *TaskPane) ReloadCurrentTask() {
	pane.list.SetItemText(pane.list.GetCurrentItem(), makeTaskListingTitle(*pane.activeTask), "")
	taskDetailPane.SetTask(pane.activeTask)
}

// RefreshAfterEdit reloads the task list after returning from the detail pane so
// that changes made there (priority, due date, done state, colors) are reflected
// and the list is re-sorted. The selection follows the edited task to its new
// position by ID, whether the list is a project or a dynamic list.
func (pane *TaskPane) RefreshAfterEdit() {
	editedID := int64(-1)
	if pane.activeTask != nil {
		editedID = pane.activeTask.ID
	}
	prev := pane.list.GetCurrentItem()

	if pane.reloadList != nil {
		pane.reloadList()
	}

	pane.selectTaskByID(editedID, prev)
}

// toggleSelectedTaskDone flips the done state of the task under the cursor and
// reloads the list so any active filter is respected (e.g. an item marked done
// disappears from a "not done" filtered list). The selection follows the task
// if it remains, otherwise it stays near its former position.
func (pane *TaskPane) toggleSelectedTaskDone() {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}

	task := &pane.tasks[idx]
	done := !task.Completed
	if err := pane.taskRepo.UpdateField(task, "Completed", done); err != nil {
		statusBar.showForSeconds("[red::]Could not update task: "+err.Error(), 5)
		return
	}
	editedID := task.ID

	if pane.reloadList != nil {
		pane.reloadList()
	}

	pane.selectTaskByID(editedID, idx)
}

// changeSelectedTaskPriority adjusts the priority of the task under the cursor.
// delta of -1 raises it (toward A), +1 lowers it (toward C); it is clamped to
// the A..C range. The list is reloaded so it re-sorts by the new priority, and
// the selection follows the task to its new position.
func (pane *TaskPane) changeSelectedTaskPriority(delta int) {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}

	if !setTaskPriority(pane.taskRepo, &pane.tasks[idx], delta) {
		return
	}
	editedID := pane.tasks[idx].ID

	if pane.reloadList != nil {
		pane.reloadList()
	}
	pane.selectTaskByID(editedID, idx)
}

// selectTaskByID moves the selection to the task with the given ID. If it is no
// longer in the list (e.g. filtered out), the selection falls back to the item
// near fallbackIdx.
func (pane *TaskPane) selectTaskByID(id int64, fallbackIdx int) {
	for i := range pane.tasks {
		if pane.tasks[i].ID == id {
			pane.list.SetCurrentItem(i)
			return
		}
	}

	if n := len(pane.tasks); n > 0 {
		if fallbackIdx >= n {
			fallbackIdx = n - 1
		}
		pane.list.SetCurrentItem(fallbackIdx)
	}
}

func (pane TaskPane) setHintMessage() {
	pane.hint.SetText("Select a list on the left and press enter to view tasks.\n" +
		"Underlined letters indicate available keys.\n" +
		"Additional key hints are shown at the bottom of the screen.\n\n" +
		"Press ? for more help.")
}
