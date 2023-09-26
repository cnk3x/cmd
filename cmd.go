package cmd

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

func New(name string, args ...string) *Cmd {
	return &Cmd{Executable: name, Args: args}
}

// 解析单行命令
func CommandLine(commandLine string) *Cmd {
	if args := ParseCommandLine(commandLine); len(args) > 0 {
		return New(args[0], args[1:]...)
	}
	return &Cmd{}
}

// 执行脚本，在windows使用`cmd /c`, unix使用`bash/sh -c`(优先bash)
func Shell(command string) *Cmd {
	name, args := shell(command)
	return New(name, args...)
}

type Cmd struct {
	Name       string   //命令名称
	Executable string   //执行文件
	Args       []string //参数
	Env        []string //环境变量
	Dir        string   //执行目录
	PidFile    PidFile  //指定PIDFile路径
	Options    Options  //选项

	stderr func(s string)            //按行处理错误输出
	stdout func(s string)            //按行处理标准输出
	ct     func(io.Reader) io.Reader //文本转换
	ctx    context.Context

	cmd *exec.Cmd
}

// add options
func (c *Cmd) With(options ...Option) *Cmd {
	c.Options = append(c.Options, options...)
	return c
}

func (c *Cmd) Context(ctx context.Context) *Cmd {
	c.ctx = ctx
	return c
}

// 设置命令行文本转换
func (c *Cmd) Transform(ct func(io.Reader) io.Reader) *Cmd {
	c.ct = ct
	return c
}

// 设置错误行输出
func (c *Cmd) Stderr(lineRd func(s string)) *Cmd {
	c.stderr = lineRd
	return c
}

// 设置标准行输出
func (c *Cmd) Stdout(lineRd func(s string)) *Cmd {
	c.stdout = lineRd
	return c
}

// 启动进程
func (c *Cmd) Start() (state *StartState) {
	state = &StartState{}

	var ctx context.Context
	if ctx = c.ctx; ctx == nil {
		ctx = context.Background()
	}

	ctx, state.cancel = context.WithCancel(ctx)
	cmd := exec.Command(c.Executable, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	setPgid(cmd.SysProcAttr)

	c.cmd = cmd
	c.Options.Apply(c)

	var (
		cmdDone = make(chan struct{})
		allDone = make(chan struct{})
		bgr     Parallel
	)

	state.done = allDone

	//all done
	go func() {
		<-cmdDone
		bgr.Wait()
		close(allDone)
	}()

	var stdout, stderr io.Reader
	var err error

	handleExit := func(err error) *StartState {
		defer close(cmdDone)
		state.Err = err
		return state
	}

	if c.stdout != nil {
		cmd.Stdout = nil
		if stdout, err = cmd.StdoutPipe(); err != nil {
			return handleExit(err)
		}
	}

	if c.stderr != nil {
		cmd.Stderr = nil
		if stderr, err = cmd.StderrPipe(); err != nil {
			return handleExit(err)
		}
	}

	if err = cmd.Start(); err != nil {
		return handleExit(err)
	}

	pid := cmd.Process.Pid
	state.PID = pid

	c.PidFile.WritePid(pid)

	//terminate when context done
	bgr.Run(func() {
		select {
		case <-cmdDone:
			return
		case <-ctx.Done():
			Terminate(pid, cmdDone)
		}
	})

	//read console and wait done
	bgr.Run(func() {
		transformRd := func(r io.Reader) io.Reader {
			if r == nil || c.ct == nil {
				return r
			}
			return c.ct(r)
		}

		var liner Liner
		liner.Read(transformRd(stdout), c.stdout)
		liner.Read(transformRd(stderr), c.stderr)
		liner.Wait()

		handleExit(cmd.Wait())
		c.PidFile.DelPid()
	})

	return
}

// 进程描述
func (c *Cmd) String() string {
	b := new(strings.Builder)
	b.WriteString(c.Executable)
	for _, a := range c.Args {
		b.WriteByte(' ')
		b.WriteString(a)
	}
	return b.String()
}

type StartState struct {
	PID    int
	Err    error
	Status Status

	done   chan struct{}
	cancel context.CancelFunc
}

type Status string

const (
	StatusStarting Status = "starting"
	StatusStarted  Status = "started"
	StatusStopping Status = "stopping"
	StatusStopped  Status = "stopped"
)

func (s *StartState) Done() <-chan struct{} {
	return s.done
}

func (s *StartState) Cancel() {
	s.cancel()
}

func (s *StartState) Wait() error {
	<-s.done
	return s.Err
}

// 并行运行，集中等待
type Parallel struct{ wg sync.WaitGroup }

func (p *Parallel) Wait()          { p.wg.Wait() }
func (p *Parallel) Run(run func()) { p.wg.Add(1); go func() { defer p.wg.Done(); run() }() }

// 按行读取，多个异步读取，集中等待
type Liner struct{ p Parallel }

func (l *Liner) Wait() { l.p.Wait() }
func (l *Liner) Read(std io.Reader, handle func(s string)) {
	l.p.Run(func() {
		if std != nil && handle != nil {
			for s := bufio.NewScanner(std); s.Scan(); {
				handle(s.Text())
			}
		}
	})
}

func waitTerminate(ctx context.Context, pid int, done <-chan struct{}) func() {
	return func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			Terminate(pid, done)
		}
	}
}

func Terminate(pid int, done <-chan struct{}) {
	sysInterrupt(pid)

	if done != nil {
		select {
		case <-done:
			return
		case <-time.After(time.Second * 3):
			sysTerminate(pid)
		}

		select {
		case <-done:
			return
		case <-time.After(time.Second * 2):
			sysKill(pid)
		}
	}
}
