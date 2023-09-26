package cmd

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
