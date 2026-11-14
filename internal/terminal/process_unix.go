//go:build !windows

package terminal

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func hasRunningProcess(ptyFile *os.File, shellPid int) bool {
	if shellPid <= 0 {
		return false
	}

	// 1. Check Linux /proc/<pid>/task/<pid>/children if available
	childrenPath := fmt.Sprintf("/proc/%d/task/%d/children", shellPid, shellPid)
	if data, err := os.ReadFile(childrenPath); err == nil {
		if len(strings.TrimSpace(string(data))) > 0 {
			return true
		}
	}

	// 2. Check terminal foreground process group
	if ptyFile != nil {
		var fgPgid int32
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, ptyFile.Fd(), uintptr(syscall.TIOCGPGRP), uintptr(unsafe.Pointer(&fgPgid)))
		if errno == 0 && fgPgid > 0 {
			shellPgid, err := syscall.Getpgid(shellPid)
			if err == nil && int(fgPgid) != shellPgid {
				return true
			}
		}
	}

	return false
}
