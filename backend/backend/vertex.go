package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"networkai/util"

	"golang.org/x/oauth2/google"
)

// VertexBackend is nil when Vertex AI is not configured; semantic search is
// then turned off and posts are saved without embeddings.
var VertexBackend *VertexAIBackend

type VertexAIBackend struct {
	client     *http.Client
	endpoint   string
	dimensions int
}

// InitVertexBackend authenticates with the service account the server runs
// as (Application Default Credentials), so no API key is needed.
func InitVertexBackend(config *util.VertexInfo) {
	if config == nil || config.Project == "" {
		fmt.Println("Vertex AI is not configured; semantic search is turned off.")
		return
	}
	client, err := google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		panic(err)
	}
	client.Timeout = 20 * time.Second

	VertexBackend = &VertexAIBackend{
		client: client,
		endpoint: fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
			config.Location, config.Project, config.Location, config.Model),
		dimensions: config.Dimensions,
	}
	fmt.Printf("Vertex AI embeddings: model %s in %s, %d dimensions\n", config.Model, config.Location, config.Dimensions)
}

// Dimensions is the length of every embedding, or 0 when Vertex AI is off.
func (b *VertexAIBackend) Dimensions() int {
	if b == nil {
		return 0
	}
	return b.dimensions
}

// Embed turns text into a vector. Use taskType "RETRIEVAL_DOCUMENT" for text
// that is stored and "RETRIEVAL_QUERY" for search queries; the model encodes
// them slightly differently so that queries match the right documents.
func (b *VertexAIBackend) Embed(text, taskType string) ([]float32, error) {
	request := map[string]interface{}{
		"instances":  []map[string]string{{"content": text, "task_type": taskType}},
		"parameters": map[string]interface{}{"outputDimensionality": b.dimensions},
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := b.client.Post(b.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if len(respBody) > 500 {
			respBody = respBody[:500]
		}
		return nil, fmt.Errorf("vertex ai returned %s: %s", resp.Status, respBody)
	}

	var parsed struct {
		Predictions []struct {
			Embeddings struct {
				Values []float32 `json:"values"`
			} `json:"embeddings"`
		} `json:"predictions"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Predictions) == 0 {
		return nil, fmt.Errorf("vertex ai returned no embedding")
	}
	values := parsed.Predictions[0].Embeddings.Values
	if len(values) != b.dimensions {
		return nil, fmt.Errorf("vertex ai returned %d dimensions, expected %d", len(values), b.dimensions)
	}
	return values, nil
}
