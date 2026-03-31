package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"trainingops/internal/config"
	"trainingops/internal/security"
)

func TestSessionLifecycle_LogoutRevokesBearerToken(t *testing.T) {
	server := newDBBackedTestServer(t)
	token, _ := registerAndLogin(t, server, "admin-session@example.com", "admin")

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 before logout, got %d", rec.Code)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutRec := httptest.NewRecorder()
	server.echo.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", logoutRec.Code)
	}

	afterReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/admin/dashboard", nil)
	afterReq.Header.Set("Authorization", "Bearer "+token)
	afterRec := httptest.NewRecorder()
	server.echo.ServeHTTP(afterRec, afterReq)
	if afterRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", afterRec.Code)
	}
}

func TestSessionRotation_OldCookieRejected_NewCookieAccepted(t *testing.T) {
	server := newDBBackedTestServer(t)
	_, cookie := registerAndLogin(t, server, "admin-rotate@example.com", "admin")
	if cookie == nil {
		t.Fatal("expected login session cookie")
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/admin/dashboard", nil)
	firstReq.AddCookie(cookie)
	firstRec := httptest.NewRecorder()
	server.echo.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first cookie call 200, got %d", firstRec.Code)
	}
	rotated := firstRec.Result().Cookies()
	if len(rotated) == 0 {
		t.Fatal("expected rotated cookie")
	}
	newCookie := rotated[0]

	oldReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/admin/dashboard", nil)
	oldReq.AddCookie(cookie)
	oldRec := httptest.NewRecorder()
	server.echo.ServeHTTP(oldRec, oldReq)
	if oldRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old cookie to be rejected, got %d", oldRec.Code)
	}

	newReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/admin/dashboard", nil)
	newReq.AddCookie(newCookie)
	newRec := httptest.NewRecorder()
	server.echo.ServeHTTP(newRec, newReq)
	if newRec.Code != http.StatusOK {
		t.Fatalf("expected new cookie to be accepted, got %d", newRec.Code)
	}
}

func TestSessionExpiry_ExpiredTokenRejected(t *testing.T) {
	server := newDBBackedTestServer(t)
	token, _ := security.SignSessionToken("test-signing-key", security.SessionClaims{
		UserID:    "u1",
		TenantID:  "tenant-1",
		Email:     "x@example.com",
		Role:      "admin",
		SessionID: "sess-expired",
		ExpiresAt: time.Now().UTC().Add(-time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rec.Code)
	}
}

func TestRBACMatrix(t *testing.T) {
	server := newDBBackedTestServer(t)
	adminToken, _ := registerAndLogin(t, server, "admin-rbac@example.com", "admin")
	coordToken, _ := registerAndLogin(t, server, "coord-rbac@example.com", "coordinator")
	instToken, _ := registerAndLogin(t, server, "inst-rbac@example.com", "instructor")
	learnerToken, _ := registerAndLogin(t, server, "learner-rbac@example.com", "learner")

	cases := []struct {
		name   string
		method string
		path   string
		token  string
		body   map[string]any
		want   int
	}{
		{name: "admin reports", method: http.MethodGet, path: "/v1/reports/bookings.csv", token: adminToken, want: http.StatusOK},
		{name: "admin permissions", method: http.MethodGet, path: "/v1/admin/permissions", token: adminToken, want: http.StatusOK},
		{name: "coordinator hold", method: http.MethodPost, path: "/v1/bookings/hold", token: coordToken, body: validHoldPayload(), want: http.StatusCreated},
		{name: "coordinator admin route denied", method: http.MethodGet, path: "/v1/reports/bookings.csv", token: coordToken, want: http.StatusForbidden},
		{name: "coordinator admin permissions denied", method: http.MethodGet, path: "/v1/admin/permissions", token: coordToken, want: http.StatusForbidden},
		{name: "instructor uploads", method: http.MethodPost, path: "/v1/uploads/start", token: instToken, body: map[string]any{"document_id": "11111111-1111-4111-8111-111111111111", "file_name": "notes.txt", "expected_chunks": 1}, want: http.StatusCreated},
		{name: "instructor hold denied", method: http.MethodPost, path: "/v1/bookings/hold", token: instToken, body: validHoldPayload(), want: http.StatusForbidden},
		{name: "instructor learner catalog denied", method: http.MethodGet, path: "/v1/learner/catalog", token: instToken, want: http.StatusForbidden},
		{name: "learner uploads denied", method: http.MethodPost, path: "/v1/uploads/start", token: learnerToken, body: map[string]any{"document_id": "22222222-2222-4222-8222-222222222222", "file_name": "notes.txt", "expected_chunks": 1}, want: http.StatusForbidden},
		{name: "learner tasks denied", method: http.MethodGet, path: "/v1/milestones/m1/tasks", token: learnerToken, want: http.StatusForbidden},
		{name: "learner catalog", method: http.MethodGet, path: "/v1/learner/catalog", token: learnerToken, want: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body *bytes.Reader
			if tc.body != nil {
				payload, _ := json.Marshal(tc.body)
				body = bytes.NewReader(payload)
			} else {
				body = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			server.echo.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
		})
	}

	noAuthReq := httptest.NewRequest(http.MethodGet, "/v1/reports/bookings.csv", nil)
	noAuthRec := httptest.NewRecorder()
	server.echo.ServeHTTP(noAuthRec, noAuthReq)
	if noAuthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated 401, got %d", noAuthRec.Code)
	}
}

