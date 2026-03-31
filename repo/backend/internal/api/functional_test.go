package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"trainingops/internal/config"
	"trainingops/internal/security"
)

func TestWorkspaceDashboards(t *testing.T) {
	server := newFunctionalDBServer(t)
	token := createTestToken(t, server)

	roles := []string{"admin", "coordinator", "instructor", "learner"}
	for _, role := range roles {
		req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+role+"/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Tenant-ID", "tenant-1")
		rec := httptest.NewRecorder()

		server.echo.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("role %s: expected 200, got %d", role, rec.Code)
		}

		payload := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("role %s: invalid json: %v", role, err)
		}
		if payload["role"] != role {
			t.Fatalf("role %s: expected payload role %q, got %#v", role, role, payload["role"])
		}
		if payload["title"] == "" {
			t.Fatalf("role %s: expected title", role)
		}
	}
}

func TestCORSPreflight(t *testing.T) {
	server := newFunctionalDBServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/v1/bookings/hold", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected allow-origin header, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("expected allow-methods header")
	}
}

func createTestToken(t *testing.T, server *Server) string {
	t.Helper()
	registerBody := map[string]string{
		"tenant_id":    "tenant-1",
		"email":        "qa@example.com",
		"display_name": "QA User",
		"role":         "admin",
		"password":     "Password123!A",
	}
	registerJSON, _ := json.Marshal(registerBody)
	registerReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerJSON))
	registerRec := httptest.NewRecorder()
	server.echo.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d: %s", registerRec.Code, registerRec.Body.String())
	}

	loginBody := map[string]string{
		"tenant_id": "tenant-1",
		"email":     "qa@example.com",
		"password":  "Password123!A",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(loginJSON))
	loginRec := httptest.NewRecorder()
	server.echo.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}

	payload := map[string]any{}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid login payload: %v", err)
	}
	token, _ := payload["access_token"].(string)
	if token == "" {
		t.Fatal("missing access_token in login response")
	}
	return token
}

func TestDashboardRoleRestriction(t *testing.T) {
	server := newFunctionalDBServer(t)

	registerBody := map[string]string{
		"tenant_id":    "tenant-1",
		"email":        "learner@example.com",
		"display_name": "Learner User",
		"role":         "learner",
		"password":     "Password123!A",
	}
	registerJSON, _ := json.Marshal(registerBody)
	registerReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerJSON))
	registerRec := httptest.NewRecorder()
	server.echo.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d", registerRec.Code)
	}

	loginBody := map[string]string{
		"tenant_id": "tenant-1",
		"email":     "learner@example.com",
		"password":  "Password123!A",
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(loginJSON))
	loginRec := httptest.NewRecorder()
	server.echo.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d", loginRec.Code)
	}

	payload := map[string]any{}
	_ = json.Unmarshal(loginRec.Body.Bytes(), &payload)
	token, _ := payload["access_token"].(string)
	if token == "" {
		t.Fatal("missing access_token")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for learner->admin dashboard, got %d", rec.Code)
	}
}

func newFunctionalDBServer(t *testing.T) *Server {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping DB-backed API test")
	}
	return NewServer(config.Config{
		AppEnv:        "test",
		JWTSigningKey: "test-signing-key",
		DatabaseURL:   databaseURL,
		CORSOrigins:   []string{"http://localhost:3000"},
		StoragePath:   "./uploads",
		ReportsPath:   "./reports",
	}, &security.Vault{})
}
