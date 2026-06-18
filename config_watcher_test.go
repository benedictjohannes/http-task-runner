package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/gofiber/fiber/v2"
)

func TestWatchAndReload(t *testing.T) {
	// Create a temporary config file
	tempDir, err := os.MkdirTemp("", "config-watch-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")

	// Config v1: listen on port 52258, route /test1
	v1Config := configType{
		Listen:      "127.0.0.1:52258",
		AppName:     "TestV1",
		RoutePrefix: "test1",
		Tasks: []*Task{
			{
				TaskKey:          "taskone",
				RunnerExecutable: "/bin/echo",
				Args:             []string{"hello"},
				WebhookRoute:     "taskone",
			},
		},
	}
	v1Data, err := yaml.Marshal(v1Config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, v1Data, 0644); err != nil {
		t.Fatal(err)
	}

	// Clean up logs generated during validation
	defer os.RemoveAll("logs/taskone")
	defer os.RemoveAll("logs/tasktwo")

	// Set initial config
	Config = v1Config
	if err := Config.ValidateConfig(); err != nil {
		t.Fatal(err)
	}

	reloadChan := make(chan configType, 1)
	go startWatcher(configPath, reloadChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			server := fiber.New(fiber.Config{
				AppName:      Config.AppName,
				UnescapePath: true,
			})
			taskRouter := server.Group(Config.RoutePrefix)
			Config.RegisterRoutes(taskRouter)

			setServer(server)

			listenErrChan := make(chan error, 1)
			go func() {
				listenErrChan <- server.Listen(Config.Listen)
			}()

			select {
			case <-ctx.Done():
				activeServerMu.Lock()
				if activeServer != nil {
					activeServer.Shutdown()
				}
				activeServerMu.Unlock()
				return
			case <-listenErrChan:
				return
			case newConfig := <-reloadChan:
				activeServerMu.Lock()
				if activeServer != nil {
					activeServer.Shutdown()
				}
				activeServerMu.Unlock()
				<-listenErrChan
				Config = newConfig
			}
		}
	}()

	// Wait for server to start
	time.Sleep(300 * time.Millisecond)

	// Verify v1 works
	resp, err := http.Get("http://127.0.0.1:52258/test1/logs")
	if err != nil {
		t.Fatalf("Failed to query v1 server: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "taskone") {
		t.Errorf("Expected body to contain taskone, got: %s", string(body))
	}

	// Config v2: update RoutePrefix to test2, task to tasktwo
	v2Config := configType{
		Listen:      "127.0.0.1:52258",
		AppName:     "TestV2",
		RoutePrefix: "test2",
		Tasks: []*Task{
			{
				TaskKey:          "tasktwo",
				RunnerExecutable: "/bin/echo",
				Args:             []string{"world"},
				WebhookRoute:     "tasktwo",
			},
		},
	}
	v2Data, err := yaml.Marshal(v2Config)
	if err != nil {
		t.Fatal(err)
	}

	// Write new config to trigger watch reload
	if err := os.WriteFile(configPath, v2Data, 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for watcher and debounce timer to trigger reload
	time.Sleep(500 * time.Millisecond)

	// Verify v2 works on the new route prefix
	resp2, err := http.Get("http://127.0.0.1:52258/test2/logs")
	if err != nil {
		t.Fatalf("Failed to query v2 server: %v", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for v2 server, got %d", resp2.StatusCode)
	}
	if !strings.Contains(string(body2), "tasktwo") {
		t.Errorf("Expected body2 to contain tasktwo, got: %s", string(body2))
	}
}
