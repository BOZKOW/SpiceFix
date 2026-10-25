# SpiceFix

A small app for **Windows and Linux** that **installs, fixes and updates [Spicetify](https://spicetify.app)** with one click.

![platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux-blue) ![go](https://img.shields.io/badge/built%20with-Go-00ADD8)

## What it does

When you open it, SpiceFix finds your Spotify install, shows whether Spicetify is installed, which version you have and the latest release, then lets you pick:

| Option | What happens |
|---|---|
| **1. Auto** (default) | Installs Spicetify if it's missing, updates it if it's outdated, otherwise re-applies it |
| **2. Install / reinstall** | Runs the official Spicetify installer |
| **3. Fix** | Re-applies Spicetify after a Spotify update (`restore backup apply`, with fallbacks) |
| **4. Update** | `spicetify upgrade`, falling back to a reinstall, then re-applies |
| **5. Marketplace** | Installs or repairs Spicetify Marketplace |
| **6. Deep fix** | Clears the old backup and applies from scratch – for a blank Spotify |

## Download

Get the right file from the [Releases](../../releases) page:

| System | File |
|---|---|
| Windows (64-bit) | `SpiceFix.exe` |
| Linux (most PCs) | `SpiceFix-linux-amd64` |
| Linux on ARM (Raspberry Pi 4/5, ARM laptops) | `SpiceFix-linux-arm64` |

## Windows

Double-click `SpiceFix.exe`.

- Run it as a **normal user**, not as administrator – the Spicetify installer refuses to run elevated.
- The exe isn't code-signed, so SmartScreen may warn you. Click **More info → Run anyway**.
- Use Spotify from [spotify.com](https://www.spotify.com/download/windows/). The **Microsoft Store** version can't be modified – SpiceFix will tell you if that's what you have.

## Linux

Open a terminal in the folder you downloaded it to and run:

```sh
chmod +x SpiceFix-linux-amd64
./SpiceFix-linux-amd64
```

- **Don't use `sudo`.** SpiceFix asks for your password itself, only when it has to make Spotify's folder writable (Spotify updates reset this, so you may be asked again after one).
- It needs `curl` or `wget` (preinstalled on nearly every distro).

Supported Spotify installs, detected automatically:

| Install | Typical distros | Folder |
|---|---|---|
| Official .deb repo | Ubuntu, Debian, Mint, Pop!_OS, Zorin | `/opt/spotify` or `/usr/share/spotify` |
| AUR (`spotify`) | Arch, Manjaro, EndeavourOS, CachyOS | `/opt/spotify` |
| negativo17 RPM | Fedora | `/usr/lib64/spotify-client` |
| **Flatpak** (system or user) | Any distro – Fedora, openSUSE, Bazzite, SteamOS… | `…/flatpak/app/com.spotify.Client/…` |

**Snap isn't supported** – Snap packages are read-only, so Spicetify can't modify them. If you have it, SpiceFix tells you how to switch:

```sh
sudo snap remove spotify
flatpak install flathub com.spotify.Client
```

Open Spotify once and log in before running SpiceFix – Spicetify needs the settings file Spotify creates on first launch.

## Build it yourself

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o SpiceFix.exe
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w" -o SpiceFix-linux-amd64
CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags "-s -w" -o SpiceFix-linux-arm64
```

Publishing a release on GitHub (or pushing a `v*` tag) builds all three on GitHub Actions and attaches them to the release.

## Disclaimer

Not affiliated with Spicetify or Spotify. SpiceFix only runs Spicetify's own official installers and commands.
