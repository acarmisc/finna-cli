package api

import (
	"context"
	"strconv"
)

// DashboardStats is the shape returned by GET /api/v1/dashboard/stats.
type DashboardStats struct {
	Totals     map[string]float64 `json:"totals"`
	Daily      []DailyEntry       `json:"-"` // parsed from raw
	AlertStats AlertStats         `json:"-"` // parsed from raw
	Raw        map[string]any
}

// ActiveAlertItem is one entry from GET /api/v1/alerts/active.
// (AlertStats is defined in alerts.go; WastageSummaryItem in wastage.go)
type ActiveAlertItem struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
	Rule        string  `json:"rule"`
	Project     string  `json:"project"`
	CostImpact  float64 `json:"cost_impact"`
	Provider    string  `json:"provider"`
	TriggeredAt string  `json:"triggered_at"`
}

// GetDashboardStats calls GET /api/v1/dashboard/stats.
func (c *Client) GetDashboardStats(ctx context.Context, window string) (*DashboardStats, error) {
	path := "/api/v1/dashboard/stats"
	if window != "" {
		path += "?range=" + window
	}
	var raw map[string]any
	if err := c.GetJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	return parseDashboardStats(raw), nil
}

// GetActiveAlerts calls GET /api/v1/alerts/active.
func (c *Client) GetActiveAlerts(ctx context.Context, limit int) ([]ActiveAlertItem, error) {
	path := "/api/v1/alerts/active"
	if limit > 0 {
		path += "?limit=" + intStr(limit)
	}
	var raw map[string]any
	if err := c.GetJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	return parseActiveAlerts(raw), nil
}

// GetWastageSummary calls GET /api/v1/wastage/summary.
func (c *Client) GetWastageSummary(ctx context.Context) ([]WastageSummaryItem, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/wastage/summary", &raw); err != nil {
		return nil, err
	}
	return parseWastageSummaryDashboard(raw), nil
}

// ---- parsing ----------------------------------------------------------------

func parseDashboardStats(raw map[string]any) *DashboardStats {
	d := &DashboardStats{Raw: raw, Totals: map[string]float64{}}
	if v, ok := raw["totals"].(map[string]any); ok {
		d.Totals = floatMap(v)
	}
	if v, ok := raw["daily"]; ok {
		d.Daily = parseDailyList(v)
	}
	if v, ok := raw["alertStats"].(map[string]any); ok {
		d.AlertStats = parseAlertStats(v)
	}
	return d
}

// AlertStats holds counts from the alertStats block.
// AlertStatsResponse (in alerts.go) embeds this type.
type AlertStats struct {
	Firing     int            `json:"firing"`
	Ack        int            `json:"ack"`
	Resolved   int            `json:"resolved"`
	BySeverity map[string]int `json:"by_severity"`
}

func parseAlertStats(m map[string]any) AlertStats {
	a := AlertStats{BySeverity: make(map[string]int)}
	intField := func(key string, dst *int) {
		if f, ok := m[key].(float64); ok {
			*dst = int(f)
		}
	}
	intField("firing", &a.Firing)
	intField("ack", &a.Ack)
	intField("resolved", &a.Resolved)
	if sev, ok := m["by_severity"].(map[string]any); ok {
		for k, v := range sev {
			if f, ok := v.(float64); ok {
				a.BySeverity[k] = int(f)
			}
		}
	}
	return a
}

func parseActiveAlerts(raw map[string]any) []ActiveAlertItem {
	var arr []any
	for _, key := range []string{"alerts", "data", "items"} {
		if v, ok := raw[key].([]any); ok {
			arr = v
			break
		}
	}
	if arr == nil {
		return nil
	}
	out := make([]ActiveAlertItem, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		a := ActiveAlertItem{}
		strField(m, "id", &a.ID)
		strField(m, "status", &a.Status)
		strField(m, "severity", &a.Severity)
		strField(m, "description", &a.Description)
		strField(m, "rule", &a.Rule)
		strField(m, "project", &a.Project)
		strField(m, "provider", &a.Provider)
		strField(m, "triggered_at", &a.TriggeredAt)
		if f, ok := m["cost_impact"].(float64); ok {
			a.CostImpact = f
		}
		out = append(out, a)
	}
	return out
}

func parseWastageSummaryDashboard(raw map[string]any) []WastageSummaryItem {
	for _, key := range []string{"summary", "categories", "data"} {
		if v, ok := raw[key].([]any); ok {
			return parseWastageItemList(v)
		}
	}
	var out []WastageSummaryItem
	for k, v := range raw {
		switch val := v.(type) {
		case float64:
			out = append(out, WastageSummaryItem{Category: k, EstimatedSavings: val})
		case map[string]any:
			item := WastageSummaryItem{Category: k}
			if f, ok := val["estimated_savings"].(float64); ok {
				item.EstimatedSavings = f
			}
			if f, ok := val["count"].(float64); ok {
				item.Count = int(f)
			}
			out = append(out, item)
		}
	}
	return out
}

func parseWastageItemList(arr []any) []WastageSummaryItem {
	out := make([]WastageSummaryItem, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		w := WastageSummaryItem{}
		strField(m, "category", &w.Category)
		if f, ok := m["estimated_savings"].(float64); ok {
			w.EstimatedSavings = f
		}
		if f, ok := m["count"].(float64); ok {
			w.Count = int(f)
		}
		out = append(out, w)
	}
	return out
}

func intStr(n int) string {
	return strconv.Itoa(n)
}
