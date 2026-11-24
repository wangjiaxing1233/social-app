package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"networkai/service"
)

// semanticSearchHandler handles GET /posts/semantic-search?q=...&limit=10.
// It returns posts ordered by how close their message is in meaning to q,
// each with a "score" field.
func semanticSearchHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one semantic search request")

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "Query parameter q is required", http.StatusBadRequest)
		return
	}
	if len(query) > 500 {
		http.Error(w, "Query is too long (limit 500 characters)", http.StatusBadRequest)
		return
	}

	limit := 10
	if value := r.URL.Query().Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 50 {
			http.Error(w, "limit must be a number from 1 to 50", http.StatusBadRequest)
			return
		}
		limit = n
	}

	posts, err := service.SemanticSearch(query, limit)
	if errors.Is(err, service.ErrSemanticSearchDisabled) {
		http.Error(w, "Semantic search is not configured on this server", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		http.Error(w, "Failed to run semantic search", http.StatusInternalServerError)
		fmt.Printf("Failed to run semantic search %v\n", err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}
