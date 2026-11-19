package http

import (
	"bytes"
	"encoding/json"
	"github.com/smartping/smartping/src/g"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestPasswordHashingAndVerification(t *testing.T) {
	plain := "mySecretPass123!"

	// 1. Hash with bcrypt
	hashed, err := HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hashed == plain {
		t.Fatal("hashed password must not equal plain password")
	}

	// 2. Verify with valid password
	if !VerifyPassword(hashed, plain) {
		t.Fatal("VerifyPassword failed for matching password")
	}

	// 3. Verify with wrong password
	if VerifyPassword(hashed, "wrongPassword") {
		t.Fatal("VerifyPassword succeeded for wrong password")
	}

	// 4. Backward-compatibility: legacy plaintext comparison
	legacyPlain := "legacyPassword"
	if !VerifyPassword(legacyPlain, legacyPlain) {
		t.Fatal("VerifyPassword failed for legacy plaintext match")
	}
	if VerifyPassword(legacyPlain, "notLegacy") {
		t.Fatal("VerifyPassword succeeded for mismatched plaintext")
	}
}

func TestJWTGenerationAndValidation(t *testing.T) {
	subject := "admin"
	ttl := 1 * time.Hour

	token, err := GenerateToken(subject, ttl)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.Sub != subject {
		t.Errorf("expected subject %s, got %s", subject, claims.Sub)
	}

	// Tampered token test
	tampered := token[:len(token)-5] + "XXXXX"
	if _, err := ValidateToken(tampered); err == nil {
		t.Fatal("ValidateToken should fail on tampered signature")
	}

	// Expired token test
	expiredToken, err := GenerateToken(subject, -1*time.Minute)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if _, err := ValidateToken(expiredToken); err == nil {
		t.Fatal("ValidateToken should fail on expired token")
	}
}

func TestTokenExtraction(t *testing.T) {
	testToken := "header.payload.signature"

	// 1. Bearer Header
	req1 := httptest.NewRequest("GET", "/api/config.json", nil)
	req1.Header.Set("Authorization", "Bearer "+testToken)
	if got := ExtractToken(req1); got != testToken {
		t.Errorf("ExtractToken from header = %s, expected %s", got, testToken)
	}

	// 2. Cookie
	req2 := httptest.NewRequest("GET", "/api/config.json", nil)
	req2.AddCookie(&http.Cookie{Name: tokenCookieName, Value: testToken})
	if got := ExtractToken(req2); got != testToken {
		t.Errorf("ExtractToken from cookie = %s, expected %s", got, testToken)
	}

	// 3. Query Param
	req3 := httptest.NewRequest("GET", "/api/config.json?token="+testToken, nil)
	if got := ExtractToken(req3); got != testToken {
		t.Errorf("ExtractToken from query = %s, expected %s", got, testToken)
	}
}

func TestLoginAndAuthFlow(t *testing.T) {
	// Initialize test configuration
	g.CfgLock.Lock()
	g.Cfg.Password = "adminPass888"
	g.CfgLock.Unlock()

	server := InitHttpServer()
	mux := server.Handler

	// 1. Login with invalid password
	loginBody, _ := json.Marshal(map[string]string{"password": "wrong"})
	req := httptest.NewRequest("POST", "/api/login.json", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong password, got %d", w.Code)
	}

	// 2. Login with correct password
	loginBody, _ = json.Marshal(map[string]string{"password": "adminPass888"})
	req = httptest.NewRequest("POST", "/api/login.json", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on login success, got %d", w.Code)
	}

	var res struct {
		Status bool   `json:"status"`
		Token  string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if !res.Status || res.Token == "" {
		t.Fatalf("expected valid token response, got %s", w.Body.String())
	}

	// Check that Set-Cookie was sent
	cookies := w.Result().Cookies()
	var foundCookie bool
	for _, c := range cookies {
		if c.Name == tokenCookieName && c.Value == res.Token {
			foundCookie = true
			if !c.HttpOnly {
				t.Error("auth cookie must be HttpOnly")
			}
		}
	}
	if !foundCookie {
		t.Error("smartping_token cookie was not set")
	}

	// 3. Check auth status with token
	req = httptest.NewRequest("GET", "/api/auth/status.json", nil)
	req.Header.Set("Authorization", "Bearer "+res.Token)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var statusRes struct {
		Authenticated bool   `json:"authenticated"`
		Username      string `json:"username"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &statusRes)
	if !statusRes.Authenticated || statusRes.Username != "admin" {
		t.Fatalf("auth status should be true, got %s", w.Body.String())
	}

	// 4. Logout
	req = httptest.NewRequest("POST", "/api/logout.json", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", w.Code)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	server := InitHttpServer()
	handler := server.Handler

	// GET request to verify security headers
	req := httptest.NewRequest("GET", "/api/auth/status.json", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if nosniff := w.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %s", nosniff)
	}
	if frame := w.Header().Get("X-Frame-Options"); frame != "SAMEORIGIN" {
		t.Errorf("expected X-Frame-Options: SAMEORIGIN, got %s", frame)
	}

	// OPTIONS preflight request
	optReq := httptest.NewRequest("OPTIONS", "/api/login.json", nil)
	optW := httptest.NewRecorder()
	handler.ServeHTTP(optW, optReq)

	if optW.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", optW.Code)
	}
	if allowOrigin := optW.Header().Get("Access-Control-Allow-Origin"); allowOrigin == "" {
		t.Error("expected Access-Control-Allow-Origin header on OPTIONS preflight")
	}
}

func TestSaveConfigWithPasswordUpgrade(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(tmpDir+"/conf", 0755)
	oldRoot := g.Root
	g.Root = tmpDir
	defer func() { g.Root = oldRoot }()

	g.CfgLock.Lock()
	g.Cfg.Password = "plain123"
	g.Cfg.Name = "TestNode"
	g.Cfg.Addr = "127.0.0.1"
	g.Cfg.Base = map[string]int{"Timeout": 5, "Archive": 7, "Refresh": 1}
	g.Cfg.Topology = map[string]string{"Tline": "1", "Tsymbolsize": "70"}
	g.CfgLock.Unlock()

	server := InitHttpServer()
	handler := server.Handler

	// Form with matching password
	configJson := `{"Name":"TestNode","Addr":"127.0.0.1","Password":"newSecretPass456","Base":{"Timeout":5,"Archive":7,"Refresh":1},"Topology":{"Tline":"1","Tsymbolsize":"70"}}`
	form := url.Values{
		"password": {"plain123"},
		"config":   {configJson},
	}

	req := httptest.NewRequest("POST", "/api/saveconfig.json", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("saveconfig returned code %d, body: %s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["status"] != "true" {
		t.Fatalf("saveconfig failed: %v", res)
	}

	// Password in g.Cfg should now be upgraded to bcrypt hash!
	g.CfgLock.RLock()
	newPass := g.Cfg.Password
	g.CfgLock.RUnlock()

	if !VerifyPassword(newPass, "newSecretPass456") {
		t.Errorf("new password verification failed against %s", newPass)
	}
}
