package http

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cihub/seelog"
	"github.com/smartping/smartping/src/g"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once
)

const (
	tokenCookieName = "smartping_token"
	defaultTokenTTL = 7 * 24 * time.Hour
)

// getJWTSecret retrieves or initializes a persistent 32-byte secret key for signing tokens.
func getJWTSecret() []byte {
	jwtSecretOnce.Do(func() {
		secretPath := g.Root + "/conf/.jwt_secret"
		if data, err := os.ReadFile(secretPath); err == nil && len(data) >= 32 {
			jwtSecret = data
			return
		}

		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			seelog.Warn("[Auth] rand.Read failed, falling back to timestamp-based secret")
			h := sha256.Sum256([]byte(fmt.Sprintf("%d-%s", time.Now().UnixNano(), g.Cfg.Password)))
			key = h[:]
		}
		_ = os.WriteFile(secretPath, key, 0600)
		jwtSecret = key
	})
	return jwtSecret
}

// HashPassword hashes a plain password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// VerifyPassword verifies whether the supplied password matches the stored password.
// Supports both bcrypt hashes and backward-compatible plaintext comparisons.
func VerifyPassword(stored, password string) bool {
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	}
	// Fallback to legacy plaintext comparison
	return stored == password
}

type JWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type JWTClaims struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

// GenerateToken creates an HMAC-SHA256 signed JWT token.
func GenerateToken(sub string, ttl time.Duration) (string, error) {
	header := JWTHeader{Alg: "HS256", Typ: "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	now := time.Now().Unix()
	claims := JWTClaims{
		Sub: sub,
		Iat: now,
		Exp: now + int64(ttl.Seconds()),
	}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	mac := hmac.New(sha256.New, getJWTSecret())
	mac.Write([]byte(signingInput))
	signatureB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signatureB64, nil
}

// ValidateToken verifies the signature and expiration of a JWT token.
func ValidateToken(tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, getJWTSecret())
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, expectedSig) {
		return nil, errors.New("signature mismatch")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid claims encoding")
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, errors.New("invalid claims json")
	}

	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// ExtractToken extracts authentication token from Header, Cookie or Query.
func ExtractToken(r *http.Request) string {
	// 1. Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	// 2. Cookie
	if cookie, err := r.Cookie(tokenCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	// 3. Query or Form token parameter
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}

// IsAuthenticated checks whether the current request possesses a valid token.
func IsAuthenticated(r *http.Request) bool {
	token := ExtractToken(r)
	if token == "" {
		return false
	}
	_, err := ValidateToken(token)
	return err == nil
}

// RequireAuth middleware protects sensitive API endpoints.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":         false,
				"error":          "Unauthorized: valid token required",
				"login_required": true,
			})
			return
		}
		next(w, r)
	}
}
