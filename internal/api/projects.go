package api

import (
	"context"
	"net/http"
	"net/url"
)

// ProjectResponse is the shape returned by /api/v1/config/projects endpoints.
// The server returns additionalProperties:true so we use a flexible map with
// well-known top-level fields extracted for convenience.
type ProjectResponse struct {
	Slug string  `json:"slug"`
	Name string  `json:"name"`
	MTD  float64 `json:"mtd"`
	// Raw holds the full decoded payload for forward-compat display.
	Raw map[string]any
}

// ProjectCreate is the body for POST /api/v1/config/projects.
type ProjectCreate struct {
	Slug string `json:"slug,omitempty"`
	Name string `json:"name"`
}

// ListProjects returns all projects, optionally for a time window.
func (c *Client) ListProjects(ctx context.Context, window string) ([]ProjectResponse, error) {
	path := "/api/v1/config/projects"
	if window != "" {
		q := url.Values{"window": {window}}
		path += "?" + q.Encode()
	}
	var raws []map[string]any
	if err := c.GetJSON(ctx, path, &raws); err != nil {
		return nil, err
	}
	out := make([]ProjectResponse, 0, len(raws))
	for _, r := range raws {
		p := parseProject(r)
		out = append(out, p)
	}
	return out, nil
}

// GetProject fetches a single project by slug.
func (c *Client) GetProject(ctx context.Context, slug, window string) (*ProjectResponse, error) {
	path := "/api/v1/config/projects/" + slug
	if window != "" {
		q := url.Values{"window": {window}}
		path += "?" + q.Encode()
	}
	var raw map[string]any
	if err := c.GetJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	p := parseProject(raw)
	return &p, nil
}

// CreateProject posts to /api/v1/config/projects.
func (c *Client) CreateProject(ctx context.Context, req ProjectCreate) (*ProjectResponse, error) {
	body := map[string]any{"name": req.Name}
	if req.Slug != "" {
		body["slug"] = req.Slug
	}
	result, err := postJSON[map[string]any](ctx, c, "/api/v1/config/projects", body)
	if err != nil {
		return nil, err
	}
	p := parseProject(*result)
	return &p, nil
}

// DeleteProject deletes via DELETE /api/v1/config/projects/{slug}. Returns nil on 204.
func (c *Client) DeleteProject(ctx context.Context, slug string) error {
	resp, err := c.Do(ctx, http.MethodDelete, "/api/v1/config/projects/"+slug, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

// parseProject normalises the raw map returned by the server into a
// ProjectResponse, tolerating missing fields gracefully.
func parseProject(r map[string]any) ProjectResponse {
	p := ProjectResponse{Raw: r}
	if v, ok := r["slug"].(string); ok {
		p.Slug = v
	}
	if v, ok := r["name"].(string); ok {
		p.Name = v
	}
	// MTD may be under "mtd" or "total_cost".
	for _, key := range []string{"mtd", "total_cost", "cost"} {
		if v, ok := r[key].(float64); ok {
			p.MTD = v
			break
		}
	}
	return p
}
