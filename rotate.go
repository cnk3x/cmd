package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func Rotate(options LoggerOptions) io.WriteCloser {
	if minSize := FileSize(1 << 20); options.MaxSize < minSize {
		options.MaxSize = minSize
	}
	if maxSize := FileSize(100 << 20); options.MaxSize > maxSize {
		options.MaxSize = maxSize
	}
	if options.Keep < 0 && options.Keep != -1 {
		options.Keep = 0
	}
	return &rotateWriter{
		Path:    options.Path,
		MaxSize: int(options.MaxSize),
		Keep:    options.Keep,
	}
}

type LoggerOptions struct {
	Std     bool     `json:"std"`
	Path    string   `json:"path"`
	MaxSize FileSize `json:"max_size" yaml:"max_size"` //默认1M，最大100M
	Keep    int      `json:"keep"`                     //0, 不保留, -1, 保留所有
}

type rotateWriter struct {
	Path    string
	MaxSize int //默认1M，最大100M
	Keep    int //0, 不保留, -1, 保留所有

	current *os.File
	size    int
	err     error
	setup   sync.Once
	mu      sync.Mutex
}

func (r *rotateWriter) Write(p []byte) (n int, err error) {
	r.setup.Do(func() { r.err = r.open() })
	r.mu.Lock()
	defer r.mu.Unlock()

	if err = r.err; err != nil {
		return
	}
	if n, err = r.current.Write(p); err != nil {
		return
	}
	r.size += n
	if r.size >= int(r.MaxSize) {
		if err = r.rotate(); err != nil {
			return
		}
	}
	return
}

func (r *rotateWriter) Close() (err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != nil {
		if err = r.current.Close(); err != nil {
			return err
		}
		r.current = nil
	}
	return
}

func (r *rotateWriter) open() (err error) {
	if err = os.MkdirAll(filepath.Dir(r.Path), 0755); err != nil {
		return
	}

	if r.current, err = os.OpenFile(r.Path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
		return
	}

	r.size = 0
	return
}

func (r *rotateWriter) rotate() (err error) {
	if err = r.current.Close(); err != nil {
		return
	}

	if r.Keep == 0 {
		if err = os.Remove(r.Path); err != nil {
			return
		}
	} else {
		var (
			dir, fname = filepath.Split(r.Path)
			ext        = filepath.Ext(fname)
			name       = strings.TrimSuffix(fname, ext)
			now        = time.Now().Format("20060102-150405")
			fpath      = filepath.Join(dir, fmt.Sprintf("%s-%s%s", name, now, ext))
		)

		if err = os.Rename(r.Path, fpath); err != nil {
			return
		}

		if r.Keep > 0 {
			matches, e := filepath.Glob(filepath.Join(dir, fmt.Sprintf("%s-*%s", name, ext)))
			if e != nil {
				err = e
				return
			}
			sort.Strings(matches)
			for i, n := range matches {
				if i < r.Keep {
					continue
				}
				_ = os.Remove(n) //删除失败不报错
			}
		}
	}

	return r.open()
}

var fileSizeUnit = []string{"K", "M", "G", "T", "P", "E"}

type FileSize float64

// mib, mb
func ParseSize(s string) (FileSize, error) {
	var i int
	var c rune
	for i, c = range s {
		if c != '.' && (c < '0' || c > '9') {
			break
		}
	}

	n, err := strconv.ParseFloat(strings.TrimSpace(s[:i]), 32)
	if err != nil {
		return 0, err
	}

	u := strings.TrimSpace(s[i:])
	if len(u) == 0 {
		return FileSize(n), nil
	}

	nu := u[:1]
	su := u[1:]

	lv := 0
	switch nu {
	case "k", "K":
		lv = 1
	case "m", "M":
		lv = 2
	case "g", "G":
		lv = 3
	case "t", "T":
		lv = 4
	case "p", "P":
		lv = 5
	case "e", "E":
		lv = 6
	case "b", "B":
		su = "b"
	}

	if strings.EqualFold(su, "ib") {
		base := 1 << (lv * 10)
		return FileSize(n * float64(base)), nil
	}

	return FileSize(n * math.Pow10(lv*3)), nil
}

func (s FileSize) String() string {
	return s.ToIB()
}

func (s FileSize) ToB() string {
	for i := len(fileSizeUnit); i > 0; i-- {
		if base := math.Pow10(i * 3); float64(s) >= base {
			return strconv.FormatFloat(float64(s)/base, 'f', -1, 32) + fileSizeUnit[i-1] + "B"
		}
	}
	return strconv.FormatUint(uint64(s), 10) + "byte"
}

func (s FileSize) ToIB() string {
	for i := len(fileSizeUnit); i > 0; i-- {
		if base := float64(int(1) << (i * 10)); float64(s) >= base {
			return strconv.FormatFloat(float64(s)/base, 'f', -1, 32) + fileSizeUnit[i-1] + "iB"
		}
	}
	return strconv.FormatUint(uint64(s), 10) + "byte"
}

func (s FileSize) MarshalJSON() ([]byte, error) {
	if uint64(s) == 0 {
		return []byte(""), nil
	}
	return json.Marshal(s.ToIB())
}

func (s *FileSize) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	return s.SetString(str)
}

func (s FileSize) MarshalYAML() (any, error) {
	if uint64(s) == 0 {
		return "", nil
	}
	return s.ToIB(), nil
}

func (s *FileSize) UnmarshalYAML(unmarshal func(any) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}
	return s.SetString(str)
}

func (s *FileSize) SetString(in string) error {
	if in != "" {
		size, err := ParseSize(in)
		if err != nil {
			return err
		}
		*s = size
	}
	return nil
}
