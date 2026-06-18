package main

import (
	"bytes"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
)

func TestJSONBodyTestConditions_Test(t *testing.T) {
	tests := []struct {
		name       string
		condition  jsonBodyTestConditions
		rawJSON    string
		expectPass bool
	}{
		{
			name: "string match success",
			condition: jsonBodyTestConditions{
				Key:   "$.user.name",
				Value: "John",
			},
			rawJSON:    `{"user": {"name": "John"}}`,
			expectPass: true,
		},
		{
			name: "string match mismatch",
			condition: jsonBodyTestConditions{
				Key:   "$.user.name",
				Value: "John",
			},
			rawJSON:    `{"user": {"name": "Jane"}}`,
			expectPass: false,
		},
		{
			name: "integer match success",
			condition: jsonBodyTestConditions{
				Key:   "$.age",
				Value: 30,
			},
			rawJSON:    `{"age": 30}`,
			expectPass: true,
		},
		{
			name: "float match success",
			condition: jsonBodyTestConditions{
				Key:   "$.price",
				Value: 19.99,
			},
			rawJSON:    `{"price": 19.99}`,
			expectPass: true,
		},
		{
			name: "bool match success",
			condition: jsonBodyTestConditions{
				Key:   "$.active",
				Value: true,
			},
			rawJSON:    `{"active": true}`,
			expectPass: true,
		},
		{
			name: "key path not found",
			condition: jsonBodyTestConditions{
				Key:   "$.missing.key",
				Value: "value",
			},
			rawJSON:    `{"user": "John"}`,
			expectPass: false,
		},
		{
			name: "type mismatch",
			condition: jsonBodyTestConditions{
				Key:   "$.age",
				Value: "thirty",
			},
			rawJSON:    `{"age": 30}`,
			expectPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pass := tt.condition.Test(json.RawMessage(tt.rawJSON))
			if pass != tt.expectPass {
				t.Errorf("expected pass = %v, got %v", tt.expectPass, pass)
			}
		})
	}
}

func TestTestConditions_Test(t *testing.T) {
	app := fiber.New()

	conds := testConditions{
		Header: map[string]string{
			"X-Test-Header": "Allowed",
		},
		JSONBody: []*jsonBodyTestConditions{
			{
				Key:   "$.action",
				Value: "run",
			},
		},
	}

	app.Post("/webhook", func(c *fiber.Ctx) error {
		if conds.Test(c) {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusForbidden)
	})

	tests := []struct {
		name           string
		headers        map[string]string
		body           string
		expectedStatus int
	}{
		{
			name: "all match success",
			headers: map[string]string{
				"X-Test-Header": "Allowed",
				"Content-Type":  "application/json",
			},
			body:           `{"action": "run"}`,
			expectedStatus: fiber.StatusOK,
		},
		{
			name: "header mismatch",
			headers: map[string]string{
				"X-Test-Header": "Blocked",
				"Content-Type":  "application/json",
			},
			body:           `{"action": "run"}`,
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name: "content type not json",
			headers: map[string]string{
				"X-Test-Header": "Allowed",
				"Content-Type":  "text/plain",
			},
			body:           `{"action": "run"}`,
			expectedStatus: fiber.StatusForbidden,
		},
		{
			name: "json value mismatch",
			headers: map[string]string{
				"X-Test-Header": "Allowed",
				"Content-Type":  "application/json",
			},
			body:           `{"action": "stop"}`,
			expectedStatus: fiber.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func TestTask_Validate(t *testing.T) {
	t.Run("valid JSON path", func(t *testing.T) {
		task := &Task{
			Tests: testConditions{
				JSONBody: []*jsonBodyTestConditions{
					{Key: "$.user.name", Value: "John"},
				},
			},
		}
		if err := task.Validate(); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if task.Tests.JSONBody[0].path == nil {
			t.Error("expected parsed path to be set, got nil")
		}
	})

	t.Run("invalid JSON path", func(t *testing.T) {
		task := &Task{
			Tests: testConditions{
				JSONBody: []*jsonBodyTestConditions{
					{Key: "$.user.[invalid", Value: "John"},
				},
			},
		}
		if err := task.Validate(); err == nil {
			t.Error("expected error for invalid JSON path, got nil")
		}
	})
}

func TestTask_ConcurrencyAndExecution(t *testing.T) {
	// Look up sleep and echo executables to prevent false failures in environments without them
	sleepPath, errSleep := exec.LookPath("sleep")
	echoPath, errEcho := exec.LookPath("echo")
	if errSleep != nil || errEcho != nil {
		t.Skip("Skip test: sleep or echo binary not found on this system")
	}

	taskKey := "testconcurrencytask"
	logsDir := "logs/" + taskKey
	defer os.RemoveAll(logsDir)

	task := &Task{
		RunnerExecutable: sleepPath,
		Args:             []string{"1"},
		MaxRunSeconds:    5,
		TaskKey:          taskKey,
		logsDir:          logsDir,
	}

	// 1. Start task
	logPrefix, err := task.Run()
	if err != nil {
		t.Fatalf("expected task to run successfully: %v", err)
	}
	if logPrefix == "" {
		t.Error("expected non-empty log prefix")
	}

	// 2. Start again immediately (should fail due to concurrency control)
	_, err = task.Run()
	if err == nil {
		t.Error("expected error when running task concurrently, got nil")
	} else if !strings.Contains(err.Error(), "still running") {
		t.Errorf("expected still running error, got: %v", err)
	}

	// Clean running status manually to check echo task execution
	task.mu.Lock()
	task.runningDeadline = nil
	task.mu.Unlock()

	// 3. Test logging output with echo
	task.RunnerExecutable = echoPath
	task.Args = []string{"hello-from-test"}
	
	logPrefix2, err := task.Run()
	if err != nil {
		t.Fatalf("expected task to run successfully: %v", err)
	}

	// Wait for background cmd.Run/executeCmd execution to complete
	time.Sleep(300 * time.Millisecond)

	outPath := filepath.Join(logsDir, logPrefix2, "out.log")
	outContent, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected to read stdout log file at %s: %v", outPath, err)
	}
	if !strings.Contains(string(outContent), "hello-from-test") {
		t.Errorf("expected out.log to contain 'hello-from-test', got: %q", string(outContent))
	}
}
