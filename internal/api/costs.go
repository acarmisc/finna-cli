package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// ---- shared query-param builder ---------------------------------------------

type costQuery struct {
	Provider  string
	Project   string
	Window    string
	StartDate string
	EndDate   string
	Limit     int
	Page      int
	PageSize  int
}

func (q costQuery) encode() string {
	v := url.Values{}
	set := func(k, val string) {
		if val != "" {
			v.Set(k, val)
		}
	}
	set("provider", q.Provider)
	set("project", q.Project)
	set("window", q.Window)
	set("start_date", q.StartDate)
	set("end_date", q.EndDate)
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Page > 0 {
		v.Set("page", strconv.Itoa(q.Page))
	}
	if q.PageSize > 0 {
		v.Set("page_size", strconv.Itoa(q.PageSize))
	}
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}

// ---- response types ---------------------------------------------------------

// CostRecord is a single cost entry from GET /api/v1/costs.
type CostRecord struct {
	ID        string         `json:"id"`
	Provider  string         `json:"prov"`
	ProjectID string         `json:"project_id"`
	Name      string         `json:"name"`
	SKU       string         `json:"sku"`
	MTD       float64        `json:"mtd"`
	Delta     float64        `json:"delta"`
	Prev      float64        `json:"prev"`
	Date      string         `json:"date"`
	Tags      map[string]any `json:"tags"`
}

