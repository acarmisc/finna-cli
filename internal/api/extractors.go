package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// ---- types ------------------------------------------------------------------

// ExtractorResponse mirrors the shape in the example from GET /api/v1/extractors.
// The server uses additionalProperties:true so we keep a Raw map for forward-compat.
type ExtractorResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	ConfigID   string `json:"config_id"`
	ConfigName string `json:"config_name"`
	Enabled    bool   `json:"enabled"`
	Schedule   string `json:"schedule"` // cron string or empty
	LastRun    string `json:"last_run"`
	Status     string `json:"status"`
	Raw        map[string]any
}

// ExtractorListResponse is the paginated wrapper returned by GET /api/v1/extractors.
type ExtractorListResponse struct {
	Data     []map[string]any `json:"data"`
	Count    int              `json:"count"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	HasNext  bool             `json:"has_next"`
	HasPrev  bool             `json:"has_prev"`
}

// ExtractorCreate is the body for POST /api/v1/extractors.
type ExtractorCreate struct {
	Name          string         `json:"name"`
	Provider      string         `json:"provider"`
	ExtractorType string         `json:"extractor_type,omitempty"`
	ConfigID      string         `json:"config_id"`
	Schedule      string         `json:"schedule,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
}

// RunResponse mirrors the shape of individual run records.
type RunResponse struct {
	ID            string         `json:"id"`
	ExtractorID   string         `json:"extractor_id"`
	ExtractorName string         `json:"extractor_name"`
	Provider      string         `json:"provider"`
	Status        string         `json:"status"`
	StartedAt     string         `json:"started_at"`
	CompletedAt   string         `json:"completed_at"`
	DurationSecs  float64        `json:"duration_secs"`
	Params        map[string]any `json:"params"`
	Error         string         `json:"error"`
	Raw           map[string]any
}

