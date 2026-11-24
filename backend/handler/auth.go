package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"networkai/model"
	"networkai/service"

	jwt "github.com/form3tech-oss/jwt-go"
)

const maxJSONBodyBytes = 1 << 20

// tokenHandler signs a user in and returns an access token plus a refresh
// token as JSON. /auth/login still returns only a 24-hour access token as
// plain text, for clients built against the original API.
func tokenHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one token request")

	var user model.User
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)).Decode(&user); err != nil {
		http.Error(w, "Cannot decode user data from client", http.StatusBadRequest)
		return
	}

	success, err := service.CheckUser(user.Username, user.Password)
	if err != nil {
		http.Error(w, "Failed to read user from Elasticsearch", http.StatusInternalServerError)
		fmt.Printf("Failed to read user from Elasticsearch %v\n", err)
		return
	}
	if !success {
		http.Error(w, "User doesn't exists or wrong password", http.StatusUnauthorized)
		return
	}

	pair, err := service.IssueTokenPair(user.Username)
	if err != nil {
		http.Error(w, "Failed to generate tokens", http.StatusInternalServerError)
		fmt.Printf("Failed to generate tokens %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

// refreshHandler exchanges a refresh token for a new token pair.
func refreshHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one refresh request")

	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)).Decode(&body); err != nil {
		http.Error(w, "Cannot decode refresh token from client", http.StatusBadRequest)
		return
	}

	pair, err := service.RefreshTokens(body.RefreshToken)
	if errors.Is(err, service.ErrInvalidRefreshToken) {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "Failed to refresh tokens", http.StatusInternalServerError)
		fmt.Printf("Failed to refresh tokens %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

// logoutHandler revokes the access token used for this request. The JSON
// body is optional: {"refresh_token": "..."} also revokes that refresh
// token, and {"all": true} revokes all of the user's refresh tokens.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one logout request")

	claims := tokenClaims(r)
	username, _ := claims["username"].(string)
	jti, _ := claims["jti"].(string)
	var expiresAt int64
	if exp, ok := claims["exp"].(float64); ok {
		expiresAt = int64(exp)
	}

	var body struct {
		RefreshToken string `json:"refresh_token"`
		All          bool   `json:"all"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, "Cannot decode logout request from client", http.StatusBadRequest)
		return
	}

	if err := service.Logout(username, jti, expiresAt, body.RefreshToken, body.All); err != nil {
		http.Error(w, "Failed to log out", http.StatusInternalServerError)
		fmt.Printf("Failed to log out %v\n", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// checkRevoked rejects access tokens that were revoked by a logout. It runs
// after the JWT middleware has validated the token.
func checkRevoked(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jti, _ := tokenClaims(r)["jti"].(string)
		revoked, err := service.IsAccessTokenRevoked(jti)
		if err != nil {
			http.Error(w, "Failed to check token", http.StatusInternalServerError)
			fmt.Printf("Failed to check token %v\n", err)
			return
		}
		if revoked {
			http.Error(w, "Token has been revoked", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tokenClaims returns the claims of the JWT that the middleware validated,
// or nil when the request has none.
func tokenClaims(r *http.Request) jwt.MapClaims {
	token, ok := r.Context().Value("user").(*jwt.Token)
	if !ok {
		return nil
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	return claims
}

func usernameFromRequest(r *http.Request) string {
	username, _ := tokenClaims(r)["username"].(string)
	return username
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Printf("Failed to write JSON response %v\n", err)
	}
}
