//go:build windows

package cmd

import (
	"os/exec"
	"strconv"
	"syscall"
)

type attr = syscall.SysProcAttr

func shell(command string) (name string, args []string) {
	return "cmd", []string{"/c", command}
}

func setUser(*attr, uint32, uint32) {}
func setPgid(*attr)                 {}

func killPid(pid int) error {
	return exec.Command("taskkill", "/f", "/t", "/pid", strconv.Itoa(pid)).Run()
}

func sysInterrupt(pid int) error { return killPid(pid) }
func sysTerminate(pid int) error { return killPid(pid) }
func sysKill(pid int) error      { return killPid(pid) }
