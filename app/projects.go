package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
)

// ProjectPane Displays projects and dynamic lists
type ProjectPane struct {
	*tview.Flex
	projects            []model.Project
	list                *tview.List
	newProject          *tview.InputField
	repo                repository.ProjectRepository
	activeProject       *model.Project
	projectListStarting int // The index in list where project names starts
	dynamicListStarting int // The index in list where dynamic lists start
}

// NewProjectPane initializes
func NewProjectPane(repo repository.ProjectRepository) *ProjectPane {
	pane := ProjectPane{
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
		list:       tview.NewList().ShowSecondaryText(false),
		newProject: makeLightTextInput("+[New Project]"),
		repo:       repo,
	}

	pane.newProject.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			pane.addNewProject()
		case tcell.KeyEsc:
			pane.hideNewProjectInput()
		}
	})

	// The new-project input is hidden until the user presses "n".
	pane.AddItem(pane.list, 0, 1, true)

	pane.SetBorder(true).SetTitle("[::u]P[::-]rojects")
	pane.loadListItems(false)

	return &pane
}

// showNewProjectInput reveals the new-project input at the bottom of the pane
// and focuses it.
func (pane *ProjectPane) showNewProjectInput() {
	pane.newProject.SetText("")
	pane.RemoveItem(pane.newProject) // avoid duplicating if already shown
	pane.AddItem(pane.newProject, 1, 0, false)
	app.SetFocus(pane.newProject)
}

// hideNewProjectInput removes the new-project input and returns focus to the
// project list.
func (pane *ProjectPane) hideNewProjectInput() {
	pane.RemoveItem(pane.newProject)
	app.SetFocus(projectPane)
}

func (pane *ProjectPane) addNewProject() {
	name := pane.newProject.GetText()
	if len(name) < 3 {
		statusBar.showForSeconds("[red::]Project name should be at least 3 character", 5)
		return
	}

	project, err := pane.repo.Create(name, "")
	if err != nil {
		statusBar.showForSeconds("[red::]Failed to create Project:"+err.Error(), 5)
	} else {
		statusBar.showForSeconds(fmt.Sprintf("[yellow::]Project %s created. Press n to start adding new tasks.", name), 10)
		pane.projects = append(pane.projects, project)
		pane.newProject.SetText("")
		// Hide the input; activating the new project moves focus to the task pane.
		pane.RemoveItem(pane.newProject)
		pane.addProjectToList(len(pane.projects)-1, true)
	}
}

func (pane *ProjectPane) addDynamicLists() {
	pane.addSection("Dynamic Lists")
	pane.dynamicListStarting = pane.list.GetItemCount()
	pane.list.AddItem("- All", "", 0, func() { taskPane.LoadDynamicList("all") })
	pane.list.AddItem("- Today", "", 0, func() { taskPane.LoadDynamicList("today") })
	pane.list.AddItem("- Tomorrow", "", 0, func() { taskPane.LoadDynamicList("tomorrow") })
	pane.list.AddItem("- Upcoming", "", 0, func() { taskPane.LoadDynamicList("upcoming") })
	pane.list.AddItem("- Unscheduled", "", 0, func() { taskPane.LoadDynamicList("unscheduled") })
}

func (pane *ProjectPane) addProjectList() {
	pane.addSection("Projects")
	pane.projectListStarting = pane.list.GetItemCount()

	var err error
	pane.projects, err = pane.repo.GetAll()
	if err != nil {
		statusBar.showForSeconds("Could not load Projects: "+err.Error(), 5)
		return
	}

	for i := range pane.projects {
		pane.addProjectToList(i, false)
	}

	pane.list.SetCurrentItem(pane.dynamicListStarting) // Select "All" on start
}

func (pane *ProjectPane) addProjectToList(i int, selectItem bool) {
	// To avoid overriding of loop variables - https://www.calhoun.io/gotchas-and-common-mistakes-with-closures-in-go/
	pane.list.AddItem("- "+pane.projects[i].Title, "", 0, func(idx int) func() {
		return func() { pane.activateProject(idx) }
	}(i))

	if selectItem {
		pane.list.SetCurrentItem(-1)
		pane.activateProject(i)
	}
}

