package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

const (
	appTitle         = "CPA Control Center"
	preferredWidth   = 1760
	preferredHeight  = 1000
	defaultMinWidth  = 720
	defaultMinHeight = 480
)

func main() {
	app := NewApp()

	err := wails.Run(newAppOptions(app))
	if err != nil {
		println("Error:", err.Error())
	}
}

func newAppOptions(app *App) *options.App {
	startupWidth, startupHeight := resolveStartupWindowSize()

	appOptions := &options.App{
		Title:     appTitle,
		Width:     startupWidth,
		Height:    startupHeight,
		MinWidth:  defaultMinWidth,
		MinHeight: defaultMinHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: appBackgroundColour(),
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	}

	applyPlatformOptions(appOptions)
	return appOptions
}

func appBackgroundColour() *options.RGBA {
	return &options.RGBA{R: 242, G: 238, B: 227, A: 1}
}
