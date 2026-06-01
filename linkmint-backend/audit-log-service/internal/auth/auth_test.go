package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func genKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func sign(t *testing.T, key *rsa.PrivateKey, alg string, claims map[string]any) string {
	t.Helper()
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"` + alg + `","typ":"JWT"}`))
	pb, _ := json.Marshal(claims)
	pl := base64.RawURLEncoding.EncodeToString(pb)
	si := hdr + "." + pl
	digest := sha256.Sum256([]byte(si))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	return si + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestDisabledPassesThrough(t *testing.T) {
	v, err := New("", "", "", nil)
	if err != nil || v.Enabled() {
		t.Fatalf("empty pem should disable: enabled=%v err=%v", v.Enabled(), err)
	}
	called := false
	rec := httptest.NewRecorder()
	v.RequireReader(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/v1/audit-log", nil))
	if !called {
		t.Fatal("disabled verifier must pass through")
	}
}

func TestValidTokenWithUserRole(t *testing.T) {
	key, pemStr := genKey(t)
	v, err := New(pemStr, "linkmint-identity", "linkmint", []string{"admin", "compliance"})
	if err != nil {
		t.Fatal(err)
	}
	tok := sign(t, key, "RS256", map[string]any{
		"sub": "u1", "iss": "linkmint-identity", "aud": "linkmint",
		"exp": time.Now().Add(time.Hour).Unix(), "user_roles": []string{"admin"},
	})
	req := httptest.NewRequest("GET", "/v1/audit-log", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	ok := false
	v.RequireReader(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ok = true })).ServeHTTP(rec, req)
	if !ok || rec.Code != http.StatusOK {
		t.Fatalf("valid admin token should pass, code=%d", rec.Code)
	}
}

func TestRolesFromOrgMemberships(t *testing.T) {
	key, pemStr := genKey(t)
	v, _ := New(pemStr, "", "", []string{"compliance"})
	tok := sign(t, key, "RS256", map[string]any{
		"exp":   time.Now().Add(time.Hour).Unix(),
		"roles": []map[string]any{{"role": "compliance", "org_type": "platform"}},
	})
	c, err := v.verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if !v.hasReaderRole(c) {
		t.Fatal("compliance org-role should grant reader access")
	}
}

func TestEmptyAllowlistAcceptsAnyValidToken(t *testing.T) {
	key, pemStr := genKey(t)
	v, _ := New(pemStr, "", "", nil) // no role requirement
	tok := sign(t, key, "RS256", map[string]any{"exp": time.Now().Add(time.Hour).Unix(), "user_roles": []string{"payer"}})
	c, err := v.verify(tok)
	if err != nil || !v.hasReaderRole(c) {
		t.Fatalf("empty allowlist should accept any valid token: err=%v", err)
	}
}

func TestRejectsMissingRole(t *testing.T) {
	key, pemStr := genKey(t)
	v, _ := New(pemStr, "", "", []string{"admin"})
	tok := sign(t, key, "RS256", map[string]any{"exp": time.Now().Add(time.Hour).Unix(), "user_roles": []string{"payer"}})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	v.RequireReader(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing role should be 403, got %d", rec.Code)
	}
}

func TestRejectsAlgConfusion(t *testing.T) {
	key, pemStr := genKey(t)
	v, _ := New(pemStr, "", "", nil)
	tok := sign(t, key, "none", map[string]any{"exp": time.Now().Add(time.Hour).Unix()})
	if _, err := v.verify(tok); err == nil {
		t.Fatal("alg != RS256 must be rejected")
	}
}

func TestRejectsExpiredAndMissingExp(t *testing.T) {
	key, pemStr := genKey(t)
	v, _ := New(pemStr, "", "", nil)
	if _, err := v.verify(sign(t, key, "RS256", map[string]any{"exp": time.Now().Add(-time.Hour).Unix()})); err == nil {
		t.Fatal("expired token must be rejected")
	}
	if _, err := v.verify(sign(t, key, "RS256", map[string]any{"sub": "u"})); err == nil {
		t.Fatal("token without exp must be rejected")
	}
}

func TestRejectsNotYetValid(t *testing.T) {
	key, pemStr := genKey(t)
	v, _ := New(pemStr, "", "", nil)
	tok := sign(t, key, "RS256", map[string]any{
		"exp": time.Now().Add(time.Hour).Unix(),
		"nbf": time.Now().Add(time.Hour).Unix(), // not valid until an hour from now
	})
	if _, err := v.verify(tok); err == nil {
		t.Fatal("a not-yet-valid (nbf in the future) token must be rejected")
	}
}

func TestRejectsBadSignature(t *testing.T) {
	key1, _ := genKey(t)
	_, pem2 := genKey(t)
	v, _ := New(pem2, "", "", nil) // pinned to a different key
	tok := sign(t, key1, "RS256", map[string]any{"exp": time.Now().Add(time.Hour).Unix()})
	if _, err := v.verify(tok); err == nil {
		t.Fatal("signature from a different key must be rejected")
	}
}

func TestRejectsIssuerAndAudience(t *testing.T) {
	key, pemStr := genKey(t)
	v, _ := New(pemStr, "want-iss", "want-aud", nil)
	if _, err := v.verify(sign(t, key, "RS256", map[string]any{"exp": time.Now().Add(time.Hour).Unix(), "iss": "other", "aud": "want-aud"})); err == nil {
		t.Fatal("issuer mismatch must be rejected")
	}
	if _, err := v.verify(sign(t, key, "RS256", map[string]any{"exp": time.Now().Add(time.Hour).Unix(), "iss": "want-iss", "aud": "other"})); err == nil {
		t.Fatal("audience mismatch must be rejected")
	}
}

func TestMalformedTokens(t *testing.T) {
	key, pemStr := genKey(t)
	_ = key
	v, _ := New(pemStr, "", "", nil)
	for _, tok := range []string{"", "a.b", "a.b.c.d", "!!.??.$$"} {
		if _, err := v.verify(tok); err == nil {
			t.Fatalf("malformed token %q must be rejected", tok)
		}
	}
}

func TestMissingBearerIs401(t *testing.T) {
	_, pemStr := genKey(t)
	v, _ := New(pemStr, "", "", nil)
	rec := httptest.NewRecorder()
	v.RequireReader(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token should be 401, got %d", rec.Code)
	}
}

func TestInvalidPEM(t *testing.T) {
	if _, err := New("not a pem", "", "", nil); err == nil {
		t.Fatal("invalid pem must error")
	}
}

func TestAudienceArray(t *testing.T) {
	key, pemStr := genKey(t)
	v, _ := New(pemStr, "", "linkmint", nil)
	tok := sign(t, key, "RS256", map[string]any{"exp": time.Now().Add(time.Hour).Unix(), "aud": []string{"other", "linkmint"}})
	if _, err := v.verify(tok); err != nil {
		t.Fatalf("audience present in array should pass: %v", err)
	}
}
