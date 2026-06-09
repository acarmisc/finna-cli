package api

import (
	"context"
	"net/url"
	"strconv"
)

// ---- types ------------------------------------------------------------------

// AlertResponse represents a single alert record from the list/active endpoints.
type AlertResponse struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	Severity       string  `json:"severity"`
	Description    string  `json:"description"`
	Rule           string  `json:"rule"`
	Project        string  `json:"project"`
	CostImpact     float64 `json:"cost_impact"`
	Provider       string  `json:"provider"`
	IsAcknowledged bool    `json:"is_acknowledged"`
	TriggeredAt    string  `json:"triggered_at"`
	FirstSeen      string  `json:"first_seen"`
	LastSeen       string  `json:"last_seen"`
	Raw            map[string]any
}

// AlertListResponse is the paginated wrapper returned by GET /api/v1/alerts.
type AlertListResponse struct {
	Alerts   []AlertResponse
	Count    int
	Total    int
	Page     int
	PageSize int
	HasNext  bool
	HasPrev  bool
}

// AlertStatsResponse holds the counts returned by GET /api/v1/alerts/stats.
// It extends the embedded AlertStats (from dashboard.go) with additional fields.
type AlertStatsResponse struct {
	AlertStats              // embedded: Firing, Ack, Resolved, BySeverity
	Total  int
	Active int
	Raw    map[string]any
}

// AlertAckResponse is the body returned by acknowledge endpoints.
type AlertAckResponse struct {
	Count int
	IDs   []string
	Raw   map[string]any
}

// AlertQuery holds optional filters for listing alerts.
type AlertQuery struct {
	Status   string
	Severity string
	Limit    int
	Page     int
}

// ---- API methods ------------------------------------------------------------

// ListAlerts returns alerts, optionally filtered.
func (c *Client) ListAlerts(ctx context.Context, q AlertQuery) (*AlertListResponse, error) {
	params := url.Values{}
	if q.Status != "" {
		params.Set("status", q.Status)
	}
	if q.Severity != "" {
		params.Set("severity", q.Severity)
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Page > 0 {
		params.Set("page", strconv.Itoa(q.Page))
	}
	path := "/api/v1/alerts"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var raw map[string]any
	if err := c.GetJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	return parseAlertListResponse(raw), nil
}

// ListActiveAlerts returns only firing/unacknowledged alerts.
func (c *Client) ListActiveAlerts(ctx context.Context) (*AlertListResponse, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/alerts/active", &raw); err != nil {
		return nil, err
	}
	return parseAlertListResponse(raw), nil
}

// GetAlertStats returns alert counts by status and severity.
func (c *Client) GetAlertStats(ctx context.Context) (*AlertStatsResponse, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/alerts/stats", &raw); err != nil {
		return nil, err
	}
	return parseAlertStatsResponse(raw), nil
}

// AcknowledgeAlert POSTs to /api/v1/alerts/{alert_id}/acknowledge.
func (c *Client) AcknowledgeAlert(ctx context.Context, alertID string) (*AlertAckResponse, error) {
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/alerts/"+alertID+"/acknowledge", nil)
	if err != nil {
		return nil, err
	}
	return parseAlertAck(*result), nil
}

// AcknowledgeAllAlerts POSTs to /api/v1/alerts/acknowledge-all.
func (c *Client) AcknowledgeAllAlerts(ctx context.Context) (*AlertAckResponse, error) {
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/alerts/acknowledge-all", nil)
	if err != nil {
		return nil, err
	}
	return parseAlertAck(*result), nil
}

// ---- parsing helpers --------------------------------------------------------

func parseAlertListResponse(raw map[string]any) *AlertListResponse {
	out := &AlertListResponse{}
	for _, pair := range []struct {
		key string
		dst *int
	}{
		{"count", &out.Count},
		{"total", &out.Total},
		{"page", &out.Page},
		{"page_size", &out.PageSize},
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
	// Items may be under "alerts" or "data".
	var items []any
	for _, key := range []string{"alerts", "data"} {
		if v, ok := raw[key].([]any); ok {
			items = v
			break
		}
	}
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out.Alerts = append(out.Alerts, parseAlertItem(m))
		}
	}
	return out
}

func parseAlertItem(raw map[string]any) AlertResponse {
	a := AlertResponse{Raw: raw}
	strField(raw, "id", &a.ID)
	strField(raw, "status", &a.Status)
	strField(raw, "severity", &a.Severity)
	strField(raw, "description", &a.Description)
	strField(raw, "rule", &a.Rule)
	strField(raw, "project", &a.Project)
	strField(raw, "provider", &a.Provider)
	strField(raw, "triggered_at", &a.TriggeredAt)
	strField(raw, "first_seen", &a.FirstSeen)
	strField(raw, "last_seen", &a.LastSeen)
	if v, ok := raw["cost_impact"].(float64); ok {
		a.CostImpact = v
	}
	if v, ok := raw["is_acknowledged"].(bool); ok {
		a.IsAcknowledged = v
	}
	return a
}

func parseAlertStatsResponse(raw map[string]any) *AlertStatsResponse {
	s := &AlertStatsResponse{
		Raw:        raw,
		AlertStats: parseAlertStats(raw),
	}
	for _, pair := range []struct {
		key string
		dst *int
	}{
		{"total", &s.Total},
		{"active", &s.Active},
	} {
		if v, ok := raw[pair.key].(float64); ok {
			*pair.dst = int(v)
		}
	}
	return s
}

func parseAlertAck(raw map[string]any) *AlertAckResponse {
	a := &AlertAckResponse{Raw: raw}
	if v, ok := raw["count"].(float64); ok {
		a.Count = int(v)
	}
	if ids, ok := raw["ids"].([]any); ok {
		for _, id := range ids {
			if s, ok := id.(string); ok {
				a.IDs = append(a.IDs, s)
			}
		}
	}
	return a
}
