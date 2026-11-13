//go:build windows

package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// SpiceFix installs Spicetify itself on Windows instead of piping the official
// PowerShell script into "iex". Running downloaded scripts that way is what
// antivirus programs flag as trojan-like behaviour.
const nativeInstaller = true

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
)

func enableANSI() {
	kernel32.NewProc("SetConsoleOutputCP").Call(65001) // UTF-8 so ✔ / → render
	h, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	var mode uint32
	kernel32.NewProc("GetConsoleMode").Call(uintptr(h), uintptr(unsafe.Pointer(&mode)))
	kernel32.NewProc("SetConsoleMode").Call(uintptr(h), uintptr(mode|0x0004))
}

func checkSpotify() bool {
	// Microsoft Store Spotify isn't supported by Spicetify.
	store, _ := filepath.Glob(filepath.Join(os.Getenv("LOCALAPPDATA"), "Packages", "SpotifyAB.SpotifyMusic_*"))
	if len(store) > 0 {
		fail("You have the Microsoft Store version of Spotify, which Spicetify can't modify.")
		fmt.Println("  Uninstall it, install Spotify from https://www.spotify.com/download/windows/,")
		fmt.Println("  open it once and log in, then run this app again.")
		return false
	}
	spotify := filepath.Join(os.Getenv("APPDATA"), "Spotify", "Spotify.exe")
	if _, err := os.Stat(spotify); err != nil {
		warn("Spotify desktop wasn't found at " + spotify)
		fmt.Println("  Install it from https://www.spotify.com/download/windows/ and open it once.")
		return confirm("  Continue anyway?")
	}
	info("Spotify: " + green("found") + "  →  " + dim(spotify))
	return true
}

func installDir() string { return filepath.Join(os.Getenv("LOCALAPPDATA"), "spicetify") }

func findSpicetify() string {
	for _, c := range []string{
		filepath.Join(installDir(), "spicetify.exe"),
		filepath.Join(os.Getenv("USERPROFILE"), "spicetify-cli", "spicetify.exe"),
	} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if p, err := exec.LookPath("spicetify"); err == nil {
		return p
	}
	return ""
}

