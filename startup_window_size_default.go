//go:build !windows && !darwin

package main

func resolveStartupWindowSize() (int, int) {
	return preferredWidth, preferredHeight
}
