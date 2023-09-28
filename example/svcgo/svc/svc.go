package svc

import (
	"context"
	"flag"
)

var ErrHelp = flag.ErrHelp

type Config struct {
	Name         string            `json:"name,omitempty" `
	Label        string            `json:"label,omitempty" `
	Workdir      string            `json:"workdir,omitempty" `
	Env          map[string]string `json:"env,omitempty" `
	Description  string            `json:"description,omitempty" `
	Dependencies []string          `json:"dependencies,omitempty" `
}

type (
	ServiceManage  func(command string) (status string, err error)
	ServiceRunner  func(ctx context.Context) (done <-chan struct{}, err error)
	ServiceFactory func(runner ServiceRunner, cfg Config, installArgs ...string) ServiceManage
)

func (fr ServiceRunner) Run(ctx context.Context) (done <-chan struct{}, err error) {
	return fr(ctx)
}

func (fm ServiceManage) Manage(command string) (status string, err error) {
	return fm(command)
}
