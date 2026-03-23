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

## On-device AI

Thawts includes a local AI model that runs entirely on your device. No internet connection is required after the first launch, and no data ever leaves your machine.

The model ([all-MiniLM-L6-v2](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2), ~22 MB) is embedded directly in the binary. The ONNX Runtime library (~30 MB) is extracted to your OS cache folder silently on first launch.

| Feature | How it works |
|---|---|
| **Auto-tagging** | Classifies thoughts into categories (todo, idea, question, calendar, reminder, finance) using semantic similarity |
| **Sentiment tracking** | Scores the emotional tone of each thought for the wellbeing trend view |
| **Semantic search** | Finds thoughts by meaning, not just keywords (embeddings stored locally in `~/.thawts/thawts.db`) |
| **Mishap detection** | Flags accidental captures (passwords, code snippets, large pastes) |

AI is enabled on macOS (arm64) and Linux (amd64/arm64). Windows uses a regex-based fallback; full AI support is planned.

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

## Building from source

```sh
# Install prerequisites
brew install go node   # or your platform's package manager

# Clone and build (stub AI — no downloads needed)
git clone https://github.com/thawts/thawts
cd thawts
npm ci --prefix frontend && npm run build --prefix frontend
go build -o thawts .

# Build with on-device AI (macOS / Linux)
bash scripts/download_ai_deps.sh   # downloads model + native libs (~90 MB, one-time)
go build -tags with_onnx -o thawts .
```

## License

MIT
