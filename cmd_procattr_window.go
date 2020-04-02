// +build windows

package cmd

import "syscall"

func newProcAttr() *syscall.SysProcAttr{
	return &syscall.SysProcAttr{HideWindow: true}
}
