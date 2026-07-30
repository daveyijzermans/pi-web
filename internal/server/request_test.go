package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONBodyRejectsExplicitNonJSONContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	var body map[string]any

	if decodeJSONBody(rec, req, &body) {
		t.Fatal("decodeJSONBody accepted text/plain")
	}
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestDecodeJSONBodyRejectsOversizedBody(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPost,
		"/",
		strings.NewReader(`{"value":"`+strings.Repeat("x", maxJSONRequestBytes)+`"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	var body map[string]any

	if decodeJSONBody(rec, req, &body) {
		t.Fatal("decodeJSONBody accepted an oversized body")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestDecodeJSONBodyRejectsTrailingJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}{"extra":true}`))
	rec := httptest.NewRecorder()
	var body map[string]any

	if decodeJSONBody(rec, req, &body) {
		t.Fatal("decodeJSONBody accepted multiple JSON values")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDecodeJSONBodyAllowsMissingContentTypeForCLICompatibility(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}`))
	rec := httptest.NewRecorder()
	var body map[string]bool

	if !decodeJSONBody(rec, req, &body) {
		t.Fatalf("decodeJSONBody failed: status %d, body %s", rec.Code, rec.Body.String())
	}
	if !body["ok"] {
		t.Fatalf("decoded body = %#v", body)
	}
}
