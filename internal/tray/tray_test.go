package tray

import (
	"context"
	"testing"
	"thawts-client/internal/app"
)

func TestRegisterApp(t *testing.T) {
	// This test just ensures RegisterApp sets the global variable without panic.
	// We cannot test InitTray easily as it involves CGO/UI loops.

	// Create a dummy app
	a := app.NewApp(nil)
	a.Startup(context.Background())

	// Call RegisterApp
	RegisterApp(a)

	if globalApp != a {
		t.Error("RegisterApp did not set globalApp correctly")
	}
}
