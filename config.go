package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/gofiber/fiber/v2"
)

type configType struct {
	Listen      string           `yaml:"Listen"`
	AppName     string           `yaml:"AppName"`
	RoutePrefix string           `yaml:"RoutePrefix"`
	Tasks       map[string]*Task `yaml:"Tasks"`
}

var Config configType

var allowedTaskKeyRegex = regexp.MustCompile("^[a-zAA-Z_-]+$")

func (c configType) ValidateConfig() (err error) {
	for taskKey, task := range c.Tasks {
		if !allowedTaskKeyRegex.MatchString(taskKey) {
			return fmt.Errorf("illegal character for a taskKey: %s" + taskKey)
		}
		_, err = os.Stat(task.RunnerExecutable)
		if err != nil {
			return fmt.Errorf("runner executable for taskKey %s (%s) is not found", taskKey, task.RunnerExecutable)
		}
		task.logsDir = "logs/" + taskKey
		err = os.MkdirAll(task.logsDir, 0755)
		if err != nil {
			return
		}
		task.taskKey = taskKey
		if task.MaxRunSeconds == 0 {
			task.MaxRunSeconds = 60
		}
	}
	return
}
func (c configType) RegisterRoutes(r fiber.Router) {
	for _, t := range c.Tasks {
		r.All(t.taskKey, func(c *fiber.Ctx) (err error) {
			ts, err := t.Run()
			if err != nil {
				return
			}
			return c.SendString(ts)

		})
		r.Get(t.taskKey+"/logs", t.DirBrowser(c.RoutePrefix))
		r.Static(t.taskKey+"/logs", "logs/"+t.taskKey, fiber.Static{Browse: true})
	}

}
