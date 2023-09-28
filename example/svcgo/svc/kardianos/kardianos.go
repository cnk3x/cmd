package kardianos

import (
	"context"
	"fmt"
	"time"

	"github.com/cnk3x/cmd/example/svcgo/svc"
	"github.com/kardianos/service"
)

func New(runner svc.ServiceRunner, cfg svc.Config, installArgs ...string) svc.ServiceManage {
	s, e := service.New(newKardianosProgram(runner), &service.Config{
		Name:             cfg.Name,
		DisplayName:      cfg.Label,
		Description:      cfg.Description,
		Dependencies:     cfg.Dependencies,
		WorkingDirectory: cfg.Workdir,
		EnvVars:          cfg.Env,
		Arguments:        installArgs,
	})

	return func(command string) (status string, err error) {
		if e != nil {
			err = e
			return
		}

		switch command {
		case service.ControlAction[0]:
			err = s.Start()
		case service.ControlAction[1]:
			err = s.Stop()
		case service.ControlAction[2]:
			err = s.Restart()
		case service.ControlAction[3]:
			err = s.Install()
		case service.ControlAction[4]:
			err = s.Uninstall()
		case "status":
			_, err = s.Status()
		case "run":
			err = s.Run()
		default:
			err = svc.ErrHelp
		}

		return
	}
}

func newKardianosProgram(runner svc.ServiceRunner) service.Interface {
	return &kardianosProgram{runner: runner}
}

type kardianosProgram struct {
	runner svc.ServiceRunner
	cancel context.CancelFunc
	done   <-chan struct{}
}

func (p *kardianosProgram) Start(s service.Service) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	if p.runner != nil {
		p.done, err = p.runner.Run(ctx)
	}
	return
}

func (p *kardianosProgram) Stop(s service.Service) error {
	p.cancel()
	if p.done != nil {
		select {
		case <-time.After(time.Second * 3):
			return fmt.Errorf("stop timeout")
		case <-p.done:
		}
	}
	return nil
}
