package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

var logEntryDirRegex = regexp.MustCompile(`^\d\d\d\d\d\d.\d\d\d\d\d\d$`)
var tsLogFormat = "060102.150405"

type Task struct {
	RunnerExecutable string   `yaml:"RunnerExecutable"`
	Args             []string `yaml:"Args"`
	MaxRunSeconds    int      `yaml:"MaxRunSeconds"`
	taskKey          string
	logsDir          string
	mu               sync.Mutex
	runningDeadline  *time.Time
}

func (t *Task) startTask() (ts time.Time, err error) {
	ts = time.Now()
	if t.runningDeadline != nil && t.runningDeadline.After(ts) {
		err = fmt.Errorf("current task is still running until %q", t.runningDeadline)
		return
	}
	t.mu.Lock()
	deadline := ts.Add(time.Duration(t.MaxRunSeconds) * time.Second)
	t.runningDeadline = &deadline
	t.mu.Unlock()
	return
}
func (t *Task) executeCmd(cmd *exec.Cmd) {
	err := cmd.Run()
	if err != nil {
		cmd.Stderr.Write([]byte("\n" + err.Error()))
	}
	t.mu.Lock()
	t.runningDeadline = nil
	t.mu.Unlock()
}
func (t *Task) Run() (logPrefix string, err error) {
	ts, err := t.startTask()
	if err != nil {
		return
	}
	logPrefix = ts.Format(tsLogFormat)
	dir := "logs/" + t.taskKey + "/" + logPrefix
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}
	outFile, err := os.OpenFile(dir+"/out.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	errFile, err := os.OpenFile(dir+"/err.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	cmd := exec.Command(t.RunnerExecutable, t.Args...)
	cmd.Stdout = outFile
	cmd.Stderr = errFile
	cmd.WaitDelay = time.Second * time.Duration(t.MaxRunSeconds)
	go t.executeCmd(cmd)
	return
}
func (t *Task) DirBrowser(routePrefix string) func(c *fiber.Ctx) (err error) {
	return func(c *fiber.Ctx) (err error) {
		files, err := os.ReadDir(t.logsDir)
		if err != nil {
			return
		}
		usedRoutePrefix := routePrefix
		if !strings.HasSuffix(routePrefix, "/") {
			usedRoutePrefix = "/" + routePrefix
		}
		dirs := make([]string, 0, len(files))
		for _, f := range files {
			if f.IsDir() && logEntryDirRegex.MatchString(f.Name()) {
				dir := f.Name()
				ts, err := time.Parse(tsLogFormat, dir)
				if err != nil {
					continue
				}
				li := fmt.Sprintf("<li><a href=\"%s/%s/logs/%s\">%s</a></li>",
					usedRoutePrefix, t.taskKey, dir, ts.Format("2006-01-02 15:04:05"),
				)
				dirs = append(dirs, li)
			}

		}
		slices.Sort(dirs)
		slices.Reverse(dirs)
		c.Set("Content-Type", "text/html")
		html := fmt.Sprintf("<html><head><title>%s</title></head>"+
			"<body><h1>Log entries for %s</h1><ul>%s</ul></body></html>",
			t.taskKey, t.taskKey, strings.Join(dirs, ""),
		)
		return c.SendString(html)
	}
}
