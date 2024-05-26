package main

import (
	"flag"
	"io"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)

func main() {
	configFileName := flag.String("config", "config.yaml", "Configuration file to run")
	configFile, err := os.Open(*configFileName)
	if err != nil {
		log.Fatalln(err)
	}
	configB, err := io.ReadAll(configFile)
	if err != nil {
		log.Fatalln(err)
	}
	err = yaml.Unmarshal(configB, &Config)
	if err != nil {
		log.Fatalln(err)
	}
	err = Config.ValidateConfig()
	if err != nil {
		log.Fatalln(err)
	}
	server := fiber.New(fiber.Config{
		AppName:      Config.AppName,
		UnescapePath: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).SendString(err.Error())
		},
	})
	taskRouter := server.Group(Config.RoutePrefix)
	Config.RegisterRoutes(taskRouter)
	server.All("**", func(c *fiber.Ctx) error { return c.SendStatus(404) })
	server.Listen(Config.Listen)
}
