package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"networkai/backend"
	"networkai/constants"
	"networkai/model"

	"github.com/olivere/elastic/v7"
)

var ErrSemanticSearchDisabled = errors.New("semantic search is not configured")

// postDocument is what is stored in the post index: the post plus the vector
// of its message. Search responses use model.Post, which has no vector.
type postDocument struct {
	model.Post
	Embedding []float32 `json:"embedding,omitempty"`
}

// ScoredPost is one semantic search result. Score is the cosine similarity
// between the query and the post message: closer to 1 means more similar.
type ScoredPost struct {
	model.Post
	Score float64 `json:"score"`
}

// savePostDocument saves a post, with an embedding of its message when
// Vertex AI is configured. Embedding is best effort: if Vertex AI fails, the
// post is still saved, and the backfill command can add the vector later.
func savePostDocument(post *model.Post) error {
	doc := postDocument{Post: *post}
	vector, err := embedDocument(post.Message)
	if err != nil {
		fmt.Printf("Failed to embed post %s, saving it without an embedding: %v\n", post.Id, err)
	} else {
		doc.Embedding = vector
	}
	return backend.ESBackend.SaveToES(doc, constants.POST_INDEX, post.Id)
}

func embedDocument(text string) ([]float32, error) {
	if backend.VertexBackend == nil || strings.TrimSpace(text) == "" {
		return nil, nil
	}
	return backend.VertexBackend.Embed(text, "RETRIEVAL_DOCUMENT")
}

// SemanticSearch finds the posts whose messages mean something close to the
// query, even when they share no words with it.
func SemanticSearch(query string, limit int) ([]ScoredPost, error) {
	if backend.VertexBackend == nil {
		return nil, ErrSemanticSearchDisabled
	}
	vector, err := backend.VertexBackend.Embed(query, "RETRIEVAL_QUERY")
	if err != nil {
		return nil, err
	}

	result, err := backend.ESBackend.SemanticSearchES(constants.POST_INDEX, "embedding", vector, limit)
	if err != nil {
		return nil, err
	}

	posts := []ScoredPost{}
	for _, hit := range result.Hits.Hits {
		var post model.Post
		if err := json.Unmarshal(hit.Source, &post); err != nil {
			return nil, err
		}
		score := 0.0
		if hit.Score != nil {
			score = *hit.Score - 1.0 // undo the +1 that keeps Elasticsearch scores positive
		}
		posts = append(posts, ScoredPost{Post: post, Score: score})
	}
	return posts, nil
}

// BackfillEmbeddings adds embeddings to posts saved before semantic search
// existed. It returns how many posts were updated and how many were skipped
// (empty message or embedding error).
func BackfillEmbeddings() (updated, skipped int, err error) {
	if backend.VertexBackend == nil {
		return 0, 0, ErrSemanticSearchDisabled
	}
	query := elastic.NewBoolQuery().MustNot(elastic.NewExistsQuery("embedding"))
	result, err := backend.ESBackend.SearchES(constants.POST_INDEX, query, 1000)
	if err != nil {
		return 0, 0, err
	}

	for _, hit := range result.Hits.Hits {
		var post model.Post
		if err := json.Unmarshal(hit.Source, &post); err != nil {
			return updated, skipped, err
		}
		vector, err := embedDocument(post.Message)
		if err != nil || vector == nil {
			if err != nil {
				fmt.Printf("Skipping post %s: %v\n", hit.Id, err)
			}
			skipped++
			continue
		}
		if err := backend.ESBackend.UpdateInES(constants.POST_INDEX, hit.Id, map[string]interface{}{"embedding": vector}); err != nil {
			return updated, skipped, err
		}
		updated++
	}
	return updated, skipped, nil
}
