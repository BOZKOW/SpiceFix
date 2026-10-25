//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// spotifyInstall describes the Spotify client SpiceFix found.
type spotifyInstall struct {
	Kind  string // "deb/rpm/AUR", "Flatpak (system)", "Flatpak (user)"
	Dir   string // folder containing the Apps directory
	Prefs string // Spotify's prefs file
}

var spotify spotifyInstall

func enableANSI() {}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func writable(p string) bool { return syscall.Access(p, 2 /* W_OK */) == nil }

func checkSpotify() bool {
	if os.Geteuid() == 0 {
		fail("Don't run SpiceFix with sudo / as root.")
		fmt.Println("  Spicetify must be set up for your normal user. Run it without sudo –")
		fmt.Println("  SpiceFix will ask for your password only when it needs it.")
		return false
	}

	h := home()
	flatpakSub := filepath.Join("app", "com.spotify.Client", "current", "active", "files", "extra", "share", "spotify")
	candidates := []spotifyInstall{
		{"deb / AUR", "/opt/spotify", filepath.Join(h, ".config", "spotify", "prefs")},
		{"package", "/usr/share/spotify", filepath.Join(h, ".config", "spotify", "prefs")},
		{"package", "/usr/lib/spotify", filepath.Join(h, ".config", "spotify", "prefs")},
		{"rpm (negativo17)", "/usr/lib64/spotify-client", filepath.Join(h, ".config", "spotify", "prefs")},
		{"Flatpak (system)", filepath.Join("/var/lib/flatpak", flatpakSub), filepath.Join(h, ".var", "app", "com.spotify.Client", "config", "spotify", "prefs")},
		{"Flatpak (user)", filepath.Join(h, ".local", "share", "flatpak", flatpakSub), filepath.Join(h, ".var", "app", "com.spotify.Client", "config", "spotify", "prefs")},
	}
	for _, c := range candidates {
		if exists(filepath.Join(c.Dir, "Apps")) {
			// Resolve Flatpak's "current"/"active" symlinks to the real folder.
			if real, err := filepath.EvalSymlinks(c.Dir); err == nil {
				c.Dir = real
			}
			spotify = c
			break
		}
	}

	if spotify.Dir == "" {
		if exists("/snap/spotify") {
			fail("You have the Snap version of Spotify, which Spicetify can't modify (it's read-only).")
			fmt.Println("  Remove it:   sudo snap remove spotify")
			fmt.Println("  Then install the Flatpak or your distro's package, e.g.:")
			fmt.Println("    flatpak install flathub com.spotify.Client")
			fmt.Println("  Open Spotify once, log in, and run SpiceFix again.")
			return false
		}
		fail("Spotify wasn't found.")
		fmt.Println("  Install it, open it once and log in, then run SpiceFix again:")
		fmt.Println("    Ubuntu/Debian/Mint:  see https://www.spotify.com/download/linux/")
		fmt.Println("    Arch / Manjaro:      yay -S spotify")
		fmt.Println("    Fedora / any distro: flatpak install flathub com.spotify.Client")
		return false
	}

	info("Spotify: " + green("found") + " – " + spotify.Kind + "  →  " + dim(spotify.Dir))
	if !exists(spotify.Prefs) {
		warn("Spotify's settings file doesn't exist yet (" + spotify.Prefs + ").")
		fmt.Println("  Open Spotify once and log in, then close it – Spicetify needs that file.")
		return confirm("  Continue anyway?")
	}
	return true
}

func findSpicetify() string {
	h := home()
	for _, c := range []string{
		filepath.Join(h, ".spicetify", "spicetify"),
		filepath.Join(h, ".local", "bin", "spicetify"),
	} {
		if exists(c) {
			return c
		}
	}
	if p, err := exec.LookPath("spicetify"); err == nil {
		return p
	}
	return ""
}

// managedByPackage reports whether spicetify came from a distro package (e.g. AUR).
func managedByPackage(exe string) bool {
	return strings.HasPrefix(exe, "/usr/") || strings.HasPrefix(exe, "/opt/")
}

func closeSpotify() {
	_ = exec.Command("pkill", "-x", "spotify").Run()
	if exists("/usr/bin/flatpak") {
		_ = exec.Command("flatpak", "kill", "com.spotify.Client").Run()
	}
	time.Sleep(800 * time.Millisecond)
}

// prepareApply makes Spotify's folder writable and points Spicetify at it.
// Spotify updates reset these permissions, so this runs before every apply.
func prepareApply(exe string) error {
	if spotify.Dir == "" {
		return nil
	}
	apps := filepath.Join(spotify.Dir, "Apps")
	if !writable(spotify.Dir) || !writable(apps) {
		step("Spicetify needs write access to Spotify's folder (" + spotify.Dir + ").")
		fmt.Println(dim("  This runs: sudo chmod a+wr " + spotify.Dir + " && sudo chmod -R a+wr " + apps))
		if !confirm("  Allow it? You may be asked for your password.") {
			return errors.New("can't modify Spotify without write access to its folder")
		}
		if err := passthrough("sudo", "chmod", "a+wr", spotify.Dir); err != nil {
			return fmt.Errorf("couldn't change permissions: %w", err)
		}
		if err := passthrough("sudo", "chmod", "-R", "a+wr", apps); err != nil {
			return fmt.Errorf("couldn't change permissions: %w", err)
		}
	}

	step("Pointing Spicetify at your Spotify install…")
	args := []string{"config", "spotify_path", spotify.Dir}
	if exists(spotify.Prefs) {
		args = append(args, "prefs_path", spotify.Prefs)
	}
	return run(exe, args...)
}

// runInstaller runs <repoRaw>/<path>.sh from Spicetify's GitHub.
func runInstaller(path string) error {
	url := repoRaw + "/" + path + ".sh"
	var fetch string
	switch {
	case lookPath("curl"):
		fetch = "curl -fsSL '" + url + "'"
	case lookPath("wget"):
		fetch = "wget -qO- '" + url + "'"
	default:
		return errors.New("neither curl nor wget is installed – install curl (e.g. 'sudo apt install curl') and try again")
	}
	if err := passthrough("sh", "-c", fetch+" | sh"); err != nil {
		return err
	}
	ensurePath()
	return nil
}

func lookPath(name string) bool { _, err := exec.LookPath(name); return err == nil }

// ensurePath tells the user if ~/.spicetify isn't on their PATH yet.
func ensurePath() {
	dir := filepath.Join(home(), ".spicetify")
	if strings.Contains(os.Getenv("PATH"), dir) || !exists(dir) {
		return
	}
	fmt.Println(dim("  Note: to use 'spicetify' in your terminal, open a new terminal window"))
	fmt.Println(dim("  (the installer added " + dir + " to your shell's PATH)."))
}
