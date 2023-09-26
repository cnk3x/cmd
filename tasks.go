package cmd

import "sync"

//顺序执行
type Runner struct {
	tasks []Task
	wg    sync.WaitGroup
}

type Task struct {
	Parallel bool
	Run      func()
}

func (p *Runner) Append(task func(), parallel ...bool) {
	p.tasks = append(p.tasks, Task{Parallel: len(parallel) > 0 && parallel[0], Run: task})
}

func (p *Runner) Parallel(run ...func()) {
	for _, run := range run {
		p.Append(run, true)
	}
}

func (p *Runner) Run() {
	if p != nil {
		for _, task := range p.tasks {
			if task.Parallel {
				bgRun(&p.wg, task.Run)
			} else {
				task.Run()
			}
		}
		p.wg.Wait()
	}
}
