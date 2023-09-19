//go:build windows

package cmd

import (
	"os/exec"
	"strconv"
	"syscall"
)

func shell(command string) (name string, args []string) {
	return "cmd", []string{"/c", command}
}

func terminate(pid int) (err error) {
	_, err = exec.Command("taskkill", "/f", "/t", "/pid", strconv.Itoa(pid)).CombinedOutput()
	return
}

func kill(pid int) (err error) {
	_, err = exec.Command("taskkill", "/f", "/t", "/pid", strconv.Itoa(pid)).CombinedOutput()
	return
}

func setUser(*syscall.SysProcAttr, uint32, uint32) {}

func setPgid(*syscall.SysProcAttr) {}

func setPdeathsig(*syscall.SysProcAttr, syscall.Signal) {}
