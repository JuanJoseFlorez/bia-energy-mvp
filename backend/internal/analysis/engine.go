package analysis

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// maxLineBytes bounds one NDJSON line; the "result" line carries every anomaly's evidence.
const maxLineBytes = 4 << 20

// Engine failures, mapped to client-safe run errors by the service.
var (
	errEngineUnavailable = errors.New("analysis engine unavailable")
	errEngineFailed      = errors.New("analysis engine failed")
	errNoResult          = errors.New("analysis engine returned no result")
)

// EngineClient calls engine-ai's POST /analyze and reads its NDJSON progress stream.
type EngineClient struct {
	baseURL string
	http    *http.Client
}

// NewEngineClient returns a client for the engine-ai base URL (e.g. http://engine-ai:8000).
func NewEngineClient(baseURL string) *EngineClient {
	// No client Timeout: the stream lasts the whole analysis; the caller's ctx carries the deadline.
	return &EngineClient{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{}}
}

// Analyze runs one analysis, calling onStep for every progress line, and returns the result.
func (c *EngineClient) Analyze(ctx context.Context, analysisID int64, onStep func(step string)) (Result, error) {
	body, err := json.Marshal(map[string]int64{"analysis_id": analysisID})
	if err != nil {
		return Result{}, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/analyze", bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, fmt.Errorf("%w: %v", errEngineUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("%w: status %d", errEngineUnavailable, resp.StatusCode)
	}

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), maxLineBytes)
	for sc.Scan() {
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var line streamLine
		if err := json.Unmarshal(raw, &line); err != nil {
			return Result{}, fmt.Errorf("%w: invalid line: %v", errNoResult, err)
		}
		switch line.Type {
		case "step":
			onStep(line.Step)
		case "result":
			return line.Result, nil
		case "error":
			return Result{}, fmt.Errorf("%w: %s", errEngineFailed, line.Message)
		default:
			slog.DebugContext(ctx, "ignoring unknown engine stream line", "analysis_id", analysisID, "type", line.Type)
		}
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if err := sc.Err(); err != nil {
		return Result{}, fmt.Errorf("%w: read stream: %v", errNoResult, err)
	}
	return Result{}, errNoResult
}
