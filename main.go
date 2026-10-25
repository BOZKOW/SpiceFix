// SpiceFix - install, repair and update Spicetify on Windows and Linux.
//
// Build:
//
//	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o SpiceFix.exe
//	GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w" -o SpiceFix-linux-amd64
//
// Platform-specific parts live in platform_windows.go and platform_linux.go.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	latestReleaseAPI = "https://api.github.com/repos/spicetify/cli/releases/latest"
	repoRaw          = "https://raw.githubusercontent.com/spicetify"
)

var in = bufio.NewReader(os.Stdin)

func main() {
	enableANSI()
	banner()

	if !checkSpotify() {
		pause()
		return
	}

	for {
		exe := findSpicetify()
		current := ""
		if exe != "" {
			current = spicetifyVersion(exe)
		}
		latest := latestVersion()

		fmt.Println()
		if exe == "" {
			info("Spicetify: " + red("not installed"))
		} else {
			v := current
			if v == "" {
				v = "unknown version"
			}
			info("Spicetify: " + green("installed") + " (" + v + ")  →  " + dim(exe))
		}
		if latest != "" {
			info("Latest release: " + latest)
		}

		fmt.Println()
		fmt.Println("  [1] Auto  – do whatever is needed (recommended)")
		fmt.Println("  [2] Install / reinstall Spicetify")
		fmt.Println("  [3] Fix  – re-apply after a Spotify update (blank / vanilla Spotify)")
		fmt.Println("  [4] Update Spicetify to the latest version")
		fmt.Println("  [5] Install / repair Spicetify Marketplace")
		fmt.Println("  [6] Deep fix – wipe backup and re-apply from scratch")
		fmt.Println("  [0] Exit")
		choice := ask("\nChoose an option", "1")

		var err error
		switch choice {
		case "1":
			err = auto(exe, current, latest)
		case "2":
			err = install()
		case "3":
			err = fix(exe)
		case "4":
			err = update(exe)
		case "5":
			err = installMarketplace()
		case "6":
			err = deepFix(exe)
		case "0", "q", "exit":
			return
		default:
			warn("Unknown option.")
			continue
		}

		fmt.Println()
		if err != nil {
			fail(err.Error())
			fmt.Println(dim("  Tip: try option [6] Deep fix, or reinstall Spotify and run [2]."))
		} else {
			ok("Done! Spotify should now open with Spicetify.")
		}
		if strings.ToLower(ask("\nBack to menu? (y/n)", "n")) != "y" {
			return
		}
	}
}

// ---------- actions ----------

func auto(exe, current, latest string) error {
	if exe == "" {
		step("Spicetify isn't installed – installing it.")
		return install()
	}
	if latest != "" && current != "" && normalize(current) != normalize(latest) {
		step(fmt.Sprintf("Update available (%s → %s) – updating.", current, latest))
		return update(exe)
	}
	step("Spicetify is up to date – re-applying to make sure it works.")
	return fix(exe)
}

func install() error {
	closeSpotify()
	step("Running the official Spicetify installer…")
	fmt.Println(dim("  (It may ask whether to install Marketplace – answer Y or N.)"))
	if err := runInstaller("cli/main/install"); err != nil {
		return fmt.Errorf("installer failed: %w", err)
	}
	exe := findSpicetify()
	if exe == "" {
		return errors.New("installer finished but the spicetify program was not found")
	}
	return fix(exe)
}

func fix(exe string) error {
	if exe == "" {
		return errors.New("Spicetify is not installed – choose [2] first")
	}
	closeSpotify()
	if err := prepareApply(exe); err != nil {
		return err
	}
	step("Restoring backup and re-applying Spicetify…")
	if err := run(exe, "restore", "backup", "apply"); err == nil {
		return nil
	}
	warn("Normal re-apply failed – trying a fresh backup.")
	if err := run(exe, "backup", "apply"); err == nil {
		return nil
	}
	return deepFix(exe)
}

