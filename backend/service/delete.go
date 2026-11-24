package service

import (
	"networkai/backend"
	"networkai/constants"

	"github.com/olivere/elastic/v7"
)

// DeletePost deletes the post only when it belongs to user.
func DeletePost(id string, user string) error {
	query := elastic.NewBoolQuery()
	query.Must(elastic.NewTermQuery("id", id))
	query.Must(elastic.NewTermQuery("user", user))

	return backend.ESBackend.DeleteFromES(query, constants.POST_INDEX)
}