func TestObjectAuthorization_CrossUserAccessDenied(t *testing.T) {
	server := newDBBackedTestServer(t)
	coord1, _ := registerAndLogin(t, server, "coord1-owner@example.com", "coordinator")
	coord2, _ := registerAndLogin(t, server, "coord2-owner@example.com", "coordinator")

	holdReq := httptest.NewRequest(http.MethodPost, "/v1/bookings/hold", mustJSON(validHoldPayload()))
	holdReq.Header.Set("Content-Type", "application/json")
	holdReq.Header.Set("Authorization", "Bearer "+coord1)
	holdRec := httptest.NewRecorder()
	server.echo.ServeHTTP(holdRec, holdReq)
	if holdRec.Code != http.StatusCreated {
		t.Fatalf("expected hold create 201, got %d: %s", holdRec.Code, holdRec.Body.String())
	}
	var holdPayload map[string]any
	_ = json.Unmarshal(holdRec.Body.Bytes(), &holdPayload)
	bookingMap, _ := holdPayload["booking"].(map[string]any)
	bookingID, _ := bookingMap["ID"].(string)
	if bookingID == "" {
		bookingID, _ = bookingMap["id"].(string)
	}
	if bookingID == "" {
		t.Fatal("expected booking id in hold response")
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/bookings/cancel", mustJSON(map[string]any{"booking_id": bookingID}))
	cancelReq.Header.Set("Content-Type", "application/json")
	cancelReq.Header.Set("Authorization", "Bearer "+coord2)
	cancelRec := httptest.NewRecorder()
	server.echo.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusForbidden {
		t.Fatalf("expected booking ownership 403, got %d", cancelRec.Code)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/uploads/start", mustJSON(map[string]any{
		"document_id":     "33333333-3333-4333-8333-333333333333",
		"file_name":       "manual.txt",
		"expected_chunks": 1,
	}))
	startReq.Header.Set("Content-Type", "application/json")
	startReq.Header.Set("Authorization", "Bearer "+coord1)
	startRec := httptest.NewRecorder()
	server.echo.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusCreated {
		t.Fatalf("expected upload start 201, got %d", startRec.Code)
	}
	var startPayload map[string]any
	_ = json.Unmarshal(startRec.Body.Bytes(), &startPayload)
	sessionID, _ := startPayload["ID"].(string)
	if sessionID == "" {
		sessionID, _ = startPayload["id"].(string)
	}
	if sessionID == "" {
		t.Fatal("expected upload session id")
	}

	appendReq := httptest.NewRequest(http.MethodPost, "/v1/uploads/chunk", mustJSON(map[string]any{
		"session_id": sessionID,
		"index":      0,
		"chunk_b64":  "YQ==",
		"checksum":   "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
	}))
	appendReq.Header.Set("Content-Type", "application/json")
	appendReq.Header.Set("Authorization", "Bearer "+coord2)
	appendRec := httptest.NewRecorder()
	server.echo.ServeHTTP(appendRec, appendReq)
	if appendRec.Code != http.StatusForbidden {
		t.Fatalf("expected content ownership 403, got %d", appendRec.Code)
	}
}

