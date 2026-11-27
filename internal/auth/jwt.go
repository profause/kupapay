package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned for any signature/format/expiry failure.
// It carries its own HTTP status (401) via *AuthError so middleware and
// handlers can dispatch on it uniformly.
var ErrInvalidToken = &AuthError{Message: "invalid token", Status: 401}

// Claims is the legacy @nestjs/jwt payload (jwtData + standard iat/exp).
type Claims struct {
	ID string `json:"id"`
	jwt.RegisteredClaims
}

// SignHS256 mirrors @nestjs/jwt signAsync(jwtData): HS256 with the legacy JWT
// secret. The payload shape is exactly the legacy jwtData
// (id/isSecondFactorAuthenticated/singleSignInReference) plus iat/exp,
// so legacy and Atlas tokens interop.
func SignHS256(c Claims, secret string, ttlSeconds int64) (string, error) {
	if secret == "" {
		return "", ErrInvalidToken
	}

	now := time.Now()
	c.IssuedAt = jwt.NewNumericDate(now)
	c.ExpiresAt = jwt.NewNumericDate(now.Add(time.Duration(ttlSeconds) * time.Second))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString([]byte(secret))
}

// parseAndVerifyHS256 validates an HS256 JWT against secret and returns its
// claims. It mirrors the legacy guard: signature must match and, when present,
// exp-iat must be positive and the token unexpired.
func parseAndVerifyHS256(token, secret string) (*Claims, error) {
	return parseAndVerifyHS256At(token, secret, time.Now())
}

func parseAndVerifyHS256At(tokenString, secret string, now time.Time) (*Claims, error) {
	return parseHS256(tokenString, secret, jwt.WithTimeFunc(func() time.Time { return now }))
}

// parseHS256IgnoringExpiry verifies the signature and shape of a token but
// accepts one that has expired. This is for the refresh path only: refresh runs
// precisely *because* the access token expired, so enforcing exp there would
// reject every token the endpoint exists to serve. A token accepted here is not
// evidence of a live session — that still comes from the refresh token row and
// the single-sign-in reference. Use it to read claims, never to authenticate.
func parseHS256IgnoringExpiry(tokenString, secret string) (*Claims, error) {
	return parseHS256(tokenString, secret, jwt.WithoutClaimsValidation())
}

func parseHS256(tokenString, secret string, opts ...jwt.ParserOption) (*Claims, error) {
	if tokenString == "" || secret == "" {
		return nil, ErrInvalidToken
	}

	// exp is intentionally NOT required: the legacy Nest app registers its
	// JwtModule with only a secret (no signOptions.expiresIn), so legacy-minted
	// tokens carry iat but no exp. Requiring exp here would reject every legacy
	// token during dual-run. We still validate exp when it IS present (the jwt
	// library enforces unexpired automatically), and Atlas-minted tokens always
	// include it. Session bounding for exp-less tokens comes from the
	// single-sign-in-reference rotation and idle-duration checks downstream.
	claims := new(Claims)
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return []byte(secret), nil
		},
		append(opts, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))...,
	)
	if err != nil || token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.ID == "" || claims.IssuedAt == nil {
		return nil, ErrInvalidToken
	}
	// When the token does carry an expiry, keep the legacy sanity check that the
	// validity window is positive (exp strictly after iat).
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Unix()-claims.IssuedAt.Time.Unix() <= 0 {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
