package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/gofiber/fiber/v2"
)

type configType struct {
	Listen      string  `yaml:"Listen"`
	AppName     string  `yaml:"AppName"`
	RoutePrefix string  `yaml:"RoutePrefix"`
	Tasks       []*Task `yaml:"Tasks"`
}

var Config configType

var allowedTaskKeyRegex = regexp.MustCompile(`^[0-9a-zAA-Z-_\.]+$`)

func (c configType) ValidateConfig() (err error) {
	for _, task := range c.Tasks {
		if !allowedTaskKeyRegex.MatchString(task.TaskKey) {
			return fmt.Errorf("illegal character for a taskKey: %s", task.TaskKey)
		}
		_, err = os.Stat(task.RunnerExecutable)
		if err != nil {
			return fmt.Errorf("runner executable for taskKey %s (%s) is not found", task.TaskKey, task.RunnerExecutable)
		}
		if len(task.Route) > 0 && !allowedTaskKeyRegex.MatchString(task.Route) {
			return fmt.Errorf("illegal character for a task Route: %s", task.Route)
		}
		task.logsDir = "logs/" + task.TaskKey
		err = os.MkdirAll(task.logsDir, 0755)
		if err != nil {
			return
		}
		if task.MaxRunSeconds == 0 {
			task.MaxRunSeconds = 60
		}
	}
	return
}
func (c configType) RegisterRoutes(r fiber.Router) {
	routeMap := make(map[string][]*Task)
	for _, t := range c.Tasks {
		if len(t.Route) > 0 {
			routeMap[t.Route] = append(routeMap[t.Route], t)
		}
		r.Get("logs/"+t.TaskKey, t.DirBrowser(c.RoutePrefix))
		r.Static("logs/"+t.TaskKey, "logs/"+t.TaskKey, fiber.Static{Browse: true})
	}
	if len(routeMap) > 0 {
		taskRouter := r.Group("tasks")
		for r, tr := range routeMap {
			fn := func(c *fiber.Ctx) error {
				for _, t := range tr {
					if t.ShouldRun(c) {
						t.Run()
					}
				}
				return c.SendStatus(fiber.StatusOK)
			}
			taskRouter.Get(r, fn)
			taskRouter.Post(r, fn)
		}
	}
}
