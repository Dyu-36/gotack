//go:build windows

package main

import (
	_ "embed"
	"runtime"

	"fyne.io/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/Dyu-36/gotack/internal/userstrings"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// startTray brings up the notification-area icon and keeps its message loop
// alive for the whole process lifetime. The loop owns a dedicated locked OS
// thread so it never steals the thread Wails runs its window loop on.
func startTray(a *App) {
	go func() {
		runtime.LockOSThread()
		systray.Run(func() { onTrayReady(a) }, nil)
	}()
}

func onTrayReady(a *App) {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("Tack")
	show := systray.AddMenuItem(userstrings.TrayShow, userstrings.TrayShow)
	for range show.ClickedCh {
		wailsruntime.WindowShow(a.ctx)
	}
}
