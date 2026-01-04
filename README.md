# Thawts Client

Thawts Client is a minimalistic, Raycast-inspired desktop application for capturing thoughts, built with **Go (Wails)** and **Vanilla JS**.

## Features

- **Global Hotkey**: Press `Ctrl+Shift+Space` to toggle the input window from anywhere.
- **Agent Mode**: Runs in the background with no Dock icon.
- **System Tray**: Accessible via the menu bar icon (click to Show/Quit).
- **Auto-Hide**: The window automatically hides when it loses focus or when `Esc` is pressed.
- **Frameless UI**: A clean, floating input box design.

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Node.js](https://nodejs.org/) (for frontend build)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

## Development

To run the application in development mode:

```bash
wails dev
```

Note: In development mode, a terminal window will appear, and the app may show a Dock icon depending on debug configuration. Global hotkeys might conflict if multiple instances are running.

## Building

To build the production application (macOS .app bundle):

```bash
wails build
```

The output will be located in `build/bin/thawts-client.app`.

### Running the Production Build

To experience the "Agent Mode" (hidden dock) and System Tray features correctly, run the built bundle:

```bash
open build/bin/thawts-client.app
```

## Configuration

- **Hotkey**: Configured in `main.go`. Default is `Ctrl+Shift+Space`.
- **Agent Mode**: controlled by `LSUIElement` in `build/darwin/Info.plist`.

## Troubleshooting

- **Crash on Startup**: Ensure you are not running the binary directly from the terminal if possible, or ensure `Info.plist` is correct. The app relies on the bundle structure for resources.
- **Hotkeys not working**: Check Accessibility permissions in System Settings -> Privacy & Security -> Accessibility if you change the hotkey implementation to one requiring it (currently uses `RegisterEventHotKey` which usually works without, but permissions may vary by OS version).

## License

MIT License
