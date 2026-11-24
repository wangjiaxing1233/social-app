package main

import (
	"fmt"
	"log"
	"net/http"

	"networkai/backend"
	"networkai/handler"
	"networkai/util"
)

func main() {
	fmt.Println("started-service")

	config, err := util.LoadApplicationConfig("conf", "deploy.yml")
	if err != nil {
		panic(err)
	}

	backend.InitElasticsearchBackend(config.ElasticsearchConfig)
	backend.InitGCSBackend(config.GCSConfig)
	backend.InitVertexBackend(config.VertexConfig)
	if err := backend.EnsureExtraIndexes(backend.VertexBackend.Dimensions()); err != nil {
		panic(err)
	}

	log.Fatal(http.ListenAndServe(":8080", handler.InitRouter(config.TokenConfig)))
}