func (pane *ProjectPane) addSection(name string) {
	pane.list.AddItem("[::d]"+name, "", 0, nil)
	pane.list.AddItem("[::d]"+strings.Repeat(string(tcell.RuneHLine), 25), "", 0, nil)
}

func (pane *ProjectPane) handleShortcuts(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'J':
		// Jump down to the first project if the cursor is above it and a project exists
		if pane.list.GetCurrentItem() < pane.projectListStarting && len(pane.projects) > 0 {
			pane.list.SetCurrentItem(pane.projectListStarting)
		}
		return nil
	case 'K':
		// Jump up to the first dynamic list item if the cursor is below it
		if pane.list.GetCurrentItem() > pane.dynamicListStarting {
			pane.list.SetCurrentItem(pane.dynamicListStarting)
		}
		return nil
	}

	switch unicode.ToLower(event.Rune()) {
	case 'j':
		pane.list.SetCurrentItem(pane.list.GetCurrentItem() + 1)
		return nil
	case 'k':
		pane.list.SetCurrentItem(pane.list.GetCurrentItem() - 1)
		return nil
	case 'n':
		pane.showNewProjectInput()
		return nil
	case 'd':
		pane.startDeleteSelected()
		return nil
	}

	return event
}

// deleteConfirmActive is true while the "are you sure?" prompt for deleting a
// project is showing on the status bar. deleteConfirmName is the project name
// used in that prompt.
var (
	deleteConfirmActive bool
	deleteConfirmName   string
)

// deleteConfirmOptions are the choices shown in the delete confirmation prompt.
var deleteConfirmOptions = []keyHint{
	{"y", "Yes"},
	{"n", "No"},
}

// startDeleteSelected begins deleting the project under the cursor. The project
// is opened first (so its tasks are visible), then a confirmation prompt is
// shown on the status bar. Does nothing when the selection is not a project.
func (pane *ProjectPane) startDeleteSelected() {
	idx := pane.list.GetCurrentItem() - pane.projectListStarting
	if idx < 0 || idx >= len(pane.projects) {
		return
	}

	if pane.activeProject == nil || pane.activeProject.ID != pane.projects[idx].ID {
		pane.activateProject(idx)
	}
	app.SetFocus(projectPane) // keep the projects pane focused during the prompt

	deleteConfirmName = pane.projects[idx].Title
	deleteConfirmActive = true
}

// handleDeleteConfirm resolves the delete confirmation prompt. "y" deletes the
// active project; any other key (including "n" and Esc) cancels.
func handleDeleteConfirm(event *tcell.EventKey) *tcell.EventKey {
	deleteConfirmActive = false

	if unicode.ToLower(event.Rune()) == 'y' {
		projectPane.RemoveActivateProject()
	} else {
		app.SetFocus(projectPane)
	}

	return nil
}

func (pane *ProjectPane) activateProject(idx int) {
	pane.activeProject = &pane.projects[idx]
	taskPane.LoadProjectTasks(*pane.activeProject)

	removeThirdCol()
	projectDetailPane.SetProject(pane.activeProject)
	contents.AddItem(projectDetailPane, 25, 0, false)
	app.SetFocus(taskPane)
}

// RemoveActivateProject deletes the currently active project
func (pane *ProjectPane) RemoveActivateProject() {
	if pane.activeProject != nil && pane.repo.Delete(pane.activeProject) == nil {

		for i := range taskPane.tasks {
			_ = taskRepo.Delete(&taskPane.tasks[i])
		}

		title := pane.activeProject.Title
		pane.activeProject = nil
		// No project/list is shown anymore, so fall back to the splash screen.
		taskPane.ShowSplash()

		statusBar.showForSeconds("[lime]Removed Project: "+title, 5)
		removeThirdCol()

		pane.loadListItems(true)
	}
}

func (pane *ProjectPane) loadListItems(focus bool) {
	pane.list.Clear()
	pane.addDynamicLists()
	pane.list.AddItem("", "", 0, nil)
	pane.addProjectList()

	if focus {
		app.SetFocus(pane)
	}
}

// GetActiveProject provides pointer to currently active project
func (pane *ProjectPane) GetActiveProject() *model.Project {
	return pane.activeProject
}
