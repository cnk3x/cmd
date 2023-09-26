package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cnk3x/cmd"
	"github.com/takama/daemon"
	"github.com/valyala/fasttemplate"
	"gopkg.in/yaml.v3"
)

func flagParse() (configFn, command string) {
	flag.Usage = func() {
		name := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "测试运行  %s -c path/to/config.yaml           \n", name)
		fmt.Fprintf(os.Stderr, "安装服务  %s -c path/to/config.yaml install   \n", name)
		fmt.Fprintf(os.Stderr, "卸载服务  %s -c path/to/config.yaml uninstall \n", name)
	}

	var cwd, _ = os.Getwd()
	configFn = filepath.Base(cwd) + ".yaml"
	flag.StringVar(&configFn, "c", configFn, "配置文件")
	flag.Parse()

	command = flag.Arg(0)
	configFn, _ = filepath.Abs(configFn)
	if !strings.HasSuffix(configFn, ".yaml") {
		configFn += ".yaml"
	}
	return
}

func main() {
	configFn, command := flagParse()

	workDir, name := filepath.Split(configFn)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	if err := os.Chdir(workDir); err != nil {
		log.Fatalln(err)
	}

	var cfg Config
	if err := readYaml(configFn, &cfg); err != nil {
		if !os.IsNotExist(err) {
			log.Fatalln(err)
		}

		cfg.Description = "{name} service"
		cfg.Command = "{base}/some.exe -arg0 value0 -arg1 \"hello world\""
		cfg.Env = []string{"HELLO=World"}
		cfg.Logger = "{base}/{name}.log"
		cfg.LoggerOptions = RotateOptions{MaxSize: cmd.FileSize(5 << 20), Keep: 3}

		if err = writeYaml(configFn, cfg); err != nil {
			log.Fatalln(err)
		}
		log.Printf("生成了一个示例配置文件: %s, 打开他去修改一下", configFn)
		return
	}

	var svc daemon.Daemon
	var err error
	var status string

	if svc, err = daemon.New(name, cfg.Description, daemon.SystemDaemon, cfg.Dependencies...); err != nil {
		return
	}

	switch command {
	case "install":
		if status, err = svc.Install("-c", configFn); err == nil {
			_, err = svc.Start()
		}
	case "uninstall":
		status, err = svc.Remove()
	case "start":
		status, err = svc.Start()
	case "stop":
		status, err = svc.Stop()
	case "status":
		status, err = svc.Status()
	case "":
		status, err = svc.Run(&Service{name: name, workDir: workDir, config: cfg, configFn: configFn})
	default:
		err = flag.ErrHelp
	}

	if status != "" {
		log.Println(strings.ReplaceAll(status, "\t\t\t\t\t", " "))
	}

	if err != nil {
		if err == flag.ErrHelp {
			flag.Usage()
		} else {
			log.Fatalln(err)
		}
	}
}

type Config struct {
	Description   string        `json:"description,omitempty" yaml:"description,omitempty"`
	Dependencies  []string      `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Command       string        `json:"command,omitempty" yaml:"command,omitempty"`
	Env           []string      `json:"env,omitempty" yaml:"env,omitempty"`
	Logger        string        `json:"logger,omitempty" yaml:"logger,omitempty"` //合并输出，单独优先
	LoggerOptions RotateOptions `json:"logger_options,omitempty" yaml:"logger_options,omitempty"`
	AfterStart    []string      `json:"after_started,omitempty" yaml:"post_start,omitempty"`
	BeforeExit    []string      `json:"before_exit,omitempty" yaml:"pre_exit,omitempty"`
}

type RotateOptions struct {
	MaxSize cmd.FileSize `json:"max_size" yaml:"max_size"` //默认1M，最大100M
	Keep    int          `json:"keep"`                     //0, 不保留, -1, 保留所有
}

type Service struct {
	name     string
	workDir  string
	config   Config
	configFn string
	state    *cmd.StartState
}

func (s *Service) Start() {
	log.Printf("config:  %s", s.configFn)
	log.Printf("workDir: %s", s.workDir)
	log.Printf("name:    %s", s.name)
	log.Printf("command: %s", s.config.Command)

	if len(s.config.Command) == 0 {
		log.Fatalln(`command is empty`)
	}

	if err := os.Chdir(s.workDir); err != nil {
		log.Fatalln(err)
	}

	repl := func(path string) string {
		path = fasttemplate.ExecuteString(path, "{", "}", map[string]any{"base": s.workDir, "name": s.name})
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(s.workDir, path)
	}

	c := cmd.CommandLine(repl(s.config.Command))
	log.Println(c.String())

	if len(s.config.Env) > 0 {
		for i, kv := range s.config.Env {
			s.config.Env[i] = repl(kv)
		}
		c = c.With(cmd.Envs(append(os.Environ(), s.config.Env...)))
	}

	if s.config.Logger != "" {
		if s.config.Logger == "std" {
			c = c.With(cmd.Standard)
		} else {
			c = c.With(cmd.Logger(cmd.Rotate(cmd.RotateOptions{
				Path:    repl(s.config.Logger),
				MaxSize: s.config.LoggerOptions.MaxSize,
				Keep:    s.config.LoggerOptions.Keep,
			})))
		}
	}

	c = c.With(cmd.PostStart(func(c *cmd.Cmd) {
		for _, script := range s.config.AfterStart {
			<-cmd.CommandLine(script).With(cmd.Standard).Run().Done()
		}
	}))

	c = c.With(cmd.PreExit(func(c *cmd.Cmd) {
		for _, script := range s.config.BeforeExit {
			<-cmd.CommandLine(script).With(cmd.Standard).Run().Done()
		}
	}))

	s.state = c.Run()
}

func (s *Service) Stop() {
	s.Stop()
}

func (s *Service) Run() {
	<-s.state.Done()
}

func readYaml(fn string, value any) error {
	data, err := os.ReadFile(fn)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, value)
}

func writeYaml(fn string, value any) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(fn, data, 0666)
}
