package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// APIError is the decoded shape of a non-2xx response from the finna API.
// FastAPI returns either {"detail": "string"} for ordinary errors or
// {"detail": [{"loc": [...], "msg": "...", "type": "..."}, ...]} for 422
// validation errors. Both shapes are normalised here.
type APIError struct {
	StatusCode int
	// Message is the plain-string detail (empty for 422 validation errors).
	Message string
	// Validation is populated only for 422 responses.
	Validation []ValidationItem
	// Raw is the original response body, kept for --debug surfaces.
	Raw []byte
}

// ValidationItem mirrors one entry of FastAPI's 422 detail array.
type ValidationItem struct {
	Loc  []string `json:"loc"`
	Msg  string   `json:"msg"`
	Type string   `json:"type"`
}

// Error renders a human-friendly one-liner. For 422 errors with multiple
// fields, use ui.FormatAPIError for a multi-line breakdown.
func (e *APIError) Error() string {
	if e == nil {
		return "<nil APIError>"
	}
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
	}
	if len(e.Validation) > 0 {
		parts := make([]string, 0, len(e.Validation))
		for _, v := range e.Validation {
			parts = append(parts, fmt.Sprintf("%s: %s", strings.Join(v.Loc, "."), v.Msg))
		}
		return fmt.Sprintf("HTTP %d: validation failed: %s", e.StatusCode, strings.Join(parts, "; "))
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// DecodeError reads an HTTP response body and constructs an APIError. The
// body is consumed; callers should not read resp.Body afterwards.
func DecodeError(resp *http.Response) *APIError {
	e := &APIError{StatusCode: resp.StatusCode}
	body, _ := io.ReadAll(resp.Body)
	e.Raw = body
	if len(body) == 0 {
		return e
	}

	// FastAPI puts the payload under "detail". We unmarshal into a
	// json.RawMessage so we can branch on string vs array shape.
	var env struct {
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(body, &env); err != nil || len(env.Detail) == 0 {
		// Not a FastAPI envelope; fall back to raw body as message.
		e.Message = strings.TrimSpace(string(body))
		return e
	}

	// Try string first.
	var s string
	if err := json.Unmarshal(env.Detail, &s); err == nil {
		e.Message = s
		return e
	}

	// Try validation list.
	var items []ValidationItem
	if err := json.Unmarshal(env.Detail, &items); err == nil {
		// FastAPI sometimes returns mixed-type loc entries (string or int).
		// Re-parse leniently if all-string failed silently.
		if len(items) == 0 {
			e.Message = string(env.Detail)
			return e
		}
		e.Validation = items
		return e
	}

	// Lenient re-parse: handle loc entries that are integers.
	var raw []map[string]any
	if err := json.Unmarshal(env.Detail, &raw); err == nil {
		for _, r := range raw {
			item := ValidationItem{}
			if msg, ok := r["msg"].(string); ok {
				item.Msg = msg
			}
			if t, ok := r["type"].(string); ok {
				item.Type = t
			}
			if locs, ok := r["loc"].([]any); ok {
				for _, l := range locs {
					item.Loc = append(item.Loc, fmt.Sprintf("%v", l))
				}
			}
			e.Validation = append(e.Validation, item)
		}
		return e
	}

	// Last resort: stash the raw detail as message.
	e.Message = string(env.Detail)
	return e
}
