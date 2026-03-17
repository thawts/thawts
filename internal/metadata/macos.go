//go:build darwin && metadata_cgo

package metadata

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics
#include <stdlib.h>
#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>

// getAppName returns the localised name of the frontmost application.
char* getAppName() {
    @autoreleasepool {
        NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!app || !app.localizedName) return strdup("");
        const char *name = [app.localizedName UTF8String];
        return strdup(name ? name : "");
    }
}

// getWindowTitle returns the title of the frontmost application's focused window
// using CGWindowListCopyWindowInfo (no Accessibility permissions required).
char* getWindowTitle() {
    @autoreleasepool {
        NSRunningApplication *frontApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        if (!frontApp) return strdup("");

        pid_t pid = frontApp.processIdentifier;

        CFArrayRef windowList = CGWindowListCopyWindowInfo(
            kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
            kCGNullWindowID
        );
        if (!windowList) return strdup("");

        NSArray *windows = (__bridge_transfer NSArray*)windowList;
        for (NSDictionary *window in windows) {
            NSNumber *ownerPID = window[(__bridge NSString*)kCGWindowOwnerPID];
            if (ownerPID.intValue == (int)pid) {
                NSString *name = window[(__bridge NSString*)kCGWindowName];
                if (name.length > 0) {
                    const char *title = [name UTF8String];
                    return strdup(title ? title : "");
                }
            }
        }
        return strdup("");
    }
}
*/
import "C"
import "unsafe"

// MacOSProvider captures active app name and window title on macOS using
// NSWorkspace and CGWindowListCopyWindowInfo (no Accessibility permissions needed).
type MacOSProvider struct{}

func NewMacOSProvider() *MacOSProvider { return &MacOSProvider{} }

func (p *MacOSProvider) GetActiveAppName() string {
	cs := C.getAppName()
	defer C.free(unsafe.Pointer(cs))
	return C.GoString(cs)
}

func (p *MacOSProvider) GetActiveWindowTitle() string {
	cs := C.getWindowTitle()
	defer C.free(unsafe.Pointer(cs))
	return C.GoString(cs)
}

// GetActiveURL requires per-browser AppleScript and is not implemented in Phase 1.
func (p *MacOSProvider) GetActiveURL() string { return "" }
