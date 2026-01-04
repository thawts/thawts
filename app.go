package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx       context.Context
	isVisible bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) Hide() {
	a.isVisible = false
	runtime.WindowHide(a.ctx)
}

func (a *App) Show() {
	a.isVisible = true
	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
}

func (a *App) Toggle() {
	if a.isVisible {
		a.Hide()
	} else {
		a.Show()
	}
}

func (a *App) Quit() {
	runtime.Quit(a.ctx)
}
