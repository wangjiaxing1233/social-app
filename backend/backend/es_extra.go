package backend

import (
	"context"
	"encoding/json"
	"fmt"

	"networkai/constants"

	"github.com/olivere/elastic/v7"
)

// EnsureExtraIndexes creates the token index used for refresh tokens and
// revoked access tokens, and adds the "embedding" vector field to the post
// index when semantic search is enabled (embeddingDims > 0).
func EnsureExtraIndexes(embeddingDims int) error {
	ctx := context.Background()
	client := ESBackend.client

	exists, err := client.IndexExists(constants.TOKEN_INDEX).Do(ctx)
	if err != nil {
		return err
	}
	if !exists {
		mapping := `{
			"mappings": {
				"properties": {
					"username":   {"type": "keyword"},
					"kind":       {"type": "keyword"},
					"revoked":    {"type": "boolean"},
					"expires_at": {"type": "date", "format": "epoch_second"},
					"created_at": {"type": "date", "format": "epoch_second"}
				}
			}
		}`
		if _, err := client.CreateIndex(constants.TOKEN_INDEX).Body(mapping).Do(ctx); err != nil {
			return fmt.Errorf("create token index: %w", err)
		}
	}

	if embeddingDims > 0 {
		body := fmt.Sprintf(`{"properties": {"embedding": {"type": "dense_vector", "dims": %d}}}`, embeddingDims)
		if _, err := client.PutMapping().Index(constants.POST_INDEX).BodyString(body).Do(ctx); err != nil {
			return fmt.Errorf("add embedding field to post index: %w", err)
		}
	}
	return nil
}

// GetFromES reads one document by id. It is real-time, so a document is
// visible right after it is written. found is false when the document does
// not exist. seqNo and primaryTerm are used for optimistic concurrency.
func (backend *ElasticsearchBackend) GetFromES(index, id string, out interface{}) (found bool, seqNo, primaryTerm int64, err error) {
	result, err := backend.client.Get().Index(index).Id(id).Do(context.Background())
	if elastic.IsNotFound(err) {
		return false, 0, 0, nil
	}
	if err != nil {
		return false, 0, 0, err
	}
	if !result.Found {
		return false, 0, 0, nil
	}
	if out != nil && result.Source != nil {
		if err := json.Unmarshal(result.Source, out); err != nil {
			return true, 0, 0, err
		}
	}
	if result.SeqNo != nil {
		seqNo = *result.SeqNo
	}
	if result.PrimaryTerm != nil {
		primaryTerm = *result.PrimaryTerm
	}
	return true, seqNo, primaryTerm, nil
}

// UpdateInES merges fields into an existing document.
func (backend *ElasticsearchBackend) UpdateInES(index, id string, fields map[string]interface{}) error {
	_, err := backend.client.Update().Index(index).Id(id).Doc(fields).Do(context.Background())
	return err
}

// UpdateInESIfUnchanged merges fields into a document only if nobody changed
// it since it was read with GetFromES. It returns changed=false when another
// request updated the document first.
func (backend *ElasticsearchBackend) UpdateInESIfUnchanged(index, id string, seqNo, primaryTerm int64, fields map[string]interface{}) (changed bool, err error) {
	_, err = backend.client.Update().Index(index).Id(id).
		IfSeqNo(seqNo).IfPrimaryTerm(primaryTerm).
		Doc(fields).Do(context.Background())
	if elastic.IsConflict(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// RevokeAllRefreshTokens marks every refresh token of a user as revoked.
func (backend *ElasticsearchBackend) RevokeAllRefreshTokens(username string) error {
	query := elastic.NewBoolQuery().
		Must(elastic.NewTermQuery("username", username)).
		Must(elastic.NewTermQuery("kind", "refresh"))
	_, err := backend.client.UpdateByQuery(constants.TOKEN_INDEX).
		Query(query).
		Script(elastic.NewScript("ctx._source.revoked = true")).
		ProceedOnVersionConflict().
		Refresh("true").
		Do(context.Background())
	return err
}

// SearchES runs a query and returns up to size hits, leaving out the listed
// source fields (for example the large "embedding" vector).
func (backend *ElasticsearchBackend) SearchES(index string, query elastic.Query, size int, excludeFields ...string) (*elastic.SearchResult, error) {
	search := backend.client.Search().Index(index).Query(query).Size(size)
	if len(excludeFields) > 0 {
		search = search.FetchSourceContext(elastic.NewFetchSourceContext(true).Exclude(excludeFields...))
	}
	return search.Do(context.Background())
}

// SemanticSearchES ranks documents by cosine similarity between their vector
// field and queryVector. Scores are cosine similarity + 1 (so 0..2), because
// Elasticsearch does not allow negative scores.
func (backend *ElasticsearchBackend) SemanticSearchES(index, field string, queryVector []float32, size int) (*elastic.SearchResult, error) {
	script := elastic.NewScript(fmt.Sprintf("cosineSimilarity(params.query_vector, '%s') + 1.0", field)).
		Param("query_vector", queryVector)
	query := elastic.NewScriptScoreQuery(elastic.NewExistsQuery(field), script)
	return backend.SearchES(index, query, size, field)
}
