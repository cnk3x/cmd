package cmd

import "os"

type Option interface{ Apply(c *Cmd) }
type FOption func(c *Cmd)
type Options []Option

func (options Options) Apply(c *Cmd) {
	for _, option := range options {
		option.Apply(c)
	}
}

func (f FOption) Apply(c *Cmd) { f(c) }

func User(uid, pid uint32) Option {
	return FOption(func(c *Cmd) { setUser(c.cmd.SysProcAttr, uid, pid) })
}

func Error(handle func(err error)) Option {
	return FOption(func(c *Cmd) { c.onError = handle })
}

func StateChange(handle func(s State)) Option {
	return FOption(func(c *Cmd) { c.onStateChange = handle })
}

func Stdout(handle func(string)) Option {
	return FOption(func(c *Cmd) { c.Stdout.Add("", handle) })
}

func Stderr(handle func(string)) Option {
	return FOption(func(c *Cmd) { c.Stderr.Add("", handle) })
}

func PreStart(options ...Option) Option {
	return FOption(func(c *Cmd) { c.preStart = append(c.preStart, options...) })
}

func PostStart(handle func(p *os.Process)) Option {
	return FOption(func(c *Cmd) { c.postStart = handle })
}
