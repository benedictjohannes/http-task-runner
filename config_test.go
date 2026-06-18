package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestConfigType_ValidateConfig(t *testing.T) {
	tempDir := t.TempDir()
	dummyExecutable := filepath.Join(tempDir, "dummy_exec")
	if err := os.WriteFile(dummyExecutable, []byte("echo"), 0755); err != nil {
		t.Fatalf("failed to write dummy executable: %v", err)
	}

	t.Run("valid config", func(t *testing.T) {
		config := configType{
			Listen:      ":8080",
			AppName:     "TestApp",
			RoutePrefix: "/api",
			Tasks: []*Task{
				{
					TaskKey:          "valid-task_key.123",
					RunnerExecutable: dummyExecutable,
					WebhookRoute:     "valid-route",
				},
			},
		}

		defer os.RemoveAll("logs/valid-task_key.123")

		if err := config.ValidateConfig(); err != nil {
			t.Errorf("expected config to be valid, got error: %v", err)
		}

		// Verify that default values were set
		task := config.Tasks[0]
		if task.MaxRunSeconds != 60 {
			t.Errorf("expected MaxRunSeconds to default to 60, got %d", task.MaxRunSeconds)
		}
		if task.logsDir != "logs/valid-task_key.123" {
			t.Errorf("expected logsDir to be logs/valid-task_key.123, got %s", task.logsDir)
		}
	})

	t.Run("default webhook route", func(t *testing.T) {
		config := configType{
			Listen:  ":8080",
			AppName: "TestApp",
			Tasks: []*Task{
				{
					TaskKey:          "my-task",
					RunnerExecutable: dummyExecutable,
				},
			},
		}

		defer os.RemoveAll("logs/my-task")

		if err := config.ValidateConfig(); err != nil {
			t.Fatalf("expected config to be valid, got error: %v", err)
		}

		task := config.Tasks[0]
		if task.WebhookRoute != "my-task" {
			t.Errorf("expected WebhookRoute to default to TaskKey 'my-task', got %q", task.WebhookRoute)
		}
	})

	t.Run("invalid task key", func(t *testing.T) {
		config := configType{
			Tasks: []*Task{
				{
					TaskKey:          "invalid task key",
					RunnerExecutable: dummyExecutable,
				},
			},
		}

		if err := config.ValidateConfig(); err == nil {
			t.Error("expected validation error due to invalid task key, got nil")
		} else if !strings.Contains(err.Error(), "illegal character for a taskKey") {
			t.Errorf("expected taskKey validation error, got: %v", err)
		}
	})

	t.Run("missing runner executable", func(t *testing.T) {
		config := configType{
			Tasks: []*Task{
				{
					TaskKey:          "validkey",
					RunnerExecutable: filepath.Join(tempDir, "non_existent_file"),
				},
			},
		}

		if err := config.ValidateConfig(); err == nil {
			t.Error("expected validation error due to missing runner executable, got nil")
		} else if !strings.Contains(err.Error(), "is not found") {
			t.Errorf("expected runner executable missing error, got: %v", err)
		}
	})

	t.Run("invalid webhook route", func(t *testing.T) {
		config := configType{
			Tasks: []*Task{
				{
					TaskKey:          "validkey",
					RunnerExecutable: dummyExecutable,
					WebhookRoute:     "invalid/route/path",
				},
			},
		}

		if err := config.ValidateConfig(); err == nil {
			t.Error("expected validation error due to invalid webhook route, got nil")
		} else if !strings.Contains(err.Error(), "illegal character for a task Route") {
			t.Errorf("expected webhook route validation error, got: %v", err)
		}
	})
}

func TestConfigType_RegisterRoutes(t *testing.T) {
	echoPath, errEcho := exec.LookPath("echo")
	if errEcho != nil {
		t.Skip("Skip test: echo binary not found on this system")
	}

	taskKey := "routertask"
	logsDir := "logs/" + taskKey
	defer os.RemoveAll(logsDir)

	config := configType{
		Listen:      ":8080",
		AppName:     "TestRouterApp",
		RoutePrefix: "/prefix",
		Tasks: []*Task{
			{
				TaskKey:          taskKey,
				RunnerExecutable: echoPath,
				Args:             []string{"hello"},
				WebhookRoute:     "webhook-path",
				MaxRunSeconds:    5,
				logsDir:          logsDir,
				Tests: testConditions{
					Header: map[string]string{
						"X-Run-Allowed": "Yes",
					},
				},
			},
		},
	}

	// Make sure logs/ directory exists for validation (and cleanup)
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	app := fiber.New()
	taskRouter := app.Group(config.RoutePrefix)
	config.RegisterRoutes(taskRouter)

	// Test 1: GET /prefix/logs endpoint
	t.Run("GET logs endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/prefix/logs", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to test app: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, "TestRouterApp") {
			t.Errorf("expected HTML to contain AppName 'TestRouterApp', got %q", bodyStr)
		}
		if !strings.Contains(bodyStr, "/prefix/logs/routertask") {
			t.Errorf("expected HTML to contain link prefix and task key, got %q", bodyStr)
		}
	})

	// Test 2: POST to task endpoint with wrong headers (should not run task)
	t.Run("POST to webhook without header", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/prefix/tasks/webhook-path", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to test app: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}
	})

	// Test 3: POST to task endpoint with valid headers (should run task)
	t.Run("POST to webhook with correct header", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/prefix/tasks/webhook-path", nil)
		req.Header.Set("X-Run-Allowed", "Yes")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to test app: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", resp.StatusCode)
		}
	})
}
