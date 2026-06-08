package httpx_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bosonicalcom/bedrock-go/syserr"
	"github.com/bosonicalcom/bedrock-go/transport/httpx"
)

func TestParseSystemError(t *testing.T) {
	errJSON, err := json.Marshal(httpx.ErrorEnvelope{
		Error: httpx.ErrorResponse{
			Code:       http.StatusNotFound,
			Message:    "some message",
			StatusCode: string(syserr.CodeNotFound),
			Details: []any{
				httpx.RequestInfoResponse{
					DetailResponse: httpx.DetailResponse{
						TypeUrl: syserr.RequestInfoType,
					},
					RequestID: "some_id",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(errJSON)),
	}

	systemErr, err := httpx.ParseSystemError(resp)
	if err != nil {
		t.Fatal(err)
	}
	if systemErr.Code != syserr.CodeNotFound {
		t.Errorf("expected error code %s, got %s", syserr.CodeNotFound, systemErr.Code)
	}
	if systemErr.Message != "some message" {
		t.Errorf("expected error message %s, got %s", "some message", systemErr.Message)
	}
	if len(systemErr.Details) != 1 {
		t.Errorf("expected 1 detail, got %d", len(systemErr.Details))
		return
	}
	if systemErr.Details[0].(syserr.RequestInfo).RequestID != "some_id" {
		t.Errorf("expected detail %s, got %s", "some_id", systemErr.Details[0].(syserr.RequestInfo).RequestID)
	}
}
