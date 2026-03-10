//go:build windows

package main

import (
	"syscall"

	"github.com/wailsapp/wails/v2/pkg/options"
	windowopts "github.com/wailsapp/wails/v2/pkg/options/windows"
)

var windowsUser32DLL = syscall.NewLazyDLL("user32.dll")

func applyPlatformOptions(appOptions *options.App) {
	appOptions.Windows = &windowopts.Options{
		Theme: windowopts.Light,
		CustomTheme: &windowopts.ThemeSettings{
			LightModeTitleBar:          windowopts.RGB(222, 214, 198),
			LightModeTitleBarInactive:  windowopts.RGB(236, 227, 211),
			LightModeTitleText:         windowopts.RGB(38, 34, 27),
			LightModeTitleTextInactive: windowopts.RGB(95, 87, 74),
			LightModeBorder:            windowopts.RGB(222, 214, 198),
			LightModeBorderInactive:    windowopts.RGB(236, 227, 211),
		},
	}
}
