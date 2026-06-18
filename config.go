package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type configType struct {
	Listen          string  `yaml:"Listen"`
	AppName         string  `yaml:"AppName"`
	RoutePrefix     string  `yaml:"RoutePrefix"`
	SeparateRunLogs bool    `yaml:"SeparateRunLogs"`
	Tasks           []*Task `yaml:"Tasks"`
}

var Config configType

var allowedTaskKeyRegex = regexp.MustCompile(`^[0-9a-zAA-Z-_\.]+$`)

func (c *configType) ValidateConfig() (err error) {
	for _, task := range c.Tasks {
		if !allowedTaskKeyRegex.MatchString(task.TaskKey) {
			return fmt.Errorf("illegal character for a taskKey: %s", task.TaskKey)
		}
		_, err = os.Stat(task.RunnerExecutable)
		if err != nil {
			return fmt.Errorf("runner executable for taskKey %s (%s) is not found", task.TaskKey, task.RunnerExecutable)
		}
		if len(task.WebhookRoute) > 0 && !allowedTaskKeyRegex.MatchString(task.WebhookRoute) {
			return fmt.Errorf("illegal character for a task Route: %s", task.WebhookRoute)
		}
		if len(task.WebhookRoute) == 0 {
			task.WebhookRoute = task.TaskKey
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
func (c *configType) RegisterRoutes(router fiber.Router) {
	webhookRouteMap := make(map[string][]*Task)
	logRouteMap := make(map[string]bool)
	var logListPageHtml string
	usedRoutePrefix := "/" + strings.TrimPrefix(strings.TrimSuffix(c.RoutePrefix, "/"), "/")
	linkPrefix := ""
	if len(usedRoutePrefix) > 1 {
		linkPrefix = usedRoutePrefix
	}
	for _, t := range c.Tasks {
		webhookRouteMap[t.WebhookRoute] = append(webhookRouteMap[t.WebhookRoute], t)
		if len(t.TaskKey) > 0 && !logRouteMap[t.TaskKey] {
			router.Get("logs/"+t.TaskKey, t.DirBrowser(usedRoutePrefix))
			router.Static("logs/"+t.TaskKey, "logs/"+t.TaskKey, fiber.Static{Browse: true})
			logListPageHtml += fmt.Sprintf("<li><a href=\"%s/logs/%s\">%s</a></li>",
				linkPrefix, t.TaskKey, t.TaskKey,
			)
			logRouteMap[t.TaskKey] = true
		}
	}
	logListPageHtml = fmt.Sprintf("<html><head><title>%s | Tasks</title></head>"+
		"<body><h1>Tasks of %s:</h1><ul>%s</ul></body></html>",
		c.AppName, c.AppName, logListPageHtml,
	)
	router.Get("/logs", func(ctx *fiber.Ctx) (err error) {
		ctx.Set("Content-Type", "text/html")
		return ctx.SendString(logListPageHtml)
	})
	if len(webhookRouteMap) > 0 {
		taskRouter := router.Group("tasks")
		for subroute, subrouteTasks := range webhookRouteMap {
			fn := func(ctx *fiber.Ctx) error {
				for _, t := range subrouteTasks {
					if t.ShouldRun(ctx) {
						t.Run(!c.SeparateRunLogs)
					}
				}
				return ctx.SendStatus(fiber.StatusOK)
			}
			taskRouter.Get(subroute, fn)
			taskRouter.Post(subroute, fn)
		}
	}
}
