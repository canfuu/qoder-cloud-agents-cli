package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/canfuu/qoder-cloud-agents-cli/internal/config"
)

type Worker struct {
	envID    string
	workerID string
	workdir  string
	baseURL  string
	token    string
	client   *http.Client

	mu              sync.Mutex
	activeWorkID    string
	lastHeartbeat   string
	stopRequested   bool
	heartbeatCancel context.CancelFunc
}

type WorkItem struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	EnvironmentID string    `json:"environment_id"`
	Data          WorkData  `json:"data"`
	State         string    `json:"state"`
	Metadata      map[string]string `json:"metadata"`
}

type WorkData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type HeartbeatResponse struct {
	Type          string `json:"type"`
	LastHeartbeat string `json:"last_heartbeat"`
	LeaseExtended bool   `json:"lease_extended"`
	State         string `json:"state"`
	TTLSeconds    int    `json:"ttl_seconds"`
}

type Event struct {
	ID          string        `json:"id"`
	Type        string        `json:"type"`
	Name        string        `json:"name,omitempty"`
	Input       interface{}   `json:"input,omitempty"`
	Content     []ContentBlock `json:"content,omitempty"`
	ProcessedAt string        `json:"processed_at,omitempty"`
	ToolUseID   string        `json:"tool_use_id,omitempty"`
	EvaluatedPermission string `json:"evaluated_permission,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func NewWorker(envID, workerID, workdir string) (*Worker, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("invalid workdir: %w", err)
	}
	if err := os.MkdirAll(absWorkdir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create workdir: %w", err)
	}

	return &Worker{
		envID:    envID,
		workerID: workerID,
		workdir:  absWorkdir,
		baseURL:  cfg.APIBase,
		token:    cfg.Token,
		client:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	fmt.Printf("Worker started\n")
	fmt.Printf("  Environment: %s\n", w.envID)
	fmt.Printf("  Worker-ID:   %s\n", w.workerID)
	fmt.Printf("  Workdir:     %s\n", w.workdir)
	fmt.Printf("  Polling...\n\n")

	for {
		select {
		case <-ctx.Done():
			w.cleanup()
			return nil
		default:
		}

		item, err := w.poll(ctx)
		if err != nil {
			if ctx.Err() != nil {
				w.cleanup()
				return nil
			}
			fmt.Printf("[error] poll: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if item == nil {
			continue
		}

		fmt.Printf("[work] received: %s (session: %s)\n", item.ID, item.Data.ID)

		if err := w.processWork(ctx, item); err != nil {
			if ctx.Err() != nil {
				w.cleanup()
				return nil
			}
			fmt.Printf("[error] processing %s: %v\n", item.ID, err)
		}
	}
}

func (w *Worker) poll(ctx context.Context) (*WorkItem, error) {
	path := fmt.Sprintf("/environments/%s/work/poll?block_ms=999", w.envID)
	req, err := w.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Worker-ID", w.workerID)

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if string(bytes.TrimSpace(body)) == "null" {
		return nil, nil
	}

	var item WorkItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf("unmarshal poll response: %w", err)
	}
	return &item, nil
}

func (w *Worker) processWork(ctx context.Context, item *WorkItem) error {
	w.mu.Lock()
	w.activeWorkID = item.ID
	w.lastHeartbeat = ""
	w.stopRequested = false
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.activeWorkID = ""
		w.mu.Unlock()
	}()

	// 1. Ack
	if err := w.ack(ctx, item.ID); err != nil {
		return fmt.Errorf("ack: %w", err)
	}
	fmt.Printf("[work] acked: %s\n", item.ID)

	// 2. First heartbeat (starting -> active)
	hb, err := w.heartbeat(ctx, item.ID, "NO_HEARTBEAT")
	if err != nil {
		return fmt.Errorf("first heartbeat: %w", err)
	}
	w.mu.Lock()
	w.lastHeartbeat = hb.LastHeartbeat
	w.mu.Unlock()
	fmt.Printf("[work] active: %s\n", item.ID)

	// 3. Start heartbeat loop
	hbCtx, hbCancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.heartbeatCancel = hbCancel
	w.mu.Unlock()
	go w.heartbeatLoop(hbCtx, item.ID)

	// 4. Process session events
	err = w.processSession(ctx, item.Data.ID)

	// 5. Stop heartbeat
	hbCancel()

	// 6. Stop work item
	if stopErr := w.stopWork(ctx, item.ID); stopErr != nil {
		fmt.Printf("[warn] stop work: %v\n", stopErr)
	}
	fmt.Printf("[work] stopped: %s\n", item.ID)

	return err
}

func (w *Worker) ack(ctx context.Context, workID string) error {
	path := fmt.Sprintf("/environments/%s/work/%s/ack", w.envID, workID)
	req, err := w.newRequest(ctx, "POST", path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Worker-ID", w.workerID)

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (w *Worker) heartbeat(ctx context.Context, workID, expectedLast string) (*HeartbeatResponse, error) {
	path := fmt.Sprintf("/environments/%s/work/%s/heartbeat?expected_last_heartbeat=%s&desired_ttl_seconds=60",
		w.envID, workID, url.QueryEscape(expectedLast))
	req, err := w.newRequest(ctx, "POST", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var hb HeartbeatResponse
	if err := json.Unmarshal(body, &hb); err != nil {
		return nil, err
	}
	return &hb, nil
}

func (w *Worker) heartbeatLoop(ctx context.Context, workID string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.mu.Lock()
			last := w.lastHeartbeat
			w.mu.Unlock()

			hb, err := w.heartbeat(ctx, workID, last)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				fmt.Printf("[warn] heartbeat: %v\n", err)
				continue
			}
			w.mu.Lock()
			w.lastHeartbeat = hb.LastHeartbeat
			w.mu.Unlock()
		}
	}
}

func (w *Worker) stopWork(ctx context.Context, workID string) error {
	// First call: starting/active -> stopping
	path := fmt.Sprintf("/environments/%s/work/%s/stop", w.envID, workID)
	req, err := w.newRequest(ctx, "POST", path, map[string]interface{}{})
	if err != nil {
		return err
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// Second call: stopping -> stopped
	req2, err := w.newRequest(context.Background(), "POST", path, map[string]interface{}{})
	if err != nil {
		return err
	}
	resp2, err := w.client.Do(req2)
	if err != nil {
		return err
	}
	resp2.Body.Close()
	return nil
}

func (w *Worker) processSession(ctx context.Context, sessionID string) error {
	// Stream events from the session and process tool_use requests
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if stop was requested
		w.mu.Lock()
		stopped := w.stopRequested
		w.mu.Unlock()
		if stopped {
			return nil
		}

		// List recent events to find pending tool_use
		events, err := w.listEvents(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("list events: %w", err)
		}

		// Find the last event to determine session state
		var pendingTools []Event
		answeredToolIDs := make(map[string]bool)
		var sessionIdle bool
		for _, evt := range events {
			switch evt.Type {
			case "agent.tool_use":
				pendingTools = append(pendingTools, evt)
			case "agent.tool_result", "user.tool_result":
				// Mark tool_use as answered
				if evt.ToolUseID != "" {
					answeredToolIDs[evt.ToolUseID] = true
				}
			case "session.status_idle":
				sessionIdle = true
			case "session.status_running":
				sessionIdle = false
			}
		}

		// Filter out already-answered tool uses
		var unanswered []Event
		for _, t := range pendingTools {
			if !answeredToolIDs[t.ID] {
				unanswered = append(unanswered, t)
			}
		}
		pendingTools = unanswered

		if sessionIdle && len(pendingTools) == 0 {
			return nil
		}

		// Execute pending tools
		for _, toolEvt := range pendingTools {
			result, isError := w.executeTool(toolEvt)
			if err := w.sendToolResult(ctx, sessionID, toolEvt.ID, result, isError); err != nil {
				fmt.Printf("[error] send tool result: %v\n", err)
			}
		}

		// Wait for agent to process and potentially issue more tool calls
		if len(pendingTools) > 0 {
			time.Sleep(2 * time.Second)
		} else {
			time.Sleep(1 * time.Second)
		}
	}
}

func (w *Worker) listEvents(ctx context.Context, sessionID string) ([]Event, error) {
	path := fmt.Sprintf("/sessions/%s/events?limit=50&order=asc", sessionID)
	req, err := w.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []Event `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (w *Worker) executeTool(evt Event) (string, bool) {
	inputMap, ok := evt.Input.(map[string]interface{})
	if !ok {
		return "invalid tool input", true
	}

	fmt.Printf("[tool] %s\n", evt.Name)

	switch evt.Name {
	case "Bash":
		return w.execBash(inputMap)
	case "Read":
		return w.execRead(inputMap)
	case "Write":
		return w.execWrite(inputMap)
	case "Edit":
		return w.execEdit(inputMap)
	case "Glob":
		return w.execGlob(inputMap)
	case "Grep":
		return w.execGrep(inputMap)
	default:
		return fmt.Sprintf("unsupported tool: %s", evt.Name), true
	}
}

func (w *Worker) execBash(input map[string]interface{}) (string, bool) {
	command, _ := input["command"].(string)
	if command == "" {
		return "missing command", true
	}

	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = w.workdir
	cmd.Env = append(os.Environ(), "HOME="+w.workdir)

	output, err := cmd.CombinedOutput()
	result := string(output)
	if err != nil {
		result += "\nExit Code: " + err.Error()
		return result, false // not a tool error, just non-zero exit
	}
	return result, false
}

func (w *Worker) execRead(input map[string]interface{}) (string, bool) {
	filePath, _ := input["file_path"].(string)
	if filePath == "" {
		return "missing file_path", true
	}

	safePath, err := w.safePath(filePath)
	if err != nil {
		return err.Error(), true
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return fmt.Sprintf("read error: %v", err), true
	}
	return string(data), false
}

func (w *Worker) execWrite(input map[string]interface{}) (string, bool) {
	filePath, _ := input["file_path"].(string)
	content, _ := input["content"].(string)
	if filePath == "" {
		return "missing file_path", true
	}

	safePath, err := w.safePath(filePath)
	if err != nil {
		return err.Error(), true
	}

	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("mkdir error: %v", err), true
	}

	if err := os.WriteFile(safePath, []byte(content), 0644); err != nil {
		return fmt.Sprintf("write error: %v", err), true
	}
	return fmt.Sprintf("Write file %s successfully", filePath), false
}

