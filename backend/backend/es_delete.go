package backend

import (
	"context"

	"github.com/olivere/elastic/v7"
)

// DeleteFromES deletes every document in index that matches query.
func (backend *ElasticsearchBackend) DeleteFromES(query elastic.Query, index string) error {
	_, err := backend.client.DeleteByQuery().
		Index(index).
		Query(query).
		Refresh("true").
		Pretty(true).
		Do(context.Background())
	return err
}
