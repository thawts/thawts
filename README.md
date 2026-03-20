# Thawts

Thought capture and review — a minimalist tool for quickly capturing thoughts, ideas, and tasks. Available as a **desktop GUI app** (macOS) and a **terminal TUI** (macOS, Linux, Windows).

## Install

### Homebrew (once available)

```sh
# GUI app (macOS only)
brew install --cask thawts

# CLI/TUI binary (macOS + Linux)
brew tap OWNER/tap
brew install thawts
```

### Download

Grab the latest binary from [GitHub Releases](https://github.com/OWNER/thawts-client-go/releases).

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

---

## Development

### Prerequisites

- **Go 1.25+** — [go.dev/dl](https://go.dev/dl/)
- **Node.js 20+** — for the frontend (GUI only)
- **Wails v3 CLI** — `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

### Running tests

The GUI embeds `frontend/dist`. On a fresh clone that directory doesn't exist yet, which causes `go test ./...` to fail for the `app` package. Create a stub first:

```sh
mkdir -p frontend/dist && touch frontend/dist/index.html
go test ./...
```

All non-GUI packages (service, storage, ai, tui, …) test cleanly without the stub.

### Dev mode (GUI)

```sh
wails3 dev
```

Hot-reloads the frontend. The app shows a Dock icon in dev mode.

### Architecture

```
main.go               entry point — --version, --tui flags, Wails bootstrap
internal/
  service/            business logic (zero Wails imports)
    service.go        SaveThought, SearchThoughts, review, intents, wellbeing, …
    notifier.go       Notifier interface + NoopNotifier + RecordingNotifier
    service_test.go   30 tests; real SQLite in t.TempDir(), StubProvider
  app/                thin Wails adapter
    app.go            window control methods, promotes service.Service to JS
    notifier.go       WailsNotifier wraps *application.App
    app_test.go
  tui/                Bubble Tea terminal UI
    tui.go            Run() entry point, TUINotifier
    model.go          pure state machine — capture + review views
    model_test.go     12 tests using apply/exec helpers
  storage/            SQLite persistence (modernc.org/sqlite — pure Go, no CGO)
  ai/                 LLM inference (local GGUF model)
  domain/             shared types
  metadata/           active window context capture
  icon/               tray icon assets
frontend/             Vanilla JS + CSS (GUI only)
```

**Key design principle**: `internal/service` contains all business logic and has zero framework imports. The `Notifier` interface (`Emit(event string, data ...any)`) decouples event emission from Wails — `WailsNotifier` wires it to Wails events, `TUINotifier` sends to the Bubble Tea program.

### Building a release

Tag a commit with a semver tag to trigger the release pipeline:

```sh
git tag v0.4.0
git push origin v0.4.0
```

This runs `.github/workflows/release.yml` which:
1. Builds binaries for darwin/linux/windows × amd64/arm64 via GoReleaser
2. Signs and notarizes the macOS binary (if `APPLE_*` secrets are set)
3. Creates a GitHub Release with zip archives and `checksums.txt`
4. Auto-updates `Formula/thawts.rb` in the homebrew-tap repo (if `HOMEBREW_TAP_*` secrets are set)

### CI

`.github/workflows/ci.yml` runs on every push and PR: builds the frontend, runs `go test ./...`, and does a smoke-build on macOS, Linux, and Windows.

### Required secrets (GitHub Actions)

| Secret | Purpose |
|---|---|
| `APPLE_CERTIFICATE` | Base64-encoded Developer ID certificate (macOS signing) |
| `APPLE_CERTIFICATE_PASSWORD` | Certificate passphrase |
| `APPLE_TEAM_ID` | Apple Developer Team ID |
| `APPLE_ID` | Apple ID for notarization |
| `APPLE_APP_PASSWORD` | App-specific password for notarization |
| `HOMEBREW_TAP_TOKEN` | GitHub PAT with `repo` write on `homebrew-tap` |
| `HOMEBREW_TAP_OWNER` | GitHub username/org owning `homebrew-tap` |

All signing/notarization and Homebrew secrets are optional — the pipeline skips those steps gracefully when they're absent.

## License

MIT
