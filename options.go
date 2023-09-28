package cmd

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Option interface{ Apply(cmd *exec.Cmd) error }
type FOption func(cmd *exec.Cmd)
type FOptionEx func(cmd *exec.Cmd) error
type Options []Option

func (f FOption) Apply(cmd *exec.Cmd) error   { f(cmd); return nil }
func (f FOptionEx) Apply(cmd *exec.Cmd) error { return f(cmd) }
func (options Options) Apply(cmd *exec.Cmd) {
	for _, option := range options {
		if cmd.Err != nil {
			return
		}
		option.Apply(cmd)
	}
}

func User(uid, pid uint32) Option {
	return FOption(func(c *exec.Cmd) { setUser(c.SysProcAttr, uid, pid) })
}

func Envs(envs []string) Option {
	return FOption(func(c *exec.Cmd) { c.Env = envs })
}

func WorkDir(workDir string) Option {
	return FOption(func(c *exec.Cmd) { c.Dir = workDir })
}

func (c *Cmd) PreExit(task func(c *Cmd), parallel ...bool) *Cmd {
	c.postStart.Append(func() { task(c) }, parallel...)
	return c
}

func (c *Cmd) PostStart(task func(c *Cmd), parallel ...bool) *Cmd {
	c.postStart.Append(func() { task(c) }, parallel...)
	return c
}

func (c *Cmd) PidFile(pidfile string) *Cmd {
	c.pid = Pid(pidfile)
	return c
}

func (c *Cmd) Logger(options LoggerOptions) *Cmd {
	if options.Path == "std" {
		options.Std = true
		options.Path = ""
	}

	create := func(std io.WriteCloser, path string) io.Writer {
		var w io.Writer
		if options.Std {
			w = std
		}

		if path != "" {
			options.Path = path
			rotate := Rotate(options)

			if w != nil {
				w = io.MultiWriter(w, rotate)
			} else {
				w = rotate
			}
			c.preExit.Parallel(WrapClose(rotate))
		}

		return w
	}

	return c.With(FOption(func(cmd *exec.Cmd) {
		if options.Path != "" {
			ext := filepath.Ext(options.Path)
			outPath := options.Path
			errPath := strings.TrimSuffix(options.Path, ext) + "-err" + ext

			cmd.Stdout = create(os.Stdout, outPath)
			cmd.Stderr = create(os.Stderr, errPath)
		}
	}))
}

func (c *Cmd) Standard() *Cmd {
	return c.With(FOption(func(c *exec.Cmd) {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}))
}

func (c *Cmd) Stderr(w io.WriteCloser) *Cmd {
	c.preExit.Parallel(WrapClose(w))
	return c.With(FOption(func(cmd *exec.Cmd) {
		cmd.Stderr = w
	}))
}

func (c *Cmd) Stdout(w io.WriteCloser) *Cmd {
	c.preExit.Parallel(WrapClose(w))
	return c.With(FOption(func(cmd *exec.Cmd) {
		cmd.Stdout = os.Stdout
	}))
}

func (c *Cmd) LoggerWriter(w io.WriteCloser) *Cmd {
	c.preExit.Append(WrapClose(w))
	return c.With(FOption(func(cmd *exec.Cmd) {
		cmd.Stderr = w
		cmd.Stdout = w
	}))
}

func (c *Cmd) LineRead(lineRd func(flag, line string), transformers ...func(io.Reader) io.Reader) *Cmd {
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

	return c.With(FOptionEx(func(cmd *exec.Cmd) error {
		cmd.Stdout = nil
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		for _, transformer := range transformers {
			stdout = io.NopCloser(transformer(stdout))
		}
		c.postStart.Parallel(startLineRead(stdout, func(s string) { lineRd("-", s) }))

		cmd.Stderr = nil
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}
		for _, transformer := range transformers {
			stderr = io.NopCloser(transformer(stderr))
		}
		c.postStart.Parallel(startLineRead(stderr, func(s string) { lineRd("-", s) }))
		return nil
	}))
}

func WrapClose(closers ...io.Closer) func() {
	return func() {
		for _, closer := range closers {
			closer.Close()
		}
	}
}
