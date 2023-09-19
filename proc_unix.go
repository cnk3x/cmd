//go:build !windows

package cmd

import (
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func shell(command string) (name string, args []string) {
	if name, _ = exec.LookPath("bash"); name == "" {
		name, _ = exec.LookPath("sh")
	}
	return name, []string{"-c", command}
}

func terminate(pid int) (err error) {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

func kill(pid int) (err error) {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

func setUser(attr *syscall.SysProcAttr, uid uint32, gid uint32) {
	if u, err := user.Current(); err == nil {
		cu, ue := strconv.ParseUint(u.Uid, 10, 32)
		cg, ge := strconv.ParseUint(u.Gid, 10, 32)
		if ue == nil && ge == nil && uint32(cu) == uid && uint32(cg) == gid {
			return
		}
	}
	attr.Credential = &syscall.Credential{Uid: uid, Gid: gid, NoSetGroups: true}
}

func setPgid(attr *syscall.SysProcAttr) {
	attr.Setpgid = true
}

func setPdeathsig(attr *syscall.SysProcAttr, sig syscall.Signal) {
	attr.Pdeathsig = sig
}
