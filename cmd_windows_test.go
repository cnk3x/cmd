package cmd

import (
	"log"
	"testing"
	"time"

	// "golang.org/x/text/encoding/simplifiedchinese"
)

func TestWindowsCmd(t *testing.T) {
	s := Shell(`dir`).With(User(1000, 1000)).
		With(PostStart(func(c *Cmd) { t.Logf("command: %s", c.String()) })).
		With(LineRead(
			func(flag string, line string) { t.Logf("[%s] %s", flag, line) },
			// simplifiedchinese.GB18030.NewDecoder().Reader,
		)).
		Run()

	go func() {
		<-time.After(time.Second)
		s.Cancel()
	}()

	log.Printf("[done]: %v", s.Wait())
}

func TestClash(t *testing.T) {
	s := CommandLine(`D:\services\clashpt\clash-windows-amd64.exe -d D:\services\clashpt\`).
		With(WorkDir(`D:\services\clashpt\`)).
		With(User(1000, 1000)).
		With(PostStart(func(c *Cmd) { log.Printf("command1: %s", c.String()) })).
		With(Logger(Rotate(RotateOptions{Path: `D:\services\clashpt\clashpt.log`}))).
		Run()

	go func() {
		<-time.After(time.Second * 2)
		s.Cancel()
	}()

	log.Printf("[done]: %v", s.Wait())
}