func managedByPackage(exe string) bool {
	// Scoop / winget installs should be updated with their own tool.
	l := strings.ToLower(exe)
	return strings.Contains(l, `\scoop\`) || strings.Contains(l, `\winget\`)
}

func closeSpotify() {
	_ = exec.Command("taskkill", "/F", "/IM", "Spotify.exe").Run()
	time.Sleep(800 * time.Millisecond)
}

func prepareApply(string) error { return nil }

// ---------- Spicetify CLI ----------

func installCLI() error {
	version := normalize(latestVersion())
	if version == "" {
		version = normalize(latestTagFromRedirect("https://github.com/spicetify/cli/releases/latest"))
	}
	if version == "" {
		return errors.New("couldn't find the latest Spicetify version – check your internet connection")
	}

	arch := "x64"
	switch strings.ToUpper(os.Getenv("PROCESSOR_ARCHITEW6432") + os.Getenv("PROCESSOR_ARCHITECTURE")) {
	case "ARM64", "ARM64ARM64", "ARM64AMD64":
		arch = "arm64"
	}
	url := fmt.Sprintf("https://github.com/spicetify/cli/releases/download/v%s/spicetify-%s-windows-%s.zip", version, version, arch)

	fmt.Println(dim("  Downloading Spicetify " + version + " (" + arch + ")…"))
	data, err := download(url)
	if err != nil {
		return err
	}
	dir := installDir()
	fmt.Println(dim("  Extracting to " + dir))
	if err := extractZip(data, dir, false); err != nil {
		return err
	}
	if err := addToUserPath(dir); err != nil {
		warn("Couldn't add Spicetify to your PATH (" + err.Error() + ") – SpiceFix still works.")
	}
	return nil
}

// ---------- Marketplace ----------

func installMarketplaceFiles(exe string) error {
	userdata := filepath.Join(os.Getenv("APPDATA"), "spicetify")
	if out, err := exec.Command(exe, "path", "userdata").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			userdata = p
		}
	}

	fmt.Println(dim("  Downloading Marketplace…"))
	data, err := download("https://github.com/spicetify/marketplace/releases/latest/download/marketplace.zip")
	if err != nil {
		return err
	}
	dest := filepath.Join(userdata, "CustomApps", "marketplace")
	_ = os.RemoveAll(dest)
	if err := extractZip(data, dest, true); err != nil {
		return err
	}

	_ = runQuiet(exe, "config", "custom_apps", "spicetify-marketplace-") // remove old name
	if err := run(exe, "config", "custom_apps", "marketplace"); err != nil {
		return err
	}
	_ = run(exe, "config", "inject_css", "1", "replace_colors", "1")

	// Marketplace themes need a placeholder theme to apply over.
	out, _ := exec.Command(exe, "config", "current_theme").Output()
	if theme := strings.TrimSpace(string(out)); theme == "" || theme == "SpicetifyDefault" {
		themeDir := filepath.Join(userdata, "Themes", "marketplace")
		if ini, err := download(repoRaw + "/marketplace/main/resources/color.ini"); err == nil {
			_ = os.MkdirAll(themeDir, 0o755)
			if os.WriteFile(filepath.Join(themeDir, "color.ini"), ini, 0o644) == nil {
				_ = run(exe, "config", "current_theme", "marketplace")
			}
		}
	}

	if err := run(exe, "apply"); err != nil {
		return run(exe, "backup", "apply")
	}
	return nil
}

// ---------- helpers ----------

func download(url string) ([]byte, error) {
	c := &http.Client{Timeout: 3 * time.Minute}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "SpiceFix")
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s (%s)", resp.Status, url)
	}
	return io.ReadAll(resp.Body)
}

// latestTagFromRedirect reads the tag from GitHub's /releases/latest redirect,
// used when the GitHub API is rate-limited.
func latestTagFromRedirect(url string) string {
	c := &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := c.Get(url)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if i := strings.LastIndex(loc, "/tag/"); i >= 0 {
		return loc[i+5:]
	}
	return ""
}

// extractZip unpacks data into dest. With stripTop, a single top-level folder
// in the archive is removed (marketplace.zip contains "marketplace-dist/").
func extractZip(data []byte, dest string, stripTop bool) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("downloaded file is not a valid zip: %w", err)
	}
	prefix := ""
	if stripTop {
		prefix = commonTopDir(zr.File)
	}
	root, _ := filepath.Abs(dest)
	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, prefix)
		if name == "" {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(name))
		if !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func commonTopDir(files []*zip.File) string {
	top := ""
	for _, f := range files {
		i := strings.Index(f.Name, "/")
		if i < 0 {
			return "" // a file at the root – nothing to strip
		}
		d := f.Name[:i+1]
		if top == "" {
			top = d
		} else if d != top {
			return ""
		}
	}
	return top
}

func writeZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	// spicetify.exe may be locked if it's running; replace via rename.
	tmp := target + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Rename(target, target+".old")
		if err := os.Rename(tmp, target); err != nil {
			os.Remove(tmp)
			return err
		}
		_ = os.Remove(target + ".old")
	}
	return nil
}

func runQuiet(exe string, args ...string) error { return exec.Command(exe, args...).Run() }

// addToUserPath appends dir to the current user's PATH (HKCU\Environment),
// the same thing the official installer does.
func addToUserPath(dir string) error {
	const (
		hkcu       = 0x80000001
		keyRead    = 0x20019
		keySetVal  = 0x0002
		regSZ      = 1
		regExpand  = 2
		errSuccess = 0
	)
	var key syscall.Handle
	sub, _ := syscall.UTF16PtrFromString("Environment")
	if r, _, _ := advapi32.NewProc("RegOpenKeyExW").Call(hkcu, uintptr(unsafe.Pointer(sub)), 0,
		keyRead|keySetVal, uintptr(unsafe.Pointer(&key))); r != errSuccess {
		return fmt.Errorf("RegOpenKeyEx error %d", r)
	}
	defer advapi32.NewProc("RegCloseKey").Call(uintptr(key))

	name, _ := syscall.UTF16PtrFromString("Path")
	var typ, size uint32
	current := ""
	if r, _, _ := advapi32.NewProc("RegQueryValueExW").Call(uintptr(key), uintptr(unsafe.Pointer(name)), 0,
		uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size))); r == errSuccess && size > 0 {
		buf := make([]uint16, size/2+1)
		advapi32.NewProc("RegQueryValueExW").Call(uintptr(key), uintptr(unsafe.Pointer(name)), 0,
			uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
		current = syscall.UTF16ToString(buf)
	} else {
		typ = regExpand
	}

	for _, p := range strings.Split(current, ";") {
		if strings.EqualFold(strings.TrimRight(strings.TrimSpace(p), `\`), strings.TrimRight(dir, `\`)) {
			return nil // already there
		}
	}
	updated := dir
	if strings.TrimSpace(current) != "" {
		updated = strings.TrimRight(current, ";") + ";" + dir
	}
	if typ != regSZ {
		typ = regExpand
	}
	val, _ := syscall.UTF16FromString(updated)
	if r, _, _ := advapi32.NewProc("RegSetValueExW").Call(uintptr(key), uintptr(unsafe.Pointer(name)), 0,
		uintptr(typ), uintptr(unsafe.Pointer(&val[0])), uintptr(len(val)*2)); r != errSuccess {
		return fmt.Errorf("RegSetValueEx error %d", r)
	}

	// Tell Explorer so new terminals pick up the change.
	env, _ := syscall.UTF16PtrFromString("Environment")
	var res uintptr
	user32.NewProc("SendMessageTimeoutW").Call(0xffff, 0x001A, 0, uintptr(unsafe.Pointer(env)), 0x0002, 3000,
		uintptr(unsafe.Pointer(&res)))
	fmt.Println(dim("  Added " + dir + " to your PATH (open a new terminal to use 'spicetify')."))
	return nil
}