// RunListResponse is the paginated wrapper returned by GET /api/v1/extractors/runs.
type RunListResponse struct {
	Data     []map[string]any `json:"data"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	HasNext  bool             `json:"has_next"`
}

// TriggerRequest is the body for POST /api/v1/extractors/run.
type TriggerRequest struct {
	ExtractorID string         `json:"extractor_id"`
	Params      map[string]any `json:"params,omitempty"`
}

// TriggerResponse holds the run_id returned by the trigger endpoint.
type TriggerResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
	// Some server versions return the full run object.
	ID string `json:"id"`
}

// RunID_ returns the non-empty run identifier, checking both RunID and ID fields.
// The trailing underscore distinguishes the method from the RunID field.
func (r *TriggerResponse) RunID_() string {
	if r.RunID != "" {
		return r.RunID
	}
	return r.ID
}

// LogsResponse is the body returned by GET /api/v1/extractors/runs/{run_id}/logs.
// The schema is additionalProperties:true; we handle both array-of-strings and
// array-of-objects shapes.
type LogsResponse struct {
	// Lines contains log lines if the server returns a string array.
	Lines []string
	// Entries contains structured log entries if the server returns objects.
	Entries []LogEntry
}

// LogEntry is a single structured log line.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// ---- API methods ------------------------------------------------------------

// ListExtractors returns extractors, optionally filtered by provider.
func (c *Client) ListExtractors(ctx context.Context, provider string, limit int) ([]ExtractorResponse, error) {
	q := url.Values{}
	if provider != "" {
		q.Set("provider", provider)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v1/extractors"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	var wrapper ExtractorListResponse
	if err := c.GetJSON(ctx, path, &wrapper); err != nil {
		return nil, err
	}
	out := make([]ExtractorResponse, 0, len(wrapper.Data))
	for _, raw := range wrapper.Data {
		out = append(out, parseExtractor(raw))
	}
	return out, nil
}

// GetExtractor fetches a single extractor by ID.
func (c *Client) GetExtractor(ctx context.Context, id string) (*ExtractorResponse, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/extractors/"+id, &raw); err != nil {
		return nil, err
	}
	e := parseExtractor(raw)
	return &e, nil
}

// RegisterExtractor creates a new extractor.
func (c *Client) RegisterExtractor(ctx context.Context, req ExtractorCreate) (*ExtractorResponse, error) {
	body := map[string]any{
		"name":      req.Name,
		"provider":  req.Provider,
		"config_id": req.ConfigID,
	}
	if req.ExtractorType != "" {
		body["extractor_type"] = req.ExtractorType
	}
	if req.Schedule != "" {
		body["schedule"] = req.Schedule
	}
	if req.Config != nil {
		body["config"] = req.Config
	}
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/extractors", body)
	if err != nil {
		return nil, err
	}
	e := parseExtractor(*result)
	return &e, nil
}

// DeleteExtractor removes an extractor. Returns nil on 204.
func (c *Client) DeleteExtractor(ctx context.Context, id string) error {
	resp, err := c.Do(ctx, http.MethodDelete, "/api/v1/extractors/"+id, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

// TriggerRun triggers an extractor run via POST /api/v1/extractors/run.
func (c *Client) TriggerRun(ctx context.Context, req TriggerRequest) (*TriggerResponse, error) {
	body := map[string]any{
		"extractor_id": req.ExtractorID,
	}
	if req.Params != nil {
		for k, v := range req.Params {
			body[k] = v
		}
	}
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/extractors/run", body)
	if err != nil {
		return nil, err
	}
	raw := *result
	tr := &TriggerResponse{}
	if v, ok := raw["run_id"].(string); ok {
		tr.RunID = v
	}
	if v, ok := raw["id"].(string); ok {
		tr.ID = v
	}
	if v, ok := raw["status"].(string); ok {
		tr.Status = v
	}
	return tr, nil
}

// ListRuns returns extractor runs, optionally filtered.
func (c *Client) ListRuns(ctx context.Context, extractorID, status string, limit int) ([]RunResponse, error) {
	q := url.Values{}
	if extractorID != "" {
		q.Set("extractor_id", extractorID)
	}
	if status != "" {
		q.Set("status", status)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v1/extractors/runs"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	var wrapper RunListResponse
	if err := c.GetJSON(ctx, path, &wrapper); err != nil {
		return nil, err
	}
	out := make([]RunResponse, 0, len(wrapper.Data))
	for _, raw := range wrapper.Data {
		out = append(out, parseRun(raw))
	}
	return out, nil
}

// GetRun fetches a single run by ID.
func (c *Client) GetRun(ctx context.Context, runID string) (*RunResponse, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/extractors/runs/"+runID, &raw); err != nil {
		return nil, err
	}
	r := parseRun(raw)
	return &r, nil
}

// CancelRun posts to /api/v1/extractors/runs/{run_id}/cancel.
func (c *Client) CancelRun(ctx context.Context, runID string) (map[string]any, error) {
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/extractors/runs/"+runID+"/cancel", nil)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// GetRunLogs returns the log output for a run. The server schema is
// additionalProperties:true; we handle both string-array and object-array shapes.
func (c *Client) GetRunLogs(ctx context.Context, runID string) (*LogsResponse, error) {
	var raw any
	if err := c.GetJSON(ctx, "/api/v1/extractors/runs/"+runID+"/logs", &raw); err != nil {
		return nil, err
	}
	return parseLogsResponse(raw), nil
}

// ---- parsing helpers --------------------------------------------------------

func parseExtractor(raw map[string]any) ExtractorResponse {
	e := ExtractorResponse{Raw: raw}
	strField(raw, "id", &e.ID)
	strField(raw, "name", &e.Name)
	strField(raw, "provider", &e.Provider)
	strField(raw, "config_id", &e.ConfigID)
	strField(raw, "config_name", &e.ConfigName)
	strField(raw, "schedule", &e.Schedule)
	strField(raw, "last_run", &e.LastRun)
	strField(raw, "status", &e.Status)
	if v, ok := raw["enabled"].(bool); ok {
		e.Enabled = v
	}
	return e
}

func parseRun(raw map[string]any) RunResponse {
	r := RunResponse{Raw: raw}
	strField(raw, "id", &r.ID)
	strField(raw, "extractor_id", &r.ExtractorID)
	strField(raw, "extractor_name", &r.ExtractorName)
	strField(raw, "provider", &r.Provider)
	strField(raw, "status", &r.Status)
	strField(raw, "started_at", &r.StartedAt)
	strField(raw, "completed_at", &r.CompletedAt)
	strField(raw, "error", &r.Error)
	if v, ok := raw["duration_secs"].(float64); ok {
		r.DurationSecs = v
	}
	if v, ok := raw["params"].(map[string]any); ok {
		r.Params = v
	}
	return r
}

func parseLogsResponse(raw any) *LogsResponse {
	lr := &LogsResponse{}
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			switch s := item.(type) {
			case string:
				lr.Lines = append(lr.Lines, s)
			case map[string]any:
				entry := LogEntry{}
				strField(s, "timestamp", &entry.Timestamp)
				strField(s, "level", &entry.Level)
				// message may be under "message" or "msg"
				strField(s, "message", &entry.Message)
				if entry.Message == "" {
					strField(s, "msg", &entry.Message)
				}
				lr.Entries = append(lr.Entries, entry)
			}
		}
	case map[string]any:
		// Some servers wrap in {"logs": [...]}
		if logs, ok := v["logs"]; ok {
			return parseLogsResponse(logs)
		}
		if lines, ok := v["lines"]; ok {
			return parseLogsResponse(lines)
		}
	}
	return lr
}

// strField extracts a string field from a raw map into dst.
func strField(m map[string]any, key string, dst *string) {
	if v, ok := m[key].(string); ok {
		*dst = v
	}
}

// IsTerminalStatus returns true when the run status is a terminal state.
func IsTerminalStatus(status string) bool {
	switch status {
	case "completed", "failed", "cancelled", "error", "success":
		return true
	}
	return false
}
