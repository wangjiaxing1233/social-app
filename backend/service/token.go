package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"networkai/backend"
	"networkai/constants"

	jwt "github.com/form3tech-oss/jwt-go"
	"github.com/pborman/uuid"
)

const (
	// AccessTokenTTL is short so a stolen access token is only useful briefly.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL is how long a user stays signed in without re-entering a password.
	RefreshTokenTTL = 7 * 24 * time.Hour
)

var ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")

var signingKey []byte

// SetSigningKey sets the HMAC key used to sign access tokens.
func SetSigningKey(key []byte) {
	signingKey = key
}

// TokenPair is returned by /auth/token and /auth/refresh.
type TokenPair struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
}

// tokenRecord is stored in the token index. Refresh tokens are stored under
// the SHA-256 of the token, never the token itself. Revoked access tokens are
// stored under their jti until they would have expired anyway.
type tokenRecord struct {
	Username  string `json:"username"`
	Kind      string `json:"kind"` // "refresh" or "revoked_access"
	Revoked   bool   `json:"revoked"`
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
}

func refreshDocID(refreshToken string) string {
	sum := sha256.Sum256([]byte(refreshToken))
	return "refresh:" + hex.EncodeToString(sum[:])
}

func revokedAccessDocID(jti string) string {
	return "access:" + jti
}

// NewAccessToken signs a JWT for username. Every token gets a unique id (jti)
// so that it can be revoked before it expires.
func NewAccessToken(username string, ttl time.Duration) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"jti":      uuid.New(),
		"iat":      now.Unix(),
		"exp":      now.Add(ttl).Unix(),
	})
	return token.SignedString(signingKey)
}

// IssueTokenPair creates a short-lived access token and a long-lived,
// random refresh token for username.
func IssueTokenPair(username string) (*TokenPair, error) {
	accessToken, err := NewAccessToken(username, AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	refreshToken := base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now()
	record := tokenRecord{
		Username:  username,
		Kind:      "refresh",
		Revoked:   false,
		ExpiresAt: now.Add(RefreshTokenTTL).Unix(),
		CreatedAt: now.Unix(),
	}
	if err := backend.ESBackend.SaveToES(record, constants.TOKEN_INDEX, refreshDocID(refreshToken)); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:      accessToken,
		TokenType:        "Bearer",
		ExpiresIn:        int64(AccessTokenTTL.Seconds()),
		RefreshToken:     refreshToken,
		RefreshExpiresIn: int64(RefreshTokenTTL.Seconds()),
	}, nil
}

// RefreshTokens exchanges a refresh token for a new token pair. Refresh
// tokens are single use (rotation): the old one is revoked. If a refresh
// token that was already used shows up again, it was probably stolen, so
// every refresh token of that user is revoked (reuse detection).
func RefreshTokens(refreshToken string) (*TokenPair, error) {
	if refreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}
	id := refreshDocID(refreshToken)

	var record tokenRecord
	found, seqNo, primaryTerm, err := backend.ESBackend.GetFromES(constants.TOKEN_INDEX, id, &record)
	if err != nil {
		return nil, err
	}
	if !found || record.Kind != "refresh" {
		return nil, ErrInvalidRefreshToken
	}
	if record.Revoked {
		fmt.Printf("Refresh token reuse detected for %s; revoking all of their refresh tokens\n", record.Username)
		if err := backend.ESBackend.RevokeAllRefreshTokens(record.Username); err != nil {
			return nil, err
		}
		return nil, ErrInvalidRefreshToken
	}
	if time.Now().Unix() >= record.ExpiresAt {
		return nil, ErrInvalidRefreshToken
	}

	// Only one request may use this refresh token, even if two arrive at once.
	changed, err := backend.ESBackend.UpdateInESIfUnchanged(constants.TOKEN_INDEX, id, seqNo, primaryTerm,
		map[string]interface{}{"revoked": true})
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, ErrInvalidRefreshToken
	}

	return IssueTokenPair(record.Username)
}

// Logout revokes the access token identified by jti (until it would have
// expired) and, if given, one refresh token of the same user. With all=true
// it revokes every refresh token of the user, signing them out everywhere.
func Logout(username, jti string, accessExpiresAt int64, refreshToken string, all bool) error {
	now := time.Now().Unix()

	if jti != "" {
		record := tokenRecord{
			Username:  username,
			Kind:      "revoked_access",
			Revoked:   true,
			ExpiresAt: accessExpiresAt,
			CreatedAt: now,
		}
		if err := backend.ESBackend.SaveToES(record, constants.TOKEN_INDEX, revokedAccessDocID(jti)); err != nil {
			return err
		}
	}

	if all {
		return backend.ESBackend.RevokeAllRefreshTokens(username)
	}

	if refreshToken != "" {
		id := refreshDocID(refreshToken)
		var record tokenRecord
		found, _, _, err := backend.ESBackend.GetFromES(constants.TOKEN_INDEX, id, &record)
		if err != nil {
			return err
		}
		// A user may only revoke their own refresh token.
		if found && record.Kind == "refresh" && record.Username == username {
			return backend.ESBackend.UpdateInES(constants.TOKEN_INDEX, id, map[string]interface{}{"revoked": true})
		}
	}
	return nil
}

// IsAccessTokenRevoked reports whether the access token with this jti was
// revoked by a logout. Tokens without a jti (issued before this feature)
// cannot be revoked and are treated as valid until they expire.
func IsAccessTokenRevoked(jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	found, _, _, err := backend.ESBackend.GetFromES(constants.TOKEN_INDEX, revokedAccessDocID(jti), nil)
	return found, err
}
