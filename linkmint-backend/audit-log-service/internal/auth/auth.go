// Package auth verifies identity-service RS256 JWTs on the read endpoints and checks the caller
// holds an accepted reader role (admin/compliance). Verification uses only the standard library
// (crypto/rsa) — no third-party JWT dependency.
//
// It is config-gated: with no public key configured the Verifier is disabled and RequireReader is a
// pass-through (the Kong gateway is the authority — the existing Go-service convention, ADR-009).
// In-service verification is defense-in-depth for the system-of-record, not the primary control.
package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/paylink/audit-log-service/internal/httpx"
)

// Verifier holds the pinned RS256 public key and authorization policy.
type Verifier struct {
	pub      *rsa.PublicKey
	issuer   string
	audience string
	roles    map[string]struct{}
	now      func() time.Time
}

// New builds a Verifier. An empty pemStr returns a disabled verifier (no error). roles is the
// allowlist of accepted reader roles; an empty allowlist accepts any validly-signed, unexpired token.
func New(pemStr, issuer, audience string, roles []string) (*Verifier, error) {
	v := &Verifier{issuer: issuer, audience: audience, roles: map[string]struct{}{}, now: time.Now}
	for _, r := range roles {
		if r = strings.ToLower(strings.TrimSpace(r)); r != "" {
			v.roles[r] = struct{}{}
		}
	}
	if strings.TrimSpace(pemStr) == "" {
		return v, nil // disabled — gateway-trust
	}
	pub, err := parseRSAPublicKey(pemStr)
	if err != nil {
		return nil, err
	}
	v.pub = pub
	return v, nil
}

// Enabled reports whether in-service verification is active.
func (v *Verifier) Enabled() bool { return v.pub != nil }

// RequireReader is chi middleware. Disabled → pass-through. Enabled → require a valid RS256 bearer
// token carrying an accepted reader role; else 401/403.
func (v *Verifier) RequireReader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !v.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		tok := bearerToken(r)
		if tok == "" {
			httpx.WriteError(w, r, httpx.NewError(httpx.CodeUnauthorized, "missing bearer token", nil))
			return
		}
		c, err := v.verify(tok)
		if err != nil {
			httpx.WriteError(w, r, httpx.NewError(httpx.CodeUnauthorized, err.Error(), nil))
			return
		}
		if !v.hasReaderRole(c) {
			httpx.WriteError(w, r, httpx.NewError(httpx.CodeForbidden, "caller lacks an audit reader role (admin/compliance)", nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type claims struct {
	Sub       string          `json:"sub"`
	Iss       string          `json:"iss"`
	Aud       json.RawMessage `json:"aud"`
	Exp       int64           `json:"exp"`
	Nbf       int64           `json:"nbf"`
	UserRoles []string        `json:"user_roles"`
	Roles     json.RawMessage `json:"roles"`
}

// verify checks the RS256 signature and the exp/iss/aud claims, returning the parsed claims.
func (v *Verifier) verify(token string) (claims, error) {
	var c claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, errors.New("malformed token")
	}
	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, errors.New("invalid token header")
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil {
		return c, errors.New("invalid token header")
	}
	if hdr.Alg != "RS256" { // alg-confusion guard: never accept "none" or HS*
		return c, errors.New("unsupported token alg (RS256 only)")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, errors.New("invalid token payload")
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return c, errors.New("invalid token claims")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return c, errors.New("invalid token signature encoding")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(v.pub, crypto.SHA256, digest[:], sig); err != nil {
		return c, errors.New("invalid token signature")
	}
	now := v.now().Unix()
	if c.Exp == 0 || c.Exp < now {
		return c, errors.New("token expired")
	}
	if c.Nbf != 0 && c.Nbf > now {
		return c, errors.New("token not yet valid")
	}
	if v.issuer != "" && c.Iss != v.issuer {
		return c, errors.New("invalid token issuer")
	}
	if v.audience != "" && !audienceContains(c.Aud, v.audience) {
		return c, errors.New("invalid token audience")
	}
	return c, nil
}

func (v *Verifier) hasReaderRole(c claims) bool {
	if len(v.roles) == 0 {
		return true
	}
	for _, r := range collectRoles(c) {
		if _, ok := v.roles[strings.ToLower(r)]; ok {
			return true
		}
	}
	return false
}

// collectRoles gathers role strings from user_roles plus the org-membership roles array, which may
// be objects with a "role" field or bare strings (best effort — unknown shapes are ignored).
func collectRoles(c claims) []string {
	out := append([]string{}, c.UserRoles...)
	if len(c.Roles) > 0 {
		var objs []struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(c.Roles, &objs); err == nil {
			for _, o := range objs {
				if o.Role != "" {
					out = append(out, o.Role)
				}
			}
		} else {
			var strs []string
			if err := json.Unmarshal(c.Roles, &strs); err == nil {
				out = append(out, strs...)
			}
		}
	}
	return out
}

func audienceContains(raw json.RawMessage, want string) bool {
	if len(raw) == 0 {
		return false
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return one == want
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		for _, a := range many {
			if a == want {
				return true
			}
		}
	}
	return false
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("auth: invalid PEM public key")
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("auth: public key is not RSA")
		}
		return rsaPub, nil
	}
	if rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return rsaPub, nil
	}
	return nil, errors.New("auth: unsupported public key format")
}
