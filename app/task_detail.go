package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/pgavlin/femto"
	"github.com/pgavlin/femto/runtime"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
	"github.com/ajaxray/geek-life/util"
)

const dateLayoutISO = "2006-01-02"
const dateLayoutHuman = "02 Jan, Monday"

// taskNoteDefaultHeight is the preferred height (in rows) of the task note
// editor when the terminal is tall enough to accommodate it.
const taskNoteDefaultHeight = 30

// taskDetailFixedRows is the number of rows the detail pane uses for everything
// other than the note editor and the flexible spacer (header, date row, labels,
// hints, status toggle, and the blank separators between them). It is used to
// work out how much vertical space is left for the note editor.
const taskDetailFixedRows = 13

// TaskDetailPane displays detailed info of a Task
type TaskDetailPane struct {
	*tview.Flex
	header           *TaskDetailHeader
	taskDateDisplay  *tview.TextView
	editorHint       *tview.TextView
	taskDate         *tview.InputField
	taskStatusToggle *tview.Button
	taskDetailView   *femto.View
	colorScheme      femto.Colorscheme
	taskRepo         repository.TaskRepository
	task             *model.Task
}

// NewTaskDetailPane initializes and configures a TaskDetailPane
func NewTaskDetailPane(taskRepo repository.TaskRepository) *TaskDetailPane {
	pane := TaskDetailPane{
		Flex:             tview.NewFlex().SetDirection(tview.FlexRow),
		header:           NewTaskDetailHeader(taskRepo),
		taskDateDisplay:  tview.NewTextView().SetDynamicColors(true),
		taskStatusToggle: makeButton("Complete", nil).SetLabelColor(tcell.ColorLightGray),
		taskRepo:         taskRepo,
	}

	pane.prepareDetailsEditor()

	toggleHint := tview.NewTextView().SetTextColor(tcell.ColorDimGray).SetText("<space> to toggle")
	pane.taskStatusToggle.SetSelectedFunc(pane.toggleTaskStatus)

	pane.editorHint = tview.NewTextView().SetText(" e = edit, v = external, ↓↑ = scroll").SetTextColor(tcell.ColorDimGray)

	// Prepare static (no external interaction) elements
	editorLabel := tview.NewFlex().
		AddItem(tview.NewTextView().SetText("Task Not[::u]e[::-]:").SetDynamicColors(true), 0, 1, false).
		AddItem(makeButton("[::u]e[::-]dit", func() { pane.activateEditor() }), 6, 0, false)
	editorHelp := tview.NewFlex().
		AddItem(pane.editorHint, 0, 1, false).
		AddItem(tview.NewTextView().SetTextAlign(tview.AlignRight).
			SetText("syntax:markdown (monakai)").
			SetTextColor(tcell.ColorDimGray), 0, 1, false)

	pane.
		AddItem(pane.header, 4, 1, true).
		AddItem(blankCell, 1, 1, false).
		AddItem(pane.makeDateRow(), 1, 1, true).
		AddItem(blankCell, 1, 1, false).
		AddItem(editorLabel, 1, 1, false).
		AddItem(pane.taskDetailView, taskNoteDefaultHeight, 0, false).
		AddItem(editorHelp, 1, 1, false).
		AddItem(blankCell, 0, 1, false).
		AddItem(toggleHint, 1, 1, false).
		AddItem(pane.taskStatusToggle, 3, 1, false)

	pane.SetBorder(true).SetTitle("Task Detail")

	// Keep the note editor at its default height when the terminal is tall
	// enough, but shrink it to fit when the terminal is shorter. Recomputed on
	// every draw so it adapts to resizes. The returned rect is the pane's inner
	// area, which the Flex layout reads back via GetInnerRect.
	pane.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		innerX, innerY, innerWidth, innerHeight := x+1, y+1, width-2, height-2

		noteHeight := taskNoteDefaultHeight
		if avail := innerHeight - taskDetailFixedRows; avail < noteHeight {
			noteHeight = avail
		}
		if noteHeight < 1 {
			noteHeight = 1
		}
		pane.ResizeItem(pane.taskDetailView, noteHeight, 0)

		return innerX, innerY, innerWidth, innerHeight
	})

	return &pane
}

func (td *TaskDetailPane) Export() {
	var content bytes.Buffer

	content.WriteString("# " + td.task.Title + " \n")
	if td.taskDate.GetText() != "" {
		content.WriteString("\n> Due Date: " + td.taskDate.GetText() + " \n")
	}
	content.WriteString("\n" + td.task.Details + " \n")

	_ = clipboard.WriteAll(content.String())
	app.SetFocus(td)
	statusBar.showForSeconds("Task copied. Try Pasting anywhere.", 5)
}

