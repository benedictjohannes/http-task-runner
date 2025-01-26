package main

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"github.com/gofiber/fiber/v2"
)

var logEntryDirRegex = regexp.MustCompile(`^\d\d\d\d\d\d.\d\d\d\d\d\d$`)
var tsLogFormat = "060102.150405"

type jsonBodyTestConditions struct {
	Key   string `yaml:"Key"`
	Value any    `yaml:"Value"`
	path  *json.Path
}

func (t *jsonBodyTestConditions) Test(b json.RawMessage) (pass bool) {
	var err error
	if t.path == nil {
		t.path, err = json.CreatePath(t.Key)
		if err != nil {
			return
		}
	}
	rt := reflect.TypeOf(t.Value)
	if rt.Kind() == reflect.String {
		tv := t.Value.(string)
		v := tv
		err = t.path.Unmarshal(b, &v)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(tv, v)
	}
	if rt.Kind() >= reflect.Int && rt.Kind() <= reflect.Uint64 {
		tv := t.Value.(int)
		v := tv
		err = t.path.Unmarshal(b, &v)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(tv, v)
	}
	if rt.Kind() >= reflect.Float32 && rt.Kind() <= reflect.Float64 {
		tv := t.Value.(float64)
		v := tv
		err = t.path.Unmarshal(b, &v)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(tv, v)
	}
	if rt.Kind() == reflect.Bool {
		tv := t.Value.(bool)
		v := tv
		err = t.path.Unmarshal(b, &v)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(tv, v)
	}
	return
}

type testConditions struct {
	Header   map[string]string         `yaml:"Header"`
	JSONBody []*jsonBodyTestConditions `yaml:"JSONBody"`
}

func (t testConditions) Test(c *fiber.Ctx) (shouldRun bool) {
	var strValue string
	for key, value := range t.Header {
		strValue = c.Get(key)
		if strValue != value {
			return
		}
	}
	if len(t.JSONBody) > 0 {
		if !strings.HasPrefix(c.Get("Content-Type"), "application/json") {
			return false
		}
		reqBody := c.Body()
		for _, t := range t.JSONBody {
			if !t.Test(reqBody) {
				return
			}
		}
	}
	return true
}

type Task struct {
	RunnerExecutable string         `yaml:"RunnerExecutable"`
	Args             []string       `yaml:"Args"`
	MaxRunSeconds    int            `yaml:"MaxRunSeconds"`
	TaskKey          string         `yaml:"TaskKey"`
	WebhookRoute     string         `yaml:"WebhookRoute"`
	Tests            testConditions `yaml:"Tests"`
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
func (t *Task) ShouldRun(c *fiber.Ctx) (shouldRun bool) {
	return t.Tests.Test(c)
}
func (t *Task) Run() (logPrefix string, err error) {
	ts, err := t.startTask()
	if err != nil {
		return
	}
	cmd := exec.Command(t.RunnerExecutable, t.Args...)
	if len(t.TaskKey) > 0 {

		logPrefix = ts.Format(tsLogFormat)
		dir := "logs/" + t.TaskKey + "/" + logPrefix
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return
		}
		var outFile, errFile *os.File
		outFile, err = os.OpenFile(dir+"/out.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return
		}
		errFile, err = os.OpenFile(dir+"/err.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return
		}
		cmd.Stdout = outFile
		cmd.Stderr = errFile
	}
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
		dirs := make([]string, 0, len(files))
		for _, f := range files {
			if f.IsDir() && logEntryDirRegex.MatchString(f.Name()) {
				dir := f.Name()
				ts, err := time.Parse(tsLogFormat, dir)
				if err != nil {
					continue
				}
				li := fmt.Sprintf("<li><a href=\"%s/logs/%s/%s\">%s</a></li>",
					routePrefix, t.TaskKey, dir, ts.Format("2006-01-02 15:04:05"),
				)
				dirs = append(dirs, li)
			}

		}
		slices.Sort(dirs)
		slices.Reverse(dirs)
		c.Set("Content-Type", "text/html")
		html := fmt.Sprintf("<html><head><title>%s</title></head>"+
			"<body><h1>Log entries for %s</h1><ul>%s</ul></body></html>",
			t.TaskKey, t.TaskKey, strings.Join(dirs, ""),
		)
		return c.SendString(html)
	}
}
func (t *Task) Validate() (err error) {
	if len(t.Tests.JSONBody) > 0 {
		for _, e := range t.Tests.JSONBody {
			e.path, err = json.CreatePath(e.Key)
			if err != nil {
				return
			}
		}
	}
	return
}
