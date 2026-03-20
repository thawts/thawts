# homebrew-tap

Homebrew tap for [Thawts](https://thawts.app).

## Setup

```sh
brew tap OWNER/tap
```

## Install

**GUI app (macOS only):**
```sh
brew install --cask thawts
```

**CLI/TUI binary (macOS + Linux):**
```sh
brew install thawts
```

## Creating the tap repo

1. Create a new GitHub repository named `homebrew-tap` under the same owner/org as the main repo.
2. Copy the contents of this directory (`build/homebrew-tap/`) into the root of that repo.
3. Replace all `OWNER` placeholders with your GitHub username/org.
4. Update `version` and `sha256` values from the first release artifacts.
5. Add a `HOMEBREW_TAP_TOKEN` secret (GitHub PAT with `repo` write access to `homebrew-tap`) and a `HOMEBREW_TAP_OWNER` secret (your GitHub username/org) to the main repo so GoReleaser can auto-update on future releases.

## Auditing

```sh
brew audit --cask thawts --online
brew audit thawts --online
```
