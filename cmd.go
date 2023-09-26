package cmd

import (
	"context"
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
	Options    Options  //选项
	Pid                 //指定PIDFile路径

	PostStart Runner //启动后执行
	PreExit   Runner //完成时执行

	cmd *exec.Cmd
}

// add options
func (c *Cmd) With(options ...Option) *Cmd {
	c.Options = append(c.Options, options...)
	return c
}

func (c *Cmd) Run() (state *StartState) {
	return c.RunWithContext(context.Background())
}

// 启动进程
func (c *Cmd) RunWithContext(ctx context.Context) (state *StartState) {
	state = &StartState{}

	ctx, state.cancel = context.WithCancel(ctx)
	cmd := exec.Command(c.Executable, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	setBg(cmd.SysProcAttr)

	var (
		cmdDone = make(chan struct{})
		allDone = make(chan struct{})
	)

	state.done = allDone
	handleExit := func(err error) *StartState {
		defer close(cmdDone)
		state.Err = err
		return state
	}

	c.cmd = cmd
	c.Options.Apply(c)
	if cmd.Err != nil {
		return handleExit(cmd.Err)
	}

	if err := cmd.Start(); state.Err != nil {
		return handleExit(err)
	}

	pid := cmd.Process.Pid
	state.PID = pid

	c.Pid.WritePid(pid)
	c.PreExit.Append(c.Pid.DelPid)

	var w sync.WaitGroup

	//terminate when context done
	bgRun(&w, waitTerminate(ctx, pid, cmdDone))

	//read console and wait done
	bgRun(&w, func() {
		c.PostStart.Run()
		handleExit(cmd.Wait())
		c.PreExit.Run()
	})

	//all done
	go func() {
		<-cmdDone
		w.Wait()
		close(allDone)
	}()

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

	done   <-chan struct{}
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
	<-s.Done()
	return s.Err
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

func bgRun(wg *sync.WaitGroup, run func()) { wg.Add(1); go func() { run(); wg.Done() }() }
