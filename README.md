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

### Import / Export

Type `/` in the capture or search input to open the command palette:

| Command | Description |
|---|---|
| `/export json` | Export all thoughts to a JSON file |
| `/export csv` | Export all thoughts to a CSV file |
| `/import json` | Import thoughts from a JSON file (additive — keeps existing data) |
| `/import json restore` | Import from JSON and replace all existing data |
| `/import csv` | Import thoughts from a CSV file (additive) |
| `/import csv restore` | Import from CSV and replace all existing data |
| `/restart` | Restart the application |
| `/quit` | Quit the application |

Use arrow keys to navigate the palette, **Tab** to autocomplete, **Enter** to execute, **Esc** to dismiss.

The JSON format preserves all fields including tags, context metadata, and intents. The CSV format covers the core thought content and is useful for importing from external sources — only a `content` column is required; `created_at`, `tags` (pipe-separated), `raw_content`, `hidden`, `window_title`, `app_name`, and `url` are optional.

## License

MIT