func (td *TaskDetailPane) makeDateRow() *tview.Flex {

	td.taskDate = makeLightTextInput("yyyy-mm-dd").
		SetLabel("Set:").
		SetLabelColor(tcell.ColorWhiteSmoke).
		SetFieldWidth(12).
		SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				date := parseDateInputOrCurrent(td.taskDate.GetText())
				td.setTaskDate(date.Unix(), true)
			case tcell.KeyEsc:
				td.setTaskDate(td.task.DueDate, false)
			}
			app.SetFocus(td)
		})

	return tview.NewFlex().
		AddItem(td.taskDateDisplay, 0, 2, true).
		AddItem(td.taskDate, 14, 0, true).
		AddItem(blankCell, 1, 0, false).
		AddItem(makeButton("today", td.todaySelector), 8, 1, false).
		AddItem(blankCell, 1, 0, false).
		AddItem(makeButton("+1", td.nextDaySelector), 4, 1, false).
		AddItem(blankCell, 1, 0, false).
		AddItem(makeButton("-1", td.prevDaySelector), 4, 1, false).
		AddItem(blankCell, 1, 0, false).
		AddItem(makeButton("unset", td.unsetDateSelector), 8, 1, false)
}

func (td *TaskDetailPane) updateToggleDisplay() {
	if td.task.Completed {
		td.taskStatusToggle.SetLabel("Resume").SetBackgroundColor(tcell.ColorMaroon)
	} else {
		td.taskStatusToggle.SetLabel("Complete").SetBackgroundColor(tcell.ColorDarkGreen)
	}
}

func (td *TaskDetailPane) toggleTaskStatus() {
	status := !td.task.Completed
	if taskRepo.UpdateField(td.task, "Completed", status) == nil {
		td.task.Completed = status
		taskPane.ReloadCurrentTask()
	}
}

// changeTaskPriority raises (delta -1) or lowers (delta +1) the shown task's
// priority. The header and the visible list item update immediately; the list
// re-sorts when the detail closes (RefreshAfterEdit).
func (td *TaskDetailPane) changeTaskPriority(delta int) {
	if td.task == nil {
		return
	}
	if !setTaskPriority(td.taskRepo, td.task, delta) {
		return
	}
	td.header.SetTask(td.task)
	taskPane.list.SetItemText(taskPane.list.GetCurrentItem(), makeTaskListingTitle(*td.task), "")
}

// Display Task date in detail pane, and update date if asked to
func (td *TaskDetailPane) setTaskDate(unixDate int64, update bool) {
	if update {
		td.task.DueDate = unixDate
		if err := td.taskRepo.UpdateField(td.task, "DueDate", unixDate); err != nil {
			statusBar.showForSeconds("Could not update due date: "+err.Error(), 5)
			return
		}
	}

	if unixDate != 0 {
		due := time.Unix(unixDate, 0)
		color := "white"
		humanDate := due.Format(dateLayoutHuman)

		if due.Before(time.Now()) {
			color = "red"
		}
		td.taskDateDisplay.SetText(fmt.Sprintf("[::u]D[::-]ue: [%s]%s", color, humanDate))
		td.taskDate.SetText(due.Format(dateLayoutISO))
	} else {
		td.taskDate.SetText("")
		td.taskDateDisplay.SetText("[::u]D[::-]ue: [::d]Not set")
	}
}

func (td *TaskDetailPane) prepareDetailsEditor() {

	td.taskDetailView = femto.NewView(makeBufferFromString(""))
	td.taskDetailView.SetRuntimeFiles(runtime.Files)

	// var colorScheme femto.Colorscheme
	if monokai := runtime.Files.FindFile(femto.RTColorscheme, "monokai"); monokai != nil {
		if data, err := monokai.Data(); err == nil {
			td.colorScheme = femto.ParseColorscheme(string(data))
		}
	}

	td.taskDetailView.SetColorscheme(td.colorScheme)
	td.taskDetailView.SetBorder(true)
	td.taskDetailView.SetBorderColor(tcell.ColorLightSlateGray)

	td.taskDetailView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			td.updateTaskNote(td.taskDetailView.Buf.String())
			td.deactivateEditor()
			return nil
		}

		return event
	})
}

func (td *TaskDetailPane) updateTaskNote(note string) {
	td.task.Details = note
	err := taskRepo.Update(td.task)
	if err == nil {
		statusBar.showForSeconds("[lime]Saved task detail", 5)
	} else {
		statusBar.showForSeconds("[red]Could not save: "+err.Error(), 5)
	}
}

func makeBufferFromString(content string) *femto.Buffer {
	buff := femto.NewBufferFromString(content, "")
	// taskDetail.Settings["ruler"] = false
	buff.Settings["filetype"] = "markdown"
	buff.Settings["keepautoindent"] = true
	buff.Settings["statusline"] = false
	buff.Settings["softwrap"] = true
	buff.Settings["scrollbar"] = true

	return buff
}

