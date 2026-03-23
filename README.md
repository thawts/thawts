# Thawts

Thought capture and review — a minimalist tool for quickly capturing thoughts, ideas, and tasks. Available on macOS, Linux, and Windows.

## Install

### Homebrew (macOS + Linux)

```sh
brew tap thawts/tap
brew install thawts
```

### Download

Grab the latest binary for your platform from [GitHub Releases](https://github.com/thawts/thawts/releases).

## Usage

### GUI Mode (default)

Run `thawts` to start the app. It lives in your system tray — no Dock icon. Register it to start automatically on login:

```sh
thawts --install
```

| Action | Shortcut |
|---|---|
| Show/hide capture window | `Ctrl+Alt+Space` |
| Open review | `Cmd+Option+R` (macOS) |
| Hide window | `Esc` or click away (macOS/Linux) / `Esc` or hotkey (Windows) |

### TUI Mode

Run from any terminal:

```sh
thawts --tui
```

| Key | Action |
|---|---|
| `Tab` / `Ctrl+R` | Switch to review |
| `Esc` | Review → capture; quit from capture |
| `Enter` | Save thought / confirm edit |
| `d` `d` | Delete (arm then confirm) |
| `e` | Edit selected thought |
| `/` | Focus search |

### All flags

```sh
thawts --install    # register thawts to start automatically on login
thawts --uninstall  # remove thawts from login items
thawts --tui        # run the terminal UI instead of the GUI
thawts --version    # print version and exit
```

`--install` uses the appropriate mechanism for each platform:

| Platform | Mechanism |
|---|---|
| macOS | `~/Library/LaunchAgents/` (launchd) |
| Linux | systemd user service, or `~/.config/autostart/` (XDG) |
| Windows | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` (registry) |

## Data

All data is stored locally at `~/.thawts/thawts.db` (SQLite). Nothing leaves your machine.

## License

MIT
