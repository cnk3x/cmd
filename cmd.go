package cmd

import (
	"bufio"
	"context"
	"io"
	"log"
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
func Parse(commandLine string) *Cmd {
	if args := parseCommandLine(commandLine); len(args) > 0 {
		return New(args[0], args[1:]...)
	}
	return &Cmd{}
}

// 执行脚本，在windows使用`cmd /c`, unix使用`bash/sh -c`(优先bash)
func Shell(command string, options ...Option) *Cmd {
	name, args := shell(command)
	return New(name, args...).With(options...)
}

type Cmd struct {
	Name       string     //命令名称
	Executable string     //执行文件
	Args       []string   //参数
	Env        []string   //环境变量
	Dir        string     //执行目录
	Stderr     logHandler //按行处理错误输出
	Stdout     logHandler //按行处理标准输出

	PidFile //指定PIDFile路径

	options   Options           //选项
	preStart  Options           //启动前
	postStart func(*os.Process) //启动后

	onStateChange func(s State) //onStateChange
	onError       func(error)   //错误

	done   <-chan struct{}
	cancel context.CancelFunc
	state  *State
	states []*State
	cmd    *exec.Cmd
}

// add options
func (c *Cmd) With(options ...Option) *Cmd {
	c.options = append(c.options, options...)
	return c
}

// start the process
func (c *Cmd) Start(ctx context.Context) *Cmd {
	handleStateChange := func() {
		if c.onStateChange != nil {
			c.onStateChange(c.state.Clone())
		}
	}

	c.state = &State{Status: StatusStarting, StartTime: time.Now()}
	handleStateChange()

	var w sync.WaitGroup
	ctx, c.cancel = context.WithCancel(ctx)
	cmd := exec.Command(c.Executable, c.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	setPdeathsig(cmd.SysProcAttr, syscall.SIGKILL)
	setPgid(cmd.SysProcAttr)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)
	c.cmd = cmd

	c.options.Apply(c)

	cmdDone := make(chan struct{})
	realDone := make(chan struct{})
	c.done = realDone

	handleExit := func(err error) {
		defer close(cmdDone)
		if err != nil {
			c.state.Error = err.Error()
			if c.onError != nil {
				c.onError(err)
			}
		}

		var ps *os.ProcessState
		c.state.ExitCode = cmd.ProcessState.ExitCode()
		if ps = cmd.ProcessState; ps != nil {
			c.state.UserTime = ps.UserTime()
			c.state.SysTime = ps.SystemTime()
			c.state.Success = ps.Success()
		}
		c.states = append(c.states, c.state)
		c.state.Exited = true
		c.state.ExitTime = time.Now()
		c.state.Status = StatusStopped
		handleStateChange()
	}

	var stdout, stderr io.Reader
	var err error

	if !c.Stdout.IsEmpty() {
		cmd.Stdout = nil
		if stdout, err = cmd.StdoutPipe(); err != nil {
			handleExit(err)
			return c
		}
	}

	if !c.Stderr.IsEmpty() {
		cmd.Stderr = nil
		if stderr, err = cmd.StderrPipe(); err != nil {
			handleExit(err)
			return c
		}
	}

	c.preStart.Apply(c)
	if err = cmd.Start(); err != nil {
		handleExit(err)
		return c
	}
	pid := cmd.Process.Pid

	c.state.PID = pid
	c.state.Status = StatusStarted
	handleStateChange()

	defer c.WritePid(pid).DelPid()

	if c.postStart != nil {
		c.postStart(cmd.Process)
	}

	readStd := func(std io.Reader, handle func(s string)) <-chan struct{} {
		rDone := make(chan struct{})
		w.Add(1)
		go func() {
			defer w.Done()
			defer close(rDone)
			if std != nil && handle != nil {
				for s := bufio.NewScanner(std); s.Scan(); {
					handle(s.Text())
				}
			}
		}()
		return rDone
	}

	//handleCancel
	w.Add(1)
	go func() {
		defer w.Done()

		select {
		case <-cmdDone: //已经退出了
			log.Println("[cancel] has done")
			return
		case <-ctx.Done():
			c.state.Status = StatusStopping
			handleStateChange()
			log.Printf("[cancel] terminate: %d", pid)
			terminate(pid)
		}

		//watch
		w.Add(1)
		go func() {
			defer w.Done()
			for i := 0; i < 3; i++ {
				select {
				case <-cmdDone:
					return
				case <-time.After(time.Second * 3):
					log.Printf("[cancel] kill: %d", pid)
					kill(pid)
				}
			}
		}()
	}()

	//handleWait
	w.Add(1)
	go func() {
		defer w.Done()
		oDone := readStd(stdout, c.Stdout.handle)
		eDone := readStd(stderr, c.Stderr.handle)
		<-oDone
		<-eDone
		handleExit(cmd.Wait())
	}()

	go func() {
		<-cmdDone
		w.Wait()
		close(realDone)
	}()

	return c
}

// Restart the process
func (c *Cmd) Restart(ctx context.Context) {
	c.Stop()
	<-c.done
	c.Start(ctx)
}

// Stop the process
func (c *Cmd) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *Cmd) Done() <-chan struct{} {
	return c.done
}

func (c *Cmd) State() State {
	return c.state.Clone()
}

func (c *Cmd) States() (states []State) {
	states = make([]State, len(c.states))
	for i, state := range c.states {
		states[i] = state.Clone()
	}
	return
}

func (c *Cmd) String() string {
	b := new(strings.Builder)
	b.WriteString(c.Executable)
	for _, a := range c.Args {
		b.WriteByte(' ')
		b.WriteString(a)
	}
	return b.String()
}
