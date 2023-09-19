package cmd

import (
	"os"
	"strconv"
)

type PidFile string

func (f PidFile) ReadPid() (pid int) {
	if f != "" {
		data, _ := os.ReadFile(string(f))
		pid, _ = strconv.Atoi(string(data))
	}
	return
}

func (f PidFile) WritePid(pid int) PidFile {
	if f != "" {
		os.WriteFile(string(f), []byte(strconv.Itoa(pid)), 0666)
	}
	return f
}

func (f PidFile) DelPid() PidFile {
	if f != "" {
		os.Remove(string(f))
	}
	return f
}

func (f *PidFile) SetPidFile(path string) {
	*f = PidFile(path)
}
