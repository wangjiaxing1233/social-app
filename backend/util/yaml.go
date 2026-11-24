package util

import (
    "io/ioutil"
    "path/filepath"

    "gopkg.in/yaml.v2"
)

type ElasticsearchInfo struct {
    Address  string `yaml:"address"`
    Username string `yaml:"username"`
    Password string `yaml:"password"`
}

type GCSInfo struct {
    Bucket string `yaml:"bucket"`
}

type TokenInfo struct {
    Secret string `yaml:"secret"`
}

// VertexInfo configures the Vertex AI embedding model used for semantic search.
type VertexInfo struct {
    Project    string `yaml:"project"`
    Location   string `yaml:"location"`
    Model      string `yaml:"model"`
    Dimensions int    `yaml:"dimensions"`
}

type ApplicationConfig struct {
    ElasticsearchConfig *ElasticsearchInfo `yaml:"elasticsearch"`
    GCSConfig           *GCSInfo           `yaml:"gcs"`
    TokenConfig         *TokenInfo         `yaml:"token"`
    VertexConfig        *VertexInfo        `yaml:"vertex"`
}

func LoadApplicationConfig(configDir, configFile string) (*ApplicationConfig, error) {
    content, err := ioutil.ReadFile(filepath.Join(configDir, configFile))
    if err != nil {
        return nil, err
    }

    var config ApplicationConfig
    err = yaml.Unmarshal(content, &config)
    if err != nil {
        return nil, err
    }
    return &config, nil
}