func deepFix(exe string) error {
	if exe == "" {
		return errors.New("Spicetify is not installed – choose [2] first")
	}
	closeSpotify()
	if err := prepareApply(exe); err != nil {
		return err
	}
	step("Deep fix: clearing old backup…")
	_ = run(exe, "restore")
	_ = runInput(exe, "y\n", "clear")
	step("Creating a fresh backup and applying…")
	if err := run(exe, "backup", "apply"); err != nil {
		return fmt.Errorf("deep fix failed: %w", err)
	}
	return nil
}

func update(exe string) error {
	if exe == "" {
		step("Spicetify isn't installed – installing the latest version instead.")
		return install()
	}
	if managedByPackage(exe) {
		warn("Spicetify was installed by your package manager (" + exe + ").")
		fmt.Println("  Update it the same way (e.g. 'yay -Syu spicetify-cli'), then choose [3] Fix.")
		return fix(exe)
	}
	closeSpotify()
	step("Updating Spicetify CLI…")
	if err := run(exe, "upgrade"); err != nil {
		warn("'spicetify upgrade' failed – reinstalling with the official installer.")
		if err := runInstaller("cli/main/install"); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		exe = findSpicetify()
	}
	return fix(exe)
}

func installMarketplace() error {
	exe := findSpicetify()
	if exe == "" {
		return errors.New("Spicetify is not installed – choose [2] first")
	}
	closeSpotify()
	if err := prepareApply(exe); err != nil {
		return err
	}
	step("Installing Spicetify Marketplace…")
	if err := runInstaller("marketplace/main/resources/install"); err != nil {
		return fmt.Errorf("marketplace install failed: %w", err)
	}
	return nil
}

// ---------- shared helpers ----------

func spicetifyVersion(exe string) string {
	out, err := exec.Command(exe, "-v").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func latestVersion() string {
	c := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET", latestReleaseAPI, nil)
	req.Header.Set("User-Agent", "SpiceFix")
	resp, err := c.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var r struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&r) != nil {
		return ""
	}
	return r.TagName
}

func normalize(v string) string { return strings.TrimPrefix(strings.TrimSpace(v), "v") }

func run(exe string, args ...string) error { return runInput(exe, "", args...) }

func runInput(exe, input string, args ...string) error {
	fmt.Println(dim("  > spicetify " + strings.Join(args, " ")))
	cmd := exec.Command(exe, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	} else {
		cmd.Stdin = os.Stdin
	}
	return cmd.Run()
}

// passthrough runs a command attached to the terminal.
func passthrough(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}

// ---------- UI helpers ----------

func banner() {
	fmt.Println(green(`
   ____        _          _____ _
  / ___| _ __ (_) ___ ___|  ___(_)_  __
  \___ \| '_ \| |/ __/ _ \ |_  | \ \/ /
   ___) | |_) | | (_|  __/  _| | |>  <
  |____/| .__/|_|\___\___|_|   |_/_/\_\
        |_|   install · fix · update Spicetify`))
}

func ask(prompt, def string) string {
	fmt.Printf("%s [%s]: ", prompt, def)
	s, _ := in.ReadString('\n')
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}

func confirm(prompt string) bool {
	return strings.ToLower(ask(prompt+" (y/n)", "y")) == "y"
}

func pause() {
	fmt.Print("\nPress Enter to exit…")
	in.ReadString('\n')
}

func info(s string) { fmt.Println("  " + s) }
func step(s string) { fmt.Println("\n" + cyan("» ") + s) }
func ok(s string)   { fmt.Println(green("✔ ") + s) }
func warn(s string) { fmt.Println(yellow("! ") + s) }
func fail(s string) { fmt.Println(red("✘ ") + s) }

func green(s string) string  { return "\033[32m" + s + "\033[0m" }
func red(s string) string    { return "\033[31m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func cyan(s string) string   { return "\033[36m" + s + "\033[0m" }
func dim(s string) string    { return "\033[90m" + s + "\033[0m" }