func TestLearnerReservationFlow(t *testing.T) {
	server := newDBBackedTestServer(t)
	coordToken, _ := registerAndLogin(t, server, "coord-reserve@example.com", "coordinator")
	learnerToken, _ := registerAndLogin(t, server, "learner-reserve@example.com", "learner")

	holdReq := httptest.NewRequest(http.MethodPost, "/v1/bookings/hold", mustJSON(validHoldPayload()))
	holdReq.Header.Set("Content-Type", "application/json")
	holdReq.Header.Set("Authorization", "Bearer "+coordToken)
	holdRec := httptest.NewRecorder()
	server.echo.ServeHTTP(holdRec, holdReq)
	if holdRec.Code != http.StatusCreated {
		t.Fatalf("expected hold create 201, got %d: %s", holdRec.Code, holdRec.Body.String())
	}
	var holdPayload map[string]any
	_ = json.Unmarshal(holdRec.Body.Bytes(), &holdPayload)
	bookingMap, _ := holdPayload["booking"].(map[string]any)
	bookingID, _ := bookingMap["ID"].(string)
	if bookingID == "" {
		bookingID, _ = bookingMap["id"].(string)
	}

	confirmReq := httptest.NewRequest(http.MethodPost, "/v1/bookings/confirm", mustJSON(map[string]any{"booking_id": bookingID}))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmReq.Header.Set("Authorization", "Bearer "+coordToken)
	confirmRec := httptest.NewRecorder()
	server.echo.ServeHTTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusOK {
		t.Fatalf("expected confirm 200, got %d: %s", confirmRec.Code, confirmRec.Body.String())
	}

	reserveReq := httptest.NewRequest(http.MethodPost, "/v1/learner/reserve", mustJSON(map[string]any{"booking_id": bookingID}))
	reserveReq.Header.Set("Content-Type", "application/json")
	reserveReq.Header.Set("Authorization", "Bearer "+learnerToken)
	reserveRec := httptest.NewRecorder()
	server.echo.ServeHTTP(reserveRec, reserveReq)
	if reserveRec.Code != http.StatusCreated {
		t.Fatalf("expected reserve 201, got %d: %s", reserveRec.Code, reserveRec.Body.String())
	}

	myReq := httptest.NewRequest(http.MethodGet, "/v1/learner/my-reservations", nil)
	myReq.Header.Set("Authorization", "Bearer "+learnerToken)
	myRec := httptest.NewRecorder()
	server.echo.ServeHTTP(myRec, myReq)
	if myRec.Code != http.StatusOK {
		t.Fatalf("expected my-reservations 200, got %d: %s", myRec.Code, myRec.Body.String())
	}
}

func registerAndLogin(t *testing.T, server *Server, email, role string) (string, *http.Cookie) {
	t.Helper()
	registerBody := map[string]string{
		"tenant_id":    "tenant-1",
		"email":        email,
		"display_name": role + " user",
		"role":         role,
		"password":     "Password123!A",
	}
	regReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", mustJSON(registerBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	server.echo.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}

	loginBody := map[string]string{
		"tenant_id": "tenant-1",
		"email":     email,
		"password":  "Password123!A",
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", mustJSON(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	server.echo.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(loginRec.Body.Bytes(), &payload)
	token, _ := payload["access_token"].(string)
	var cookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "trainingops_session" {
			cc := c
			cookie = cc
			break
		}
	}
	return token, cookie
}

func validHoldPayload() map[string]any {
	now := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Minute)
	return map[string]any{
		"room_id":       "room-a",
		"instructor_id": "inst-1",
		"title":         "Role matrix session",
		"start_at":      now.Format(time.RFC3339),
		"end_at":        now.Add(45 * time.Minute).Format(time.RFC3339),
		"capacity":      12,
		"attendees":     10,
	}
}

func mustJSON(v any) *bytes.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

func newDBBackedTestServer(t *testing.T) *Server {
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
