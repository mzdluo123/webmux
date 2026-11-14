//go:build windows

package winui

import (
	"sync/atomic"

	"golang.org/x/sys/windows"

	"github.com/rainchan/webmux/internal/tray"
)

const dialogTitle = "WebMux"

const (
	idOK  = 1
	idYes = 6
	idNo  = 7
)

// openDialogs counts message boxes on screen; a tray update waits for
// none to be open.
var openDialogs atomic.Int32

func messageBox(text string, flags uint32) int32 {
	openDialogs.Add(1)
	defer openDialogs.Add(-1)
	ret, _ := windows.MessageBox(0, utf16Ptr(text), utf16Ptr(dialogTitle), flags|windows.MB_SETFOREGROUND|windows.MB_TOPMOST)
	return ret
}

// confirm asks an OK/Cancel question.
func confirm(text string) bool {
	return messageBox(text, windows.MB_OKCANCEL|windows.MB_ICONWARNING|windows.MB_DEFBUTTON2) == idOK
}

// confirmRestart asks before restarting WebMux when ordinary terminal sessions
// or actively working sessions exist; idle hybrid sessions restart directly.
func confirmRestart(prefix string, snap tray.Snapshot) bool {
	prompt, needed := tray.RestartConfirmPrompt(prefix, snap)
	if !needed {
		return true
	}
	return confirm(prompt)
}

// confirmStop asks before stopping WebMux whenever sessions exist, describing
// which sessions end and which hybrid sessions recover on next start.
func confirmStop(prefix string, snap tray.Snapshot) bool {
	prompt, needed := tray.StopConfirmPrompt(prefix, snap)
	if !needed {
		return true
	}
	return confirm(prompt)
}

type quitChoice int

const (
	quitCancel quitChoice = iota
	quitStop
	quitTrayOnly
)

// askQuit is the three-way exit question for a managed running WebMux.
// MessageBoxW cannot relabel its buttons, so the text explains them.
func askQuit(sessions int, forwarding bool) quitChoice {
	text := tray.QuitPrompt(sessions, forwarding)
	switch messageBox(text, windows.MB_YESNOCANCEL|windows.MB_ICONQUESTION|windows.MB_DEFBUTTON2) {
	case idYes:
		return quitStop
	case idNo:
		return quitTrayOnly
	}
	return quitCancel
}
