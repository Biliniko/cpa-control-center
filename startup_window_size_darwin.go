//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

typedef struct {
	double width;
	double height;
	int ok;
} CPAVisibleFrame;

static CPAVisibleFrame CPAGetVisibleFrame(void) {
	@autoreleasepool {
		NSScreen *screen = [NSScreen mainScreen];
		if (screen == nil) {
			NSArray<NSScreen *> *screens = [NSScreen screens];
			if (screens.count > 0) {
				screen = screens[0];
			}
		}
		if (screen == nil) {
			return (CPAVisibleFrame){0, 0, 0};
		}

		NSRect frame = [screen visibleFrame];
		return (CPAVisibleFrame){frame.size.width, frame.size.height, 1};
	}
}
*/
import "C"

func resolveStartupWindowSize() (int, int) {
	frame := C.CPAGetVisibleFrame()
	if frame.ok == 0 {
		return preferredWidth, preferredHeight
	}

	return minInt(preferredWidth, int(frame.width)), minInt(preferredHeight, int(frame.height))
}
