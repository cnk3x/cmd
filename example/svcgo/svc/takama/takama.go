package takama

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/cnk3x/cmd/example/svcgo/svc"
	"github.com/takama/daemon"
)

func New(runner svc.ServiceRunner, cfg svc.Config, installArgs ...string) svc.ServiceManage {
	s, e := daemon.New(cfg.Name, cfg.Description, daemon.SystemDaemon, cfg.Dependencies...)

	return func(command string) (status string, err error) {
		if e != nil {
			err = e
			return
		}

		switch command {
		case "start":
			status, err = s.Start()
		case "stop":
			status, err = s.Stop()
		case "restart":
			if _, err = s.Stop(); err == nil {
				status, err = s.Start()
			}
		case "install":
			if status, err = s.Install(installArgs...); err == nil {
				_, err = s.Start()
			}
		case "uninstall":
			status, err = s.Remove()
		case "status":
			status, err = s.Status()
		case "run":
			if err = os.Chdir(cfg.Workdir); err == nil {
				status, err = s.Run(newTakamaProgram(runner))
			}
		default:
			err = svc.ErrHelp
		}

		if status != "" {
			status = strings.ReplaceAll(status, "\t\t\t\t\t", " ")
		}

		return
	}
}

func newTakamaProgram(runner svc.ServiceRunner) daemon.Executable {
	return &takamaProgram{runner: runner}
}

type takamaProgram struct {
	runner svc.ServiceRunner
	cancel context.CancelFunc
	done   <-chan struct{}
}

func (p *takamaProgram) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	if p.runner != nil {
		p.done, _ = p.runner.Run(ctx)
	}
	return
}

func (p *takamaProgram) Stop() {
	p.cancel()
	if p.done != nil {
		select {
		case <-time.After(time.Second * 3):
		case <-p.done:
		}
	}
	return
}

func (p *takamaProgram) Run() {
	if p.done != nil {
		<-p.done
	}
}
