package api_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

func mkResp(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestDecodeError_StringDetail(t *testing.T) {
	e := api.DecodeError(mkResp(400, `{"detail":"bad input"}`))
	require.Equal(t, 400, e.StatusCode)
	require.Equal(t, "bad input", e.Message)
	require.Empty(t, e.Validation)
	require.Contains(t, e.Error(), "bad input")
}

func TestDecodeError_ValidationDetail(t *testing.T) {
	body := `{"detail":[{"loc":["body","email"],"msg":"value is not a valid email address","type":"value_error.email"}]}`
	e := api.DecodeError(mkResp(422, body))
	require.Equal(t, 422, e.StatusCode)
	require.Empty(t, e.Message)
	require.Len(t, e.Validation, 1)
	require.Equal(t, []string{"body", "email"}, e.Validation[0].Loc)
	require.Contains(t, e.Validation[0].Msg, "valid email")
}

func TestDecodeError_MixedTypeLoc(t *testing.T) {
	// FastAPI sometimes emits integer indices in loc (e.g. ["body", 0, "name"]).
	body := `{"detail":[{"loc":["body",0,"name"],"msg":"required","type":"missing"}]}`
	e := api.DecodeError(mkResp(422, body))
	require.Len(t, e.Validation, 1)
	require.Equal(t, []string{"body", "0", "name"}, e.Validation[0].Loc)
}

func TestDecodeError_NoEnvelope(t *testing.T) {
	e := api.DecodeError(mkResp(500, "internal boom"))
	require.Equal(t, 500, e.StatusCode)
	require.Equal(t, "internal boom", e.Message)
}

func TestDecodeError_EmptyBody(t *testing.T) {
	e := api.DecodeError(mkResp(503, ""))
	require.Equal(t, 503, e.StatusCode)
	require.Empty(t, e.Message)
	require.Empty(t, e.Validation)
	require.Contains(t, e.Error(), "503")
}