func (w *Worker) execEdit(input map[string]interface{}) (string, bool) {
	filePath, _ := input["file_path"].(string)
	oldStr, _ := input["old_string"].(string)
	newStr, _ := input["new_string"].(string)
	if filePath == "" {
		return "missing file_path", true
	}

	safePath, err := w.safePath(filePath)
	if err != nil {
		return err.Error(), true
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return fmt.Sprintf("read error: %v", err), true
	}

	content := string(data)
	count := strings.Count(content, oldStr)
	if count == 0 {
		return "old_string not found in file", true
	}
	if count > 1 {
		return fmt.Sprintf("old_string appears %d times, must be unique", count), true
	}

	newContent := strings.Replace(content, oldStr, newStr, 1)
	if err := os.WriteFile(safePath, []byte(newContent), 0644); err != nil {
		return fmt.Sprintf("write error: %v", err), true
	}
	return fmt.Sprintf("Edit file %s successfully", filePath), false
}

func (w *Worker) execGlob(input map[string]interface{}) (string, bool) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return "missing pattern", true
	}

	searchPath := w.workdir
	if p, ok := input["path"].(string); ok && p != "" {
		sp, err := w.safePath(p)
		if err != nil {
			return err.Error(), true
		}
		searchPath = sp
	}

	fullPattern := filepath.Join(searchPath, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return fmt.Sprintf("glob error: %v", err), true
	}

	// Make paths relative to workdir
	var results []string
	for _, m := range matches {
		rel, _ := filepath.Rel(w.workdir, m)
		results = append(results, rel)
	}
	return strings.Join(results, "\n"), false
}

