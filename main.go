package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)

var (
	activeServer   *fiber.App
	activeServerMu sync.Mutex
)

func setServer(s *fiber.App) {
	activeServerMu.Lock()
	activeServer = s
	activeServerMu.Unlock()
}

func main() {
	configFileName := flag.String("config", "config.yaml", "Configuration file to run")
	watch := flag.Bool("watch", false, "watch the configuration and reload on change")
	flag.Parse()

	configPath, err := filepath.Abs(*configFileName)
	if err != nil {
		log.Fatalln("Err resolving absolute path for config file:", err)
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatalln("Err opening "+configPath+":", err)
	}
	configB, err := io.ReadAll(configFile)
	configFile.Close()
	if err != nil {
		log.Fatalln("Err reading "+configPath+":", err)
	}
	err = yaml.Unmarshal(configB, &Config)
	if err != nil {
		log.Fatalln("Err yaml.Unmarshal "+configPath+":", err)
	}
	err = Config.ValidateConfig()
	if err != nil {
		log.Fatalln("Err Config.ValidateConfig: ", err)
	}

	if !*watch {
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
		return
	}

	reloadChan := make(chan configType, 1)
	go startWatcher(configPath, reloadChan)

	for {
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

		setServer(server)

		log.Printf("Starting server on %s...\n", Config.Listen)
		listenErrChan := make(chan error, 1)
		go func() {
			listenErrChan <- server.Listen(Config.Listen)
		}()

		select {
		case err := <-listenErrChan:
			if err != nil {
				log.Println("Server stopped with error:", err)
			} else {
				log.Println("Server stopped")
			}
			return
		case newConfig := <-reloadChan:
			log.Println("Config change detected, reloading...")
			activeServerMu.Lock()
			if activeServer != nil {
				if err := activeServer.Shutdown(); err != nil {
					log.Println("Error shutting down server:", err)
				}
			}
			activeServerMu.Unlock()

			// Wait for the active server's listener to finish shutting down
			<-listenErrChan

			Config = newConfig
		}
	}
}
