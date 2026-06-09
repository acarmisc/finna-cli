package api

import (
	"context"
	"net/url"
	"strconv"
)

// ---- types ------------------------------------------------------------------

// WastageFinding represents a single resource wastage finding.
type WastageFinding struct {
	ID                      string         `json:"id"`
	Provider                string         `json:"provider"`
	RuleID                  string         `json:"rule_id"`
	RuleName                string         `json:"rule_name"`
	Severity                string         `json:"severity"`
	Status                  string         `json:"status"`
	EstimatedMonthlySavings float64        `json:"estimated_monthly_usd"`
	ResourceID              string         `json:"resource_id"`
	Category                string         `json:"category"`
	AccountID               string         `json:"account_id"`
	FirstSeenAt             string         `json:"first_seen_at"`
	LastSeenAt              string         `json:"last_seen_at"`
	DetectedAt              string         `json:"detected_at"`
	Raw                     map[string]any
}

// WastageFindingListResponse is the wrapper returned by GET /api/v1/wastage.
type WastageFindingListResponse struct {
	Data    []WastageFinding
	Total   int
	Limit   int
	Offset  int
	HasNext bool
	HasPrev bool
}

// WastageSummaryItem is one entry from the wastage summary by category.
// It is also used by dashboard.go.
type WastageSummaryItem struct {
	Category         string  `json:"category"`
	EstimatedSavings float64 `json:"estimated_savings"`
	Count            int     `json:"count"`
}

