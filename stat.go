package cmd

import "time"

type State struct {
	PID       int           `json:"pid,omitempty" yaml:"pid,omitempty"`               //进程ID
	Status    Status        `json:"status,omitempty" yaml:"status,omitempty"`         //状态
	StartTime time.Time     `json:"start_time,omitempty" yaml:"start_time,omitempty"` //启动时间
	ExitTime  time.Time     `json:"exit_time,omitempty" yaml:"exit_time,omitempty"`   //退出时间
	Exited    bool          `json:"exited,omitempty" yaml:"exited,omitempty"`         //已退出
	ExitCode  int           `json:"exit_code,omitempty" yaml:"exit_code,omitempty"`   //退出代码
	Error     string        `json:"error,omitempty" yaml:"error,omitempty"`           //错误消息
	UserTime  time.Duration `json:"user_time,omitempty" yaml:"user_time,omitempty"`
	SysTime   time.Duration `json:"sys_time,omitempty" yaml:"sys_time,omitempty"`
	Success   bool          `json:"success,omitempty" yaml:"success,omitempty"`
}

type Status string

const (
	StatusStarting Status = "starting"
	StatusStarted  Status = "started"
	StatusStopping Status = "stopping"
	StatusStopped  Status = "stopped"
)

func (s State) Clone() (n State) {
	return State{
		PID:       s.PID,
		Status:    s.Status,
		StartTime: s.StartTime,
		ExitTime:  s.ExitTime,
		Exited:    s.Exited,
		ExitCode:  s.ExitCode,
		Error:     s.Error,
		UserTime:  s.UserTime,
		SysTime:   s.SysTime,
		Success:   s.Success,
	}
}
