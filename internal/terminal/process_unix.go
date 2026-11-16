//go:build !windows

package terminal

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// procPidIsZombie reports whether the process with pid is a zombie awaiting
// reap. /proc/<pid>/stat has the comm field in parentheses which may contain
// spaces, so the state field is located after the last closing parenthesis.
func procPidIsZombie(pid string) bool {
	if pid == "" {
		return false
	}
	data, err := os.ReadFile("/proc/" + pid + "/stat")
	if err != nil {
		// Unreadable: treat as still running (conservative — prompts before close).
		return false
	}
	s := string(data)
	i := strings.LastIndex(s, ")")
	if i < 0 || i+2 >= len(s) {
		return false
	}
	fields := strings.Fields(s[i+2:])
	return len(fields) > 0 && fields[0] == "Z"
}

func hasRunningProcess(ptyFile *os.File, shellPid int) bool {
	if shellPid <= 0 {
		return false
	}

	// 1. Check Linux /proc/<pid>/task/<pid>/children if available
	// Zombies awaiting reap are not running and must not count.
	childrenPath := fmt.Sprintf("/proc/%d/task/%d/children", shellPid, shellPid)
	if data, err := os.ReadFile(childrenPath); err == nil {
		for _, childPid := range strings.Fields(string(data)) {
			if !procPidIsZombie(childPid) {
				return true
			}
		}
	}
	// 2. Check terminal foreground process group
	if ptyFile != nil {
		var fgPgid int32
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, ptyFile.Fd(), uintptr(syscall.TIOCGPGRP), uintptr(unsafe.Pointer(&fgPgid)))
		if errno == 0 && fgPgid > 0 {
			shellPgid, err := syscall.Getpgid(shellPid)
			if err == nil && int(fgPgid) != shellPgid {
				// A zombie that finished but was not reaped must not count as running.
				if !procPidIsZombie(fmt.Sprintf("%d", fgPgid)) {
					return true
				}
			}
		}
	}

	return false
}
