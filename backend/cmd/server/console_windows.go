package main

import (
	"syscall"
	"unsafe"
)

func minimizeConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	processes := [2]uint32{}
	count, _, _ := kernel32.NewProc("GetConsoleProcessList").Call(uintptr(unsafe.Pointer(&processes[0])), uintptr(len(processes)))
	if count != 1 {
		return
	}
	console, _, _ := kernel32.NewProc("GetConsoleWindow").Call()
	if console == 0 {
		return
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	const swMinimize = 6
	user32.NewProc("ShowWindow").Call(console, swMinimize)
}
