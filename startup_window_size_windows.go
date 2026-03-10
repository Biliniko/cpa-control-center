//go:build windows

package main

import "unsafe"

const (
	spiGetWorkArea = 0x0030
)

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func resolveStartupWindowSize() (int, int) {
	workWidth, workHeight, ok := getWindowsWorkArea()
	if !ok {
		return preferredWidth, preferredHeight
	}

	return minInt(preferredWidth, workWidth), minInt(preferredHeight, workHeight)
}

func getWindowsWorkArea() (int, int, bool) {
	if err := windowsUser32DLL.Load(); err != nil {
		return 0, 0, false
	}
	proc := windowsUser32DLL.NewProc("SystemParametersInfoW")

	var area rect
	result, _, _ := proc.Call(
		uintptr(spiGetWorkArea),
		0,
		uintptr(unsafe.Pointer(&area)),
		0,
	)
	if result == 0 {
		return 0, 0, false
	}

	return int(area.Right - area.Left), int(area.Bottom - area.Top), true
}
