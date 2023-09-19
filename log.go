package cmd

import (
	"errors"
	"sync"
	"time"
)

type LogMessage struct {
	Time    time.Time
	Message string
}

type logHandler struct {
	handlers  map[string]func(string)
	histories []LogMessage
	mu        sync.Mutex
}

func (h *logHandler) IsEmpty() bool {
	return len(h.handlers) == 0
}

func (h *logHandler) Add(key string, handle func(string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handlers == nil {
		h.handlers = map[string]func(string){}
	}
	h.handlers[key] = handle
}

func (h *logHandler) Del(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handlers != nil {
		delete(h.handlers, key)
	}
}

func (h *logHandler) handle(s string) {
	h.mu.Lock()
	h.histories = append(h.histories, LogMessage{Time: time.Now(), Message: s})
	h.mu.Unlock()
	for _, handle := range h.handlers {
		handle(s)
	}
}

func (h *logHandler) Tail(n int) (out []LogMessage) {
	hl := len(h.histories)
	if l := max(nz(n, 500), hl); l > 0 {
		out = make([]LogMessage, 0, l)
		for i := hl - n; i < hl; i++ {
			out = append(out, h.histories[i])
		}
	}
	return
}

func (h *logHandler) Follow(key string, bufSize int) *LogFollow {
	f := &LogFollow{k: key, h: h, c: make(chan LogMessage, bufSize)}
	//如果超出缓存未处理的日志，将被忽略
	h.Add(key, func(s string) { cWrite(f.c, LogMessage{Time: time.Now(), Message: s}) })
	return f
}

// 日志跟随，如果不需要了，需调用Stop释放
type LogFollow struct {
	k string
	h *logHandler
	c chan LogMessage
	m []LogMessage
}

func (f *LogFollow) Stop() {
	f.h.Del(f.k)
	close(f.c)
}

func (f *LogFollow) Read() (m LogMessage, err error) {
	if m, err = cRead(f.c); err == nil {
		f.m = append(f.m, m)
	}
	return
}

var (
	ErrChannelEmpty   = errors.New("channel has no data")
	ErrChannelBlocked = errors.New("channel has blocked")
)

func cRead[T any](c <-chan T) (out T, err error) {
	select {
	case out = <-c:
	default:
		err = ErrChannelEmpty
	}
	return
}

func cWrite[T any](c chan<- T, v T) (err error) {
	select {
	case c <- v:
	default:
		err = ErrChannelBlocked
	}
	return
}

// 选择第一个不为零值的值，如果都是零，返回零值
func nz[T comparable](args ...T) (o T) {
	for _, n := range args {
		if n != o {
			return n
		}
	}
	return
}
