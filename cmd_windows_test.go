package cmd

import (
	"context"
	"log"
	"testing"
	"time"
)

var onStateChange = func(s State) {
	log.Printf("[state] %s, startTime: %s, uptime: %s", s.Status, s.StartTime.Format(time.RFC3339), time.Since(s.StartTime))
}

func TestWindowsCmd(t *testing.T) {
	c := Shell(`ls -lah && id -u && sleep 10s`).
		With(Stderr(func(s string) { log.Printf("[%s] %s", "stderr", s) })).
		With(Stdout(func(s string) { log.Printf("[%s] %s", "stdout", s) })).
		With(Error(func(err error) { log.Printf("[%s] %v", "error", err) })).
		With(User(1000, 1000)).
		With(StateChange(onStateChange)).
		Start(context.Background())

	go func() {
		<-time.After(time.Second)
		c.Stop()
	}()

	<-c.Done()
	log.Printf("[done]")
}
