package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"go.shu.run/log"
)

func New(args ...string) *C {
	return &C{ar: args}
}

type C struct {
	ar  []string
	c   *exec.Cmd
	pid int
	ls  *os.ProcessState
}

func (c *C) Command(args ...string) {
	c.ar = args
}

func (c *C) Start() (err error) {
	defer func() {
		if re := recover(); re != nil {
			err = fmt.Errorf("%v", re)
		}
	}()
	if len(c.ar) == 0 {
		return fmt.Errorf("命令为空")
	}
	log.Infof("运行: %s", strings.Join(c.ar, " "))
	c.c = exec.Command(c.ar[0], c.ar[1:]...)
	c.c.Stderr = os.Stderr
	c.c.Stdout = os.Stdout
	c.c.Stdin = os.Stdin
	c.c.SysProcAttr = newProcAttr()
	return c.c.Start()
}

func (c *C) Run() {
	defer func() {
		if re := recover(); re != nil {
			log.Errorf("%v", re)
		}
	}()

	//杀进程
	if err := c.Kill(); err != nil {
		log.Debugf("杀死进程失败: %v", err)
	}

	//等待退出
	if !c.WaitForExit() {
		log.Debugf("等待进程退出超时")
	}

	//重置状态
	c.Reset()

	//启动
	if err := c.Start(); err != nil {
		log.Errorf("执行命令失败: %v", err)
		return
	}

	//等待启动是否成功
	if c.WaitForRun() {
		log.Infof("已运行 PID: %d", c.Pid())
	}

	//等待执行完成
	if err := c.Wait(); err != nil {
		log.Infof("进程已完成: %v", err)
	} else {
		log.Infof("进程已完成")
	}
	c.ls = c.c.ProcessState
	c.c = nil
}

func (c *C) WaitForRun() bool {
	if c.pid = c.Pid(); c.pid > 0 {
		return true
	}

	timeout := time.After(time.Second * 5)
	for {
		select {
		case <-timeout:
			return false
		case <-time.After(10 * time.Millisecond):
			if c.pid = c.Pid(); c.pid > 0 {
				return true
			}
		}
	}
}

func (c *C) WaitForExit() bool {
	if c.c == nil || c.ls != nil {
		return true
	}

	timeout := time.After(time.Second * 5)
	for {
		select {
		case <-timeout:
			return false
		case <-time.After(10 * time.Millisecond):
			if c.c == nil || c.ls != nil {
				return true
			}
		}
	}
}

func (c *C) Kill(signals ...syscall.Signal) error {
	if pid := c.Pid(); pid > 0 {
		if len(signals) > 0 {
			return syscall.Kill(-pid, signals[0])
		}
		return syscall.Kill(-pid, syscall.SIGKILL)
	}
	return nil
}

func (c *C) Wait() error {
	if c.c != nil {
		return c.c.Wait()
	}
	return nil
}

func (c *C) Reset() {
	//重置状态
	c.ls = nil
	c.pid = 0
	c.c = nil
}

func (c *C) ProcessState() *os.ProcessState {
	return c.ls
}

func (c *C) Pid() int {
	if c.c != nil && c.c.Process != nil {
		return c.c.Process.Pid
	}
	return 0
}
