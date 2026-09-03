package main

import (
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// hiddenFlag marks a launch that should start with the window hidden in the
// tray: the login autostart entry always passes it.
const hiddenFlag = "--hidden"

func main() {
	app := NewApp()

	startHidden := false
	for _, arg := range os.Args[1:] {
		if arg == hiddenFlag {
			startHidden = true
			break
		}
	}

	err := wails.Run(&options.App{
		Title:     "Tack",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		// Closing the window only hides it; the process keeps serving the
		// tray, the Zalo bot, and the scheduler until it is ended from Task
		// Manager. Windows' HideWindowOnClose also keeps runtime.Quit from
		// destroying the window (it goes through OnBeforeClose internally),
		// so hiding is the single close behavior.
		HideWindowOnClose: true,
		StartHidden:       startHidden,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},

		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.dyu-36.tack",
			OnSecondInstanceLaunch: func(second options.SecondInstanceData) {
				// A second launch (Start menu, autostart while running)
				// surfaces the first instance's window instead — unless it
				// is itself a hidden autostart launch, which stays in the
				// tray.
				for _, arg := range second.Args {
					if arg == hiddenFlag {
						return
					}
				}
				app.showMainWindow()
			},
		},

		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
