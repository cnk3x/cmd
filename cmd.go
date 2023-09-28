package cmd

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

func New(name string, args ...string) *Cmd {
	return &Cmd{executable: name, args: args}
}

// 解析单行命令
func CommandLine(commandLine string) *Cmd {
	if args := Fields(commandLine); len(args) > 0 {
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
	name       string   //命令名称
	executable string   //执行文件
	args       []string //参数
	options    Options  //选项
	pid        Pid      //指定PIDFile路径
	postStart  Runner   //启动后执行
	preExit    Runner   //完成时执行

	err error
	// cmd *exec.Cmd
}

// add options
func (c *Cmd) With(options ...Option) *Cmd {
	c.options = append(c.options, options...)
	return c
}

func (c *Cmd) Run() (state *StartState) {
	return c.RunWithContext(context.Background())
}

// 启动进程
func (c *Cmd) RunWithContext(ctx context.Context) (state *StartState) {
	state = &StartState{}

	ctx, state.cancel = context.WithCancel(ctx)
	cmd := exec.Command(c.executable, c.args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	setBg(cmd.SysProcAttr)

	var (
		cmdDone = make(chan struct{})
		allDone = make(chan struct{})
	)

	c.err = nil
	// c.cmd = cmd

	state.done = allDone
	handleExit := func(err error) *StartState {
		defer close(cmdDone)
		state.Err = err
		c.err = err
		return state
	}

	var w sync.WaitGroup
	//all done
	go func() {
		<-cmdDone
		w.Wait()
		close(allDone)
	}()

	c.options.Apply(cmd)
	if cmd.Err != nil {
		return handleExit(cmd.Err)
	}

	if err := cmd.Start(); err != nil {
		return handleExit(err)
	}

	pid := cmd.Process.Pid
	state.PID = pid

	c.pid.WritePid(pid)
	c.preExit.Append(c.pid.DelPid)

	//terminate when context done
	bgRun(&w, waitTerminate(ctx, pid, cmdDone))

	//read console and wait done
	bgRun(&w, func() {
		c.postStart.Run()
		handleExit(cmd.Wait())
		c.preExit.Run()
	})

	return
}

// 进程描述
func (c *Cmd) String() string {
	b := new(strings.Builder)
	b.WriteString(c.executable)
	for _, a := range c.args {
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
