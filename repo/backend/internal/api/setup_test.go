package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetupStatusEndpoint(t *testing.T) {
	server := newDBBackedTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/setup/status", nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Fatalf("expected setup status 200 or 404, got %d", rec.Code)
	}
}

func TestSetupTenantSecondCallBlocked(t *testing.T) {
	server := newDBBackedTestServer(t)
	payload := map[string]any{
		"tenant_name":    "Bootstrap Tenant",
		"tenant_slug":    "bootstrap-tenant",
		"admin_username": "Bootstrap Admin",
		"admin_email":    "bootstrap-admin@example.com",
		"admin_password": "Password123!A",
	}
	body, _ := json.Marshal(payload)
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/setup/tenant", bytes.NewReader(body))
	firstReq.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()
	server.echo.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated && firstRec.Code != http.StatusConflict {
		t.Fatalf("expected first setup call to return 201 or 409, got %d", firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/setup/tenant", bytes.NewReader(body))
	secondReq.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()
	server.echo.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected second setup call to return 409, got %d", secondRec.Code)
	}
}
