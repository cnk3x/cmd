package cmd

import (
	"bufio"
	"io"
	"os"
)

type Option interface{ Apply(c *Cmd) error }
type FOption func(c *Cmd)
type FOptionEx func(c *Cmd) error
type Options []Option

func (f FOption) Apply(c *Cmd) error   { f(c); return nil }
func (f FOptionEx) Apply(c *Cmd) error { return f(c) }
func (options Options) Apply(c *Cmd) {
	for _, option := range options {
		if c.cmd.Err != nil {
			return
		}
		option.Apply(c)
	}
}

func User(uid, pid uint32) Option {
	return FOption(func(c *Cmd) { setUser(c.cmd.SysProcAttr, uid, pid) })
}

func PidFile(pidfile string) Option {
	return FOption(func(c *Cmd) { c.Pid = Pid(pidfile) })
}

func Envs(envs []string) Option {
	return FOption(func(c *Cmd) { c.Env = envs })
}

func WorkDir(workDir string) Option {
	return FOption(func(c *Cmd) { c.Dir = workDir })
}

func PreExit(task func(c *Cmd), parallel ...bool) Option {
	return FOption(func(c *Cmd) {
		c.PreExit.Append(func() { task(c) }, parallel...)
	})
}

func PostStart(task func(c *Cmd), parallel ...bool) Option {
	return FOption(func(c *Cmd) {
		c.PostStart.Append(func() { task(c) }, parallel...)
	})
}

func Logger(w io.WriteCloser) Option {
	return FOption(func(c *Cmd) {
		c.cmd.Stderr = w
		c.cmd.Stdout = w
		c.PreExit.Append(WrapClose(w))
	})
}

func Stderr(w io.WriteCloser) Option {
	return FOption(func(c *Cmd) {
		c.cmd.Stderr = w
		c.PreExit.Append(WrapClose(w))
	})
}

func Stdout(w io.WriteCloser) Option {
	return FOption(func(c *Cmd) {
		c.cmd.Stdout = w
		c.PreExit.Append(WrapClose(w))
	})
}

var Standard = FOption(func(c *Cmd) {
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr
})

func LineRead(lineRd func(flag, line string), transformers ...func(io.Reader) io.Reader) Option {

	// 按行读取
	startLineRead := func(std io.Reader, handle func(s string)) func() {
		return func() {
			if std != nil && handle != nil {
				for s := bufio.NewScanner(std); s.Scan(); {
					handle(s.Text())
				}
			}
		}
	}

	return FOptionEx(func(c *Cmd) error {
		c.cmd.Stdout = nil
		stdout, err := c.cmd.StdoutPipe()
		if err != nil {
			return err
		}
		for _, transformer := range transformers {
			stdout = io.NopCloser(transformer(stdout))
		}
		c.PostStart.Parallel(startLineRead(stdout, func(s string) { lineRd("-", s) }))

		c.cmd.Stderr = nil
		stderr, err := c.cmd.StderrPipe()
		if err != nil {
			return err
		}
		for _, transformer := range transformers {
			stderr = io.NopCloser(transformer(stderr))
		}
		c.PostStart.Parallel(startLineRead(stderr, func(s string) { lineRd("-", s) }))
		return nil
	})
}

func WrapClose(closers ...io.Closer) func() {
	return func() {
		for _, closer := range closers {
			closer.Close()
		}
	}
}