func (w *Worker) execGrep(input map[string]interface{}) (string, bool) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return "missing pattern", true
	}

	searchPath := w.workdir
	if p, ok := input["path"].(string); ok && p != "" {
		sp, err := w.safePath(p)
		if err != nil {
			return err.Error(), true
		}
		searchPath = sp
	}

	cmd := exec.Command("grep", "-r", "-n", pattern, searchPath)
	output, _ := cmd.CombinedOutput()
	return string(output), false
}

func (w *Worker) safePath(filePath string) (string, error) {
	// Resolve the path relative to workdir
	var resolved string
	if filepath.IsAbs(filePath) {
		resolved = filepath.Clean(filePath)
	} else {
		resolved = filepath.Clean(filepath.Join(w.workdir, filePath))
	}

	// Ensure it's within workdir
	if !strings.HasPrefix(resolved, w.workdir) {
		return "", fmt.Errorf("access denied: path %s is outside workdir %s", filePath, w.workdir)
	}
	return resolved, nil
}

func (w *Worker) sendToolResult(ctx context.Context, sessionID, toolUseID, result string, isError bool) error {
	path := fmt.Sprintf("/sessions/%s/events", sessionID)
	body := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"type":        "user.tool_result",
				"tool_use_id": toolUseID,
				"content": []map[string]interface{}{
					{"type": "text", "text": result},
				},
				"is_error": isError,
			},
		},
	}

	req, err := w.newRequest(ctx, "POST", path, body)
	if err != nil {
		return err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (w *Worker) cleanup() {
	w.mu.Lock()
	workID := w.activeWorkID
	cancel := w.heartbeatCancel
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if workID != "" {
		fmt.Printf("[shutdown] stopping work item: %s\n", workID)
		ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if err := w.stopWork(ctx, workID); err != nil {
			fmt.Printf("[shutdown] force stop: %v\n", err)
			// Force stop
			path := fmt.Sprintf("/environments/%s/work/%s/stop", w.envID, workID)
			req, _ := w.newRequest(ctx, "POST", path, map[string]interface{}{"force": true})
			if req != nil {
				w.client.Do(req)
			}
		}
	}
	fmt.Printf("[shutdown] worker stopped cleanly\n")
}

func (w *Worker) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	url := w.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}
