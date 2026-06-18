package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/goccy/go-yaml"
)

func startWatcher(configPath string, reloadChan chan configType) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalln("Err creating watcher:", err)
	}
	defer watcher.Close()

	configDir := filepath.Dir(configPath)
	err = watcher.Add(configDir)
	if err != nil {
		log.Fatalln("Err adding dir to watcher:", err)
	}

	var timer *time.Timer
	debounceDuration := 100 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			eventPath := filepath.Clean(event.Name)
			if eventPath != configPath {
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Rename == fsnotify.Rename {

				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(debounceDuration, func() {
					configB, err := os.ReadFile(configPath)
					if err != nil {
						log.Println("Watcher error reading config file:", err)
						return
					}
					var newConfig configType
					err = yaml.Unmarshal(configB, &newConfig)
					if err != nil {
						log.Println("Watcher error parsing YAML config:", err)
						return
					}
					err = newConfig.ValidateConfig()
					if err != nil {
						log.Println("Watcher error validating config:", err)
						return
					}

					select {
					case reloadChan <- newConfig:
					default:
						select {
						case <-reloadChan:
						default:
						}
						reloadChan <- newConfig
					}
				})
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("Watcher error:", err)
		}
	}
}
