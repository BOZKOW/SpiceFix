//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func enableANSI() {
	k := syscall.NewLazyDLL("kernel32.dll")
	k.NewProc("SetConsoleOutputCP").Call(65001) // UTF-8 so ✔ / → render
	h, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	var mode uint32
	k.NewProc("GetConsoleMode").Call(uintptr(h), uintptr(unsafe.Pointer(&mode)))
	k.NewProc("SetConsoleMode").Call(uintptr(h), uintptr(mode|0x0004))
}

func checkSpotify() bool {
	// Microsoft Store Spotify isn't supported by Spicetify.
	out, _ := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-AppxPackage -Name SpotifyAB.SpotifyMusic).Name").Output()
	if strings.TrimSpace(string(out)) != "" {
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

func findSpicetify() string {
	for _, c := range []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "spicetify", "spicetify.exe"),
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

// runInstaller runs <repoRaw>/<path>.ps1 from Spicetify's GitHub.
func runInstaller(path string) error {
	url := repoRaw + "/" + path + ".ps1"
	return passthrough("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
		"[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; iwr -useb '"+url+"' | iex")
}
