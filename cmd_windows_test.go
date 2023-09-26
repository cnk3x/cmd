package cmd

import (
	"log"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestWindowsCmd(t *testing.T) {
	s := Shell(`dir`).With(User(1000, 1000)).
		Stderr(func(s string) { t.Logf("[E] %s", s) }).
		Stdout(func(s string) { t.Logf("[S] %s", s) }).
		Transform(simplifiedchinese.GB18030.NewDecoder().Reader).
		Start()

	go func() {
		<-time.After(time.Second)
		s.Cancel()
	}()

	log.Printf("[done]: %v", s.Wait())
}
