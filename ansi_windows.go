//go:build windows

package main

import "syscall"

func enableANSI() {
	k := syscall.NewLazyDLL("kernel32.dll")
	getMode := k.NewProc("GetConsoleMode")
	setMode := k.NewProc("SetConsoleMode")
	setCP := k.NewProc("SetConsoleOutputCP")
	setCP.Call(65001) // UTF-8 so ✔ / → render
	h, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	var mode uint32
	getMode.Call(uintptr(h), uintptr(unsafePtr(&mode)))
	setMode.Call(uintptr(h), uintptr(mode|0x0004))
}
