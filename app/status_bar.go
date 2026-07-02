package main

import (
	"time"

	"github.com/rivo/tview"
)

// StatusBar displays hints and messages at the bottom of app
type StatusBar struct {
	*tview.Pages
	message   *tview.TextView
	hints     *tview.TextView
	container *tview.Application
}

// Name of page keys
const (
	defaultPage = "default"
	messagePage = "message"
)

// Used to skip queued restore of statusBar
// in case of new showForSeconds within waiting period
var restorInQ = 0

func prepareStatusBar(app *tview.Application) *StatusBar {
	statusBar = &StatusBar{
		Pages:     tview.NewPages(),
		message:   tview.NewTextView().SetDynamicColors(true).SetText("Loading..."),
		container: app,
	}

	// Context-aware key hints. The content is left-aligned and refreshed before
	// every draw (see setKeyboardShortcuts) so it always reflects the focused
	// context; wrapping is disabled so a long list truncates on the single row.
	statusBar.hints = tview.NewTextView().SetDynamicColors(true).SetWrap(false)

	statusBar.AddPage(messagePage, statusBar.message, true, true)
	statusBar.AddPage(defaultPage, statusBar.hints, true, true)

	statusBar.updateHints()

	return statusBar
}

// updateHints refreshes the key-hint row for the currently focused context.
func (bar *StatusBar) updateHints() {
	bar.hints.SetText(formatKeyHints(currentKeyHints()))
}

func (bar *StatusBar) restore() {
	bar.container.QueueUpdateDraw(func() {
		bar.SwitchToPage(defaultPage)
	})
}

func (bar *StatusBar) showForSeconds(message string, timeout int) {
	if bar.container == nil {
		return
	}

	bar.message.SetText(message)
	bar.SwitchToPage(messagePage)
	restorInQ++

	go func() {
		time.Sleep(time.Second * time.Duration(timeout))

		// Apply restore only if this is the last pending restore
		if restorInQ == 1 {
			bar.restore()
		}
		restorInQ--
	}()
}
