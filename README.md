# SpiceFix

A small Windows app that **installs, fixes and updates [Spicetify](https://spicetify.app)** with one click.

![platform](https://img.shields.io/badge/platform-Windows-blue) ![go](https://img.shields.io/badge/built%20with-Go-00ADD8)

## What it does

When you open it, SpiceFix shows whether Spicetify is installed, which version you have and the latest release, then lets you pick:

| Option | What happens |
|---|---|
| **1. Auto** (default) | Installs Spicetify if it's missing, updates it if it's outdated, otherwise re-applies it |
| **2. Install / reinstall** | Runs the official Spicetify installer |
| **3. Fix** | Re-applies Spicetify after a Spotify update (`restore backup apply`, with fallbacks) |
| **4. Update** | `spicetify upgrade`, falling back to a reinstall, then re-applies |
| **5. Marketplace** | Installs or repairs Spicetify Marketplace |
| **6. Deep fix** | Clears the old backup and applies from scratch – for a blank Spotify |

It also warns you if you have the **Microsoft Store** version of Spotify, which Spicetify can't modify.

## Download

Grab `SpiceFix.exe` from the [Releases](../../releases) page.

> The exe isn't code-signed, so Windows SmartScreen may warn you. Click **More info → Run anyway**.

## Usage notes

- Run it as a **normal user**, not as administrator – the Spicetify installer refuses to run elevated.
- Use the Spotify desktop app from [spotify.com](https://www.spotify.com/download/windows/), opened and logged in at least once.
- Requires an internet connection; it downloads the official installers from Spicetify's GitHub.

## Build it yourself

```sh
# on Windows
go build -ldflags "-s -w" -o SpiceFix.exe

# from Linux / macOS
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o SpiceFix.exe
```

Pushing a tag like `v1.0.0` builds the exe on GitHub Actions and attaches it to a release.

## Disclaimer

Not affiliated with Spicetify or Spotify. SpiceFix only runs Spicetify's own official installers and commands.