func (td *TaskDetailPane) activateEditor() {
	td.taskDetailView.Readonly = false
	td.taskDetailView.SetBorderColor(tcell.ColorDarkOrange)
	td.editorHint.SetText(" Esc to save changes")
	app.SetFocus(td.taskDetailView)
}

func (td *TaskDetailPane) deactivateEditor() {
	td.taskDetailView.Readonly = true
	td.taskDetailView.SetBorderColor(tcell.ColorLightSlateGray)
	td.editorHint.SetText(" e = edit, v = external, ↓↑ = scroll")
	app.SetFocus(td)
}

func (td *TaskDetailPane) editInExternalEditor() {

	tmpFileName, err := writeToTmpFile(td.task.Details)
	if err != nil {
		statusBar.showForSeconds("[red::]Failed to create tmp file. Try in-app editing by pressing i", 5)
		return
	}

	var messageToShow, updatedContent string
	app.Suspend(func() {
		cmd := exec.Command(util.GetEnvStr("EDITOR", "vim"), tmpFileName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			messageToShow = "[red::]Failed to save content. Try in-app editing by pressing e"
			return
		}

		if content, readErr := ioutil.ReadFile(tmpFileName); readErr == nil {
			updatedContent = string(content)
		} else {
			messageToShow = "[red::]Failed to load external editing. Try in-app editing by pressing e"
		}
	})

	if messageToShow != "" {
		statusBar.showForSeconds(messageToShow, 10)
	}

	if updatedContent != "" {
		td.updateTaskNote(updatedContent)
		td.SetTask(td.task)
	}

	app.EnableMouse(true)

	_ = os.Remove(tmpFileName)

	// app.SetFocus(td)
}

// writeToTmpFile writes given content to a tmpFile and returns the filename
func writeToTmpFile(content string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "geek_life_task_note_*.md")
	if err != nil {
		return "", err
	}
	fileName := tmpFile.Name()

	if err = ioutil.WriteFile(fileName, []byte(content), 0777); err != nil {
		return "", err
	}

	return fileName, tmpFile.Close()
}

func (td *TaskDetailPane) handleShortcuts(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyLeft:
		// Esc and Left close the task and move focus back to the task list.
		removeThirdCol()
		taskPane.RefreshAfterEdit() // reflect date/color changes; re-sort dynamic lists
		app.SetFocus(taskPane)
		return nil
	case tcell.KeyDown:
		td.taskDetailView.ScrollDown(1)
		return nil
	case tcell.KeyUp:
		td.taskDetailView.ScrollUp(1)
		return nil
	case tcell.KeyRune:
		switch unicode.ToLower(event.Rune()) {
		case 'e':
			td.activateEditor()
			return nil
		case 'v':
			td.editInExternalEditor()
			return nil
		case 'd':
			app.SetFocus(td.taskDate)
			return nil
		case 'r':
			td.header.ShowRename()
			return nil
		case ' ':
			td.toggleTaskStatus()
			return nil
		case 'x':
			td.Export()
			return nil
		case 'o':
			td.todaySelector()
			return nil
		case 'u':
			td.unsetDateSelector()
			return nil
		case ']':
			td.nextDaySelector()
			return nil
		case '[':
			td.prevDaySelector()
			return nil
		case '=', '+':
			td.changeTaskPriority(-1)
			return nil
		case '-':
			td.changeTaskPriority(1)
			return nil
		}
	}

	return event
}

// SetTask sets a Task to be displayed
func (td *TaskDetailPane) SetTask(task *model.Task) {
	td.task = task

	td.header.SetTask(task)
	td.taskDetailView.Buf = makeBufferFromString(td.task.Details)
	td.taskDetailView.SetColorscheme(td.colorScheme)
	td.taskDetailView.Start()
	td.setTaskDate(td.task.DueDate, false)
	td.updateToggleDisplay()
	td.deactivateEditor()
}

func (td *TaskDetailPane) todaySelector() {
	td.setTaskDate(parseDateInputOrCurrent("").Unix(), true)
}

func (td *TaskDetailPane) nextDaySelector() {
	td.setTaskDate(parseDateInputOrCurrent(td.taskDate.GetText()).AddDate(0, 0, 1).Unix(), true)
}

func (td *TaskDetailPane) prevDaySelector() {
	td.setTaskDate(parseDateInputOrCurrent(td.taskDate.GetText()).AddDate(0, 0, -1).Unix(), true)
}

func (td *TaskDetailPane) unsetDateSelector() {
	td.setTaskDate(0, true)
}
