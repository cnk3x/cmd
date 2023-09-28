package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cnk3x/cmd"
	"github.com/cnk3x/cmd/example/svcgo/svc"
	"github.com/cnk3x/cmd/example/svcgo/svc/kardianos"
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
		cfg.Logger.Path = "{base}/{name}.log"
		cfg.Logger.MaxSize = cmd.FileSize(5 << 20)
		cfg.Logger.Keep = 3

		if err = writeYaml(configFn, cfg); err != nil {
			log.Fatalln(err)
		}
		log.Printf("生成了一个示例配置文件: %s, 打开他去修改一下", configFn)
		return
	}

	cfg.Name = name
	cfg.Config.Workdir = workDir

	cfg.name = cfg.Name
	cfg.workDir = workDir

	runner := createRunner(cfg.CommandOptions)
	man := kardianos.New(runner, cfg.Config, "-c", configFn, "run")

	status, err := man.Manage(command)

	if status != "" {
		log.Println(strings.ReplaceAll(status, "\t\t\t\t\t", " "))
	}

	if err != nil {
		if err == svc.ErrHelp {
			flag.Usage()
		} else {
			log.Fatalln(err)
		}
	}
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

func createRunner(cfg CommandOptions) svc.ServiceRunner {
	return func(ctx context.Context) (done <-chan struct{}, err error) {
		repl := func(s string) string {
			return fasttemplate.ExecuteString(s, "{", "}", map[string]any{"base": cfg.workDir, "name": cfg.name})
		}

		resolvePath := func(path string) string {
			if path = repl(path); path != "" && !filepath.IsAbs(path) {
				path = filepath.Join(cfg.workDir, path)
			}
			return path
		}

		c := cmd.CommandLine(repl(cfg.Command)).With(cmd.WorkDir(cfg.workDir))

		cfg.Logger.Path = resolvePath(cfg.Logger.Path)
		c.Logger(cfg.Logger)

		c.PreExit(func(c *cmd.Cmd) {
			for _, n := range cfg.BeforeExit {
				<-cmd.CommandLine(n).Standard().Run().Done()
			}
		})

		c.PostStart(func(c *cmd.Cmd) {
			for _, n := range cfg.AfterStarted {
				<-cmd.CommandLine(n).Standard().Run().Done()
			}
		})

		state := c.Run()
		return state.Done(), state.Err
	}
}

type Config struct {
	svc.Config     `json:",inline" yaml:",inline"`
	CommandOptions `json:",inline" yaml:",inline"`
}

type CommandOptions struct {
	name    string //服务名称
	workDir string //工作目录，从服务配置转存过来

	Command      string            `json:"command,omitempty" yaml:"command,omitempty"`
	Logger       cmd.LoggerOptions `json:"logger,omitempty" yaml:"logger,omitempty"`
	AfterStarted []string          `json:"after_started,omitempty" yaml:"after_started,omitempty"`
	BeforeExit   []string          `json:"before_exit,omitempty" yaml:"before_exit,omitempty"`
}
