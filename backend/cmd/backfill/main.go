// Command backfill adds embeddings to posts that were saved before semantic
// search existed. Run it from the project root so it finds conf/deploy.yml:
//
//	go run ./cmd/backfill
package main

import (
	"fmt"

	"networkai/backend"
	"networkai/service"
	"networkai/util"
)

func main() {
	config, err := util.LoadApplicationConfig("conf", "deploy.yml")
	if err != nil {
		panic(err)
	}

	backend.InitElasticsearchBackend(config.ElasticsearchConfig)
	backend.InitVertexBackend(config.VertexConfig)
	if err := backend.EnsureExtraIndexes(backend.VertexBackend.Dimensions()); err != nil {
		panic(err)
	}

	updated, skipped, err := service.BackfillEmbeddings()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Backfill finished: %d posts updated, %d skipped.\n", updated, skipped)
}