// WastageRule is a single entry from the rule catalog.
type WastageRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Provider    string         `json:"provider"`
	Severity    string         `json:"severity"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Raw         map[string]any
}

// WastageScan is a single scan run record.
type WastageScan struct {
	ID           string         `json:"id"`
	ScanID       string         `json:"scan_id"`
	Status       string         `json:"status"`
	Provider     string         `json:"provider"`
	StartedAt    string         `json:"started_at"`
	FinishedAt   string         `json:"finished_at"`
	FindingCount int            `json:"finding_count"`
	DurationSecs float64        `json:"duration_secs"`
	Raw          map[string]any
}

// WastageScanTriggerResponse is returned by POST /api/v1/wastage/scan.
type WastageScanTriggerResponse struct {
	ScanID string         `json:"scan_id"`
	ID     string         `json:"id"`
	Status string         `json:"status"`
	Raw    map[string]any
}

// ScanID_ returns the non-empty scan identifier.
func (r *WastageScanTriggerResponse) ScanID_() string {
	if r.ScanID != "" {
		return r.ScanID
	}
	return r.ID
}

// WastageFindingQuery holds optional filters for listing wastage findings.
type WastageFindingQuery struct {
	Provider  string
	Severity  string
	RuleID    string
	Status    string
	AccountID string
	Limit     int
	Offset    int
}

// ---- API methods ------------------------------------------------------------

// ListWastageFindings returns wastage findings, optionally filtered.
func (c *Client) ListWastageFindings(ctx context.Context, q WastageFindingQuery) (*WastageFindingListResponse, error) {
	params := url.Values{}
	if q.Provider != "" {
		params.Set("provider", q.Provider)
	}
	if q.Severity != "" {
		params.Set("severity", q.Severity)
	}
	if q.RuleID != "" {
		params.Set("rule_id", q.RuleID)
	}
	if q.Status != "" {
		params.Set("status", q.Status)
	}
	if q.AccountID != "" {
		params.Set("account_id", q.AccountID)
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset > 0 {
		params.Set("offset", strconv.Itoa(q.Offset))
	}
	path := "/api/v1/wastage"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var raw map[string]any
	if err := c.GetJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	return parseWastageFindingList(raw), nil
}

// GetWastageFinding fetches a single finding by ID.
func (c *Client) GetWastageFinding(ctx context.Context, findingID string) (*WastageFinding, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/wastage/"+findingID, &raw); err != nil {
		return nil, err
	}
	f := parseWastageFinding(raw)
	return &f, nil
}

// AckWastageFinding transitions a finding to acked.
func (c *Client) AckWastageFinding(ctx context.Context, findingID string) (map[string]any, error) {
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/wastage/"+findingID+"/ack", nil)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// IgnoreWastageFinding transitions a finding to ignored with an optional reason.
func (c *Client) IgnoreWastageFinding(ctx context.Context, findingID, reason string) (map[string]any, error) {
	body := map[string]any{"reason": reason}
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/wastage/"+findingID+"/ignore", body)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// ResolveWastageFinding transitions a finding to resolved.
func (c *Client) ResolveWastageFinding(ctx context.Context, findingID string) (map[string]any, error) {
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/wastage/"+findingID+"/resolve", nil)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// ListWastageRules returns the rule catalog.
func (c *Client) ListWastageRules(ctx context.Context) ([]WastageRule, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/wastage/rules", &raw); err != nil {
		return nil, err
	}
	return parseWastageRules(raw), nil
}

// TriggerWastageScan POSTs to /api/v1/wastage/scan.
func (c *Client) TriggerWastageScan(ctx context.Context, provider string) (*WastageScanTriggerResponse, error) {
	body := map[string]any{}
	if provider != "" {
		body["provider"] = provider
	}
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/wastage/scan", body)
	if err != nil {
		return nil, err
	}
	raw := *result
	resp := &WastageScanTriggerResponse{Raw: raw}
	strField(raw, "scan_id", &resp.ScanID)
	strField(raw, "id", &resp.ID)
	strField(raw, "status", &resp.Status)
	return resp, nil
}

// ListWastageScans returns recent scan run records.
func (c *Client) ListWastageScans(ctx context.Context, limit int) ([]WastageScan, error) {
	path := "/api/v1/wastage/scans"
	if limit > 0 {
		path += "?limit=" + strconv.Itoa(limit)
	}
	var raw map[string]any
	if err := c.GetJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	return parseWastageScans(raw), nil
}

// ---- parsing helpers --------------------------------------------------------

func parseWastageFindingList(raw map[string]any) *WastageFindingListResponse {
	out := &WastageFindingListResponse{}
	for _, pair := range []struct {
		key string
		dst *int
	}{
		{"total", &out.Total},
		{"limit", &out.Limit},
		{"offset", &out.Offset},
	} {
		if v, ok := raw[pair.key].(float64); ok {
			*pair.dst = int(v)
		}
	}
	if v, ok := raw["has_next"].(bool); ok {
		out.HasNext = v
	}
	if v, ok := raw["has_prev"].(bool); ok {
		out.HasPrev = v
	}
	var items []any
	for _, key := range []string{"data", "findings"} {
		if v, ok := raw[key].([]any); ok {
			items = v
			break
		}
	}
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out.Data = append(out.Data, parseWastageFinding(m))
		}
	}
	return out
}

func parseWastageFinding(raw map[string]any) WastageFinding {
	f := WastageFinding{Raw: raw}
	strField(raw, "id", &f.ID)
	strField(raw, "provider", &f.Provider)
	strField(raw, "rule_id", &f.RuleID)
	strField(raw, "rule_name", &f.RuleName)
	strField(raw, "severity", &f.Severity)
	strField(raw, "status", &f.Status)
	strField(raw, "resource_id", &f.ResourceID)
	strField(raw, "category", &f.Category)
	strField(raw, "account_id", &f.AccountID)
	strField(raw, "first_seen_at", &f.FirstSeenAt)
	strField(raw, "last_seen_at", &f.LastSeenAt)
	strField(raw, "detected_at", &f.DetectedAt)
	if v, ok := raw["estimated_monthly_usd"].(float64); ok {
		f.EstimatedMonthlySavings = v
	}
	return f
}

func parseWastageRules(raw map[string]any) []WastageRule {
	var items []any
	for _, key := range []string{"rules", "data"} {
		if v, ok := raw[key].([]any); ok {
			items = v
			break
		}
	}
	out := make([]WastageRule, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		r := WastageRule{Raw: m}
		strField(m, "id", &r.ID)
		strField(m, "name", &r.Name)
		strField(m, "provider", &r.Provider)
		strField(m, "severity", &r.Severity)
		strField(m, "category", &r.Category)
		strField(m, "description", &r.Description)
		if v, ok := m["enabled"].(bool); ok {
			r.Enabled = v
		}
		out = append(out, r)
	}
	return out
}

func parseWastageScans(raw map[string]any) []WastageScan {
	var items []any
	for _, key := range []string{"scans", "data"} {
		if v, ok := raw[key].([]any); ok {
			items = v
			break
		}
	}
	out := make([]WastageScan, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		s := WastageScan{Raw: m}
		strField(m, "id", &s.ID)
		strField(m, "scan_id", &s.ScanID)
		strField(m, "status", &s.Status)
		strField(m, "provider", &s.Provider)
		strField(m, "started_at", &s.StartedAt)
		strField(m, "finished_at", &s.FinishedAt)
		if v, ok := m["finding_count"].(float64); ok {
			s.FindingCount = int(v)
		}
		if v, ok := m["duration_secs"].(float64); ok {
			s.DurationSecs = v
		}
		out = append(out, s)
	}
	return out
}

// IsTerminalScanStatus returns true when the scan status is a terminal state.
func IsTerminalScanStatus(status string) bool {
	switch status {
	case "completed", "failed", "error", "success", "done":
		return true
	}
	return false
}
