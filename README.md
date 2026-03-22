# Thawts

Thought capture and review — a minimalist tool for quickly capturing thoughts, ideas, and tasks. Available as a **desktop GUI app** (macOS) and a **terminal TUI** (macOS, Linux, Windows).

## Install

### Homebrew

```sh
# GUI app (macOS only)
brew install --cask thawts

# CLI/TUI binary (macOS + Linux)
brew tap thawts/tap
brew install thawts
```

### Download

Grab the latest binary from [GitHub Releases](https://github.com/thawts/thawts/releases).

## Usage

### GUI Mode (default)

Launch the `.app` bundle or run the binary without flags. The app lives in your menu bar — no Dock icon.

| Action | Shortcut |
|---|---|
| Show/hide capture window | `Ctrl+Shift+Space` |
| Open review | `Cmd+Option+R` |
| Hide window | `Esc` or click away |

The window auto-hides when it loses focus (capture mode only).

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

### Other flags

```sh
thawts --version   # print version and exit
```

## Data

All data is stored locally at `~/.thawts/thawts.db` (SQLite). Nothing leaves your machine.

## License

MIT
