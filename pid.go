package cmd

import (
	"os"
	"strconv"
)

type Pid string

func (f Pid) ReadPid() (pid int) {
	if f != "" {
		data, _ := os.ReadFile(string(f))
		pid, _ = strconv.Atoi(string(data))
	}
	return
}

func (f Pid) WritePid(pid int) Pid {
	if f != "" {
		os.WriteFile(string(f), []byte(strconv.Itoa(pid)), 0666)
	}
	return f
}

func (f Pid) DelPid() {
	if f != "" {
		os.Remove(string(f))
	}
}

func (f *Pid) SetPid(path string) {
	*f = Pid(path)
}
