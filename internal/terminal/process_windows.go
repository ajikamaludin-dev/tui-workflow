//go:build windows

package terminal

import "os"

func hasRunningProcess(ptyFile *os.File, shellPid int) bool {
	return false
}
