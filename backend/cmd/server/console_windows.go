package main

import (
	"fmt"
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
	title, _ := syscall.UTF16PtrFromString("FrameFlow")
	kernel32.NewProc("SetConsoleTitleW").Call(uintptr(unsafe.Pointer(title)))
	module, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	const imageIcon = 1
	icon, _, _ := user32.NewProc("LoadImageW").Call(module, 1, imageIcon, 32, 32, 0)
	if icon != 0 {
		const wmSetIcon = 0x80
		user32.NewProc("SendMessageW").Call(console, wmSetIcon, 0, icon)
		user32.NewProc("SendMessageW").Call(console, wmSetIcon, 1, icon)
	}
	const swMinimize = 6
	user32.NewProc("ShowWindow").Call(console, swMinimize)
}

func notifyStartupError(err error, logPath string) {
	message, _ := syscall.UTF16PtrFromString(fmt.Sprintf("启动失败: %v\n\n详细日志: %s", err, logPath))
	title, _ := syscall.UTF16PtrFromString("FrameFlow - 启动失败")
	const mbIconError = 0x10 | 0x10000
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), mbIconError)
}
