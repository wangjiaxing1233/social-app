package handler

import (
	"net/http"

	"networkai/service"
	"networkai/util"

	jwtMiddleware "github.com/auth0/go-jwt-middleware"
	jwt "github.com/form3tech-oss/jwt-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var mySigningKey []byte

func InitRouter(config *util.TokenInfo) http.Handler {
	mySigningKey = []byte(config.Secret)
	service.SetSigningKey(mySigningKey)

	jwtMiddleware := jwtMiddleware.New(jwtMiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(mySigningKey), nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})

	// protected requires a valid access token that has not been revoked.
	protected := func(h http.Handler) http.Handler {
		return jwtMiddleware.Handler(checkRevoked(h))
	}

	router := mux.NewRouter()

	// Posts
	router.Handle("/posts", protected(rateLimit(uploadLimiter, byUser, http.HandlerFunc(uploadHandler)))).Methods("POST")
	router.Handle("/posts/search", protected(http.HandlerFunc(searchHandler))).Methods("GET")
	router.Handle("/posts/semantic-search", protected(http.HandlerFunc(semanticSearchHandler))).Methods("GET")
	router.Handle("/posts/upload-url", protected(rateLimit(uploadLimiter, byUser, http.HandlerFunc(uploadURLHandler)))).Methods("POST")
	router.Handle("/posts/complete", protected(http.HandlerFunc(completeUploadHandler))).Methods("POST")
	router.Handle("/posts/{id}", protected(http.HandlerFunc(deleteHandler))).Methods("DELETE")

	// Accounts
	router.Handle("/users", rateLimit(authLimiter, byIP, http.HandlerFunc(signupHandler))).Methods("POST")
	router.Handle("/auth/login", rateLimit(authLimiter, byIP, http.HandlerFunc(signinHandler))).Methods("POST")
	router.Handle("/auth/token", rateLimit(authLimiter, byIP, http.HandlerFunc(tokenHandler))).Methods("POST")
	router.Handle("/auth/refresh", rateLimit(authLimiter, byIP, http.HandlerFunc(refreshHandler))).Methods("POST")
	router.Handle("/auth/logout", protected(http.HandlerFunc(logoutHandler))).Methods("POST")

	originsOk := handlers.AllowedOrigins([]string{"*"})
	headersOk := handlers.AllowedHeaders([]string{"Authorization", "Content-Type"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "POST", "DELETE"})

	return handlers.CORS(originsOk, headersOk, methodsOk)(rateLimit(globalLimiter, byIP, router))
}
