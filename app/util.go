package main

import (
	"fmt"
	"reflect"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/util"
)

var blankCell = tview.NewTextView()

func makeHorizontalLine(lineChar rune, color tcell.Color) *tview.TextView {
	hr := tview.NewTextView()
	hr.SetDrawFunc(func(screen tcell.Screen, x int, y int, width int, height int) (int, int, int, int) {
		// Draw a horizontal line across the middle of the box.
		style := tcell.StyleDefault.Foreground(color).Background(tcell.ColorBlack)
		centerY := y + height/2
		for cx := x; cx < x+width; cx++ {
			screen.SetContent(cx, centerY, lineChar, nil, style)
		}

		// Space for other content.
		return x + 1, centerY + 1, width - 2, height - (centerY + 1 - y)
	})

	return hr
}

func makeLightTextInput(placeholder string) *tview.InputField {
	return tview.NewInputField().
		SetPlaceholder(placeholder).
		SetPlaceholderTextColor(tcell.ColorDarkSlateBlue).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldBackgroundColor(tcell.ColorLightBlue)
}

// If input text is a valid date, parse it. Or get current date
func parseDateInputOrCurrent(inputText string) time.Time {
	if dateTime, err := time.Parse(dateLayoutISO, inputText); err == nil {
		return toDate(dateTime)
	}

	return toDate(time.Now())
}

func toDate(dateTime time.Time) time.Time {
	return time.Date(dateTime.Year(), dateTime.Month(), dateTime.Day(), 0, 0, 0, 0, time.Local)
}

func makeButton(label string, handler func()) *tview.Button {
	btn := tview.NewButton(label).SetSelectedFunc(handler).
		SetLabelColor(tcell.ColorWhite)

	btn.SetBackgroundColor(tcell.ColorCornflowerBlue)

	return btn
}

func ignoreKeyEvt() bool {
	textInputs := []string{"*tview.InputField", "*femto.View"}
	return util.InArray(reflect.TypeOf(app.GetFocus()).String(), textInputs)
}

// yetToImplement - to use as callback for unimplemented features
// `yetToImplement` is unused (deadcode)
// func yetToImplement(feature string) func() {
// 	message := fmt.Sprintf("[yellow]%s is yet to implement. Please Check in next version.", feature)
// 	return func() { statusBar.showForSeconds(message, 5) }
// }

// thirdCol tracks which pane (if any) currently occupies the right-hand column of
// the contents Flex: taskDetailPane or nil. It lets Help restore the previous
// layout when it closes.
var thirdCol tview.Primitive

func removeThirdCol() {
	contents.RemoveItem(taskDetailPane)
	thirdCol = nil
}

func getTaskTitleColor(task model.Task) string {
	colorName := "smokewhite"

	if task.Completed {
		colorName = "green"
	} else if task.DueDate != 0 {
		dayDiff := int(time.Until(time.Unix(task.DueDate, 0)).Hours() / 24)
		if dayDiff == 0 {
			colorName = "orange"
		} else if dayDiff < 0 {
			colorName = "red"
		}
	}

	return colorName
}

func makeTaskListingTitle(task model.Task) string {
	checkbox := "[ []"
	if task.Completed {
		checkbox = "[x[]"
	}

	prefix := ""
	if projectPane.GetActiveProject() == nil {
		if project, err := projectRepo.GetByID(task.ProjectID); err == nil {
			prefix = project.Title + ": "
		}
	}

	color := getTaskTitleColor(task)
	title := fmt.Sprintf("[%s]%s ", color, checkbox)
	// The priority cookie carries its own color ([A] green, [C] orange); B has
	// none. Restore the base color afterwards for the title text.
	if cookie := priorityCookie(task); cookie != "" {
		title += cookie + " "
	}
	title += fmt.Sprintf("[%s]%s%s", color, prefix, task.Title)

	return title
}

// `findProjectByID` is unused (deadcode)
// func findProjectByID(id int64) *model.Project {
// 	for i := range projectPane.projects {
// 		if projectPane.projects[i].ID == id {
// 			return &projectPane.projects[i]
// 		}
// 	}

// 	return nil
// }