// CostListResponse is the paginated shape returned by GET /api/v1/costs.
type CostListResponse struct {
	Costs    []CostRecord       `json:"costs"`
	Totals   map[string]float64 `json:"totals"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	HasNext  bool               `json:"has_next"`
	HasPrev  bool               `json:"has_prev"`
	Window   string             `json:"window"`
}

// CostSummaryResponse is the shape from GET /api/v1/costs/summary.
// The server returns additionalProperties:true; we capture key fields.
type CostSummaryResponse struct {
	ByProvider map[string]float64            `json:"by_provider"`
	ByService  map[string]float64            `json:"by_service"`
	Total      float64                       `json:"total"`
	Window     string                        `json:"window"`
	Raw        map[string]any
}

// ProviderTotals holds current/prev/delta for one provider (from /costs/totals).
type ProviderTotals struct {
	MTD   float64 `json:"mtd"`
	Prev  float64 `json:"prev"`
	Delta float64 `json:"delta"`
}

// CostTotalsResponse is the shape from GET /api/v1/costs/totals.
type CostTotalsResponse struct {
	Totals    map[string]ProviderTotals `json:"totals"`
	Window    string                    `json:"window"`
	StartDate string                    `json:"startDate"`
	EndDate   string                    `json:"endDate"`
	Raw       map[string]any
}

// BreakdownItem is one SKU entry from GET /api/v1/costs/breakdown.
type BreakdownItem struct {
	SKU      string  `json:"service_name"`
	Provider string  `json:"provider"`
	Total    float64 `json:"total"`
}

// DailyEntry is one date row from GET /api/v1/costs/daily.
type DailyEntry struct {
	Date  string             `json:"date"`
	Azure float64            `json:"azure"`
	GCP   float64            `json:"gcp"`
	LLM   float64            `json:"llm"`
	Total float64            `json:"total"`
	Extra map[string]float64 // catches any other provider keys
}

// SKUItem is one entry from GET /api/v1/costs/by-sku.
type SKUItem struct {
	SKU   string  `json:"sku"`
	Total float64 `json:"total"`
}

// ---- API methods ------------------------------------------------------------

// ListCosts returns paginated cost records.
func (c *Client) ListCosts(ctx context.Context, q costQuery) (*CostListResponse, error) {
	var out CostListResponse
	if err := c.GetJSON(ctx, "/api/v1/costs"+q.encode(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCostSummary calls GET /api/v1/costs/summary.
func (c *Client) GetCostSummary(ctx context.Context, q costQuery) (*CostSummaryResponse, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/costs/summary"+q.encode(), &raw); err != nil {
		return nil, err
	}
	return parseCostSummary(raw), nil
}

// GetCostTotals calls GET /api/v1/costs/totals.
func (c *Client) GetCostTotals(ctx context.Context, q costQuery) (*CostTotalsResponse, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/costs/totals"+q.encode(), &raw); err != nil {
		return nil, err
	}
	return parseCostTotals(raw), nil
}

// GetCostBreakdown calls GET /api/v1/costs/breakdown.
func (c *Client) GetCostBreakdown(ctx context.Context, q costQuery) ([]BreakdownItem, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/costs/breakdown"+q.encode(), &raw); err != nil {
		return nil, err
	}
	return parseBreakdown(raw), nil
}

// GetDailyCosts calls GET /api/v1/costs/daily.
func (c *Client) GetDailyCosts(ctx context.Context, q costQuery) ([]DailyEntry, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/costs/daily"+q.encode(), &raw); err != nil {
		return nil, err
	}
	return parseDailyEntries(raw), nil
}

// GetCostsBySKU calls GET /api/v1/costs/by-sku.
func (c *Client) GetCostsBySKU(ctx context.Context, q costQuery) ([]SKUItem, error) {
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/costs/by-sku"+q.encode(), &raw); err != nil {
		return nil, err
	}
	return parseSKUItems(raw), nil
}

// GetSKUs calls GET /api/v1/costs/skus.
func (c *Client) GetSKUs(ctx context.Context, provider string) ([]string, error) {
	q := url.Values{"provider": {provider}}
	var raw map[string]any
	if err := c.GetJSON(ctx, "/api/v1/costs/skus?"+q.Encode(), &raw); err != nil {
		return nil, err
	}
	return parseSKUList(raw), nil
}

// ExportCosts streams the CSV export to w and returns byte count.
func (c *Client) ExportCosts(ctx context.Context, q costQuery, w io.Writer) (int64, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/api/v1/costs/export"+q.encode(), nil, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return n, fmt.Errorf("stream export: %w", err)
	}
	return n, nil
}

// ---- parsing helpers --------------------------------------------------------

func parseCostSummary(raw map[string]any) *CostSummaryResponse {
	r := &CostSummaryResponse{Raw: raw}
	if v, ok := raw["by_provider"].(map[string]any); ok {
		r.ByProvider = floatMap(v)
	}
	if v, ok := raw["by_service"].(map[string]any); ok {
		r.ByService = floatMap(v)
	}
	if v, ok := raw["total"].(float64); ok {
		r.Total = v
	}
	if v, ok := raw["window"].(string); ok {
		r.Window = v
	}
	return r
}

func parseCostTotals(raw map[string]any) *CostTotalsResponse {
	r := &CostTotalsResponse{Raw: raw}
	if v, ok := raw["window"].(string); ok {
		r.Window = v
	}
	if v, ok := raw["startDate"].(string); ok {
		r.StartDate = v
	}
	if v, ok := raw["endDate"].(string); ok {
		r.EndDate = v
	}
	r.Totals = make(map[string]ProviderTotals)
	if totals, ok := raw["totals"].(map[string]any); ok {
		for provider, entry := range totals {
			if em, ok := entry.(map[string]any); ok {
				pt := ProviderTotals{}
				if v, ok := em["mtd"].(float64); ok {
					pt.MTD = v
				}
				if v, ok := em["prev"].(float64); ok {
					pt.Prev = v
				}
				if v, ok := em["delta"].(float64); ok {
					pt.Delta = v
				}
				r.Totals[provider] = pt
			}
		}
	}
	return r
}

func parseBreakdown(raw map[string]any) []BreakdownItem {
	// Try "items" key first, then "breakdown", then root array.
	for _, key := range []string{"items", "breakdown", "data"} {
		if v, ok := raw[key]; ok {
			return parseBreakdownList(v)
		}
	}
	// Fall back: treat each key as sku -> total.
	var out []BreakdownItem
	for k, v := range raw {
		if f, ok := v.(float64); ok {
			out = append(out, BreakdownItem{SKU: k, Total: f})
		}
	}
	return out
}

func parseBreakdownList(v any) []BreakdownItem {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]BreakdownItem, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		bi := BreakdownItem{}
		if s, ok := m["service_name"].(string); ok {
			bi.SKU = s
		} else if s, ok := m["sku"].(string); ok {
			bi.SKU = s
		}
		if s, ok := m["provider"].(string); ok {
			bi.Provider = s
		}
		if f, ok := m["total"].(float64); ok {
			bi.Total = f
		}
		out = append(out, bi)
	}
	return out
}

func parseDailyEntries(raw map[string]any) []DailyEntry {
	// Server returns {"daily": [...]} or {"data": [...]} or bare array.
	for _, key := range []string{"daily", "data", "days"} {
		if v, ok := raw[key]; ok {
			return parseDailyList(v)
		}
	}
	return nil
}

func parseDailyList(v any) []DailyEntry {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]DailyEntry, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		de := DailyEntry{Extra: make(map[string]float64)}
		if s, ok := m["date"].(string); ok {
			de.Date = s
		}
		knownProviders := map[string]*float64{
			"azure": &de.Azure,
			"gcp":   &de.GCP,
			"llm":   &de.LLM,
			"total": &de.Total,
		}
		for k, fptr := range knownProviders {
			if f, ok := m[k].(float64); ok {
				*fptr = f
			}
		}
		for k, val := range m {
			if k == "date" {
				continue
			}
			if f, ok := val.(float64); ok {
				if _, known := knownProviders[k]; !known {
					de.Extra[k] = f
				}
			}
		}
		out = append(out, de)
	}
	return out
}

func parseSKUItems(raw map[string]any) []SKUItem {
	for _, key := range []string{"items", "data", "skus"} {
		if v, ok := raw[key]; ok {
			if arr, ok := v.([]any); ok {
				out := make([]SKUItem, 0, len(arr))
				for _, item := range arr {
					m, ok := item.(map[string]any)
					if !ok {
						continue
					}
					si := SKUItem{}
					if s, ok := m["sku"].(string); ok {
						si.SKU = s
					} else if s, ok := m["service_name"].(string); ok {
						si.SKU = s
					}
					if f, ok := m["total"].(float64); ok {
						si.Total = f
					}
					out = append(out, si)
				}
				return out
			}
		}
	}
	// Fall back: treat keys as sku -> total.
	var out []SKUItem
	for k, v := range raw {
		if f, ok := v.(float64); ok {
			out = append(out, SKUItem{SKU: k, Total: f})
		}
	}
	return out
}

func parseSKUList(raw map[string]any) []string {
	for _, key := range []string{"skus", "data", "items"} {
		if v, ok := raw[key]; ok {
			if arr, ok := v.([]any); ok {
				out := make([]string, 0, len(arr))
				for _, item := range arr {
					if s, ok := item.(string); ok {
						out = append(out, s)
					}
				}
				return out
			}
		}
	}
	return nil
}

func floatMap(m map[string]any) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		if f, ok := v.(float64); ok {
			out[k] = f
		}
	}
	return out
}

// CostQuery is the exported alias used by CLI commands.
type CostQuery = costQuery
