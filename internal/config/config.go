package config

import (
	"os"
	"log"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port     string `yaml:"port"`
	JWTSecret string `yaml:"jwt_secret,omitempty"` // Optional JWT secret
}

// GoogleConfig holds Google Cloud related configuration
type GoogleConfig struct {
	APIKey         string `yaml:"api_key"`
	ProjectID      string `yaml:"project_id"`
	Location       string `yaml:"location"`
	EmbeddingModel string `yaml:"embedding_model"`
}

// RetrieverConfig holds retriever configuration
type RetrieverConfig struct {
	Type string `yaml:"type"`
	TopK int    `yaml:"top_k"`
}

// DBConfig holds database configuration
type DBConfig struct {
	Type             string `yaml:"type"`
	Path             string `yaml:"path,omitempty"` // Used for file-based DBs like JSON, SQLite
	ConnectionString string `yaml:"connection_string,omitempty"` // Used for server-based DBs like Postgres
	
	// Milvus specific settings
	MilvusAddress      string `yaml:"milvus_address,omitempty"` // Milvus server address, default: localhost:19530
	MilvusCollection   string `yaml:"milvus_collection,omitempty"` // Milvus collection name, default: deepwiki_documents
	EmbeddingDimension int    `yaml:"embedding_dimension,omitempty"` // Dimension of embedding vectors, default: 768
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// TextSplitterConfig holds text splitter configuration
type TextSplitterConfig struct {
	SplitBy      string `yaml:"split_by"`
	ChunkSize    int    `yaml:"chunk_size"`
	ChunkOverlap int    `yaml:"chunk_overlap"`
}

// FileFiltersConfig holds file filters configuration
type FileFiltersConfig struct {
	ExcludedDirs  []string `yaml:"excluded_dirs"`
	ExcludedFiles []string `yaml:"excluded_files"`
}

// Config holds the overall application configuration
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Google       GoogleConfig       `yaml:"google"`
	Retriever    RetrieverConfig    `yaml:"retriever"`
	DB           DBConfig           `yaml:"db"`
	Logging      LoggingConfig      `yaml:"logging"`
	TextSplitter TextSplitterConfig `yaml:"text_splitter"`
	FileFilters  FileFiltersConfig  `yaml:"file_filters"`
	OpenAIAPIKey string             `yaml:"openai_api_key"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	// Default path if not provided
	if configPath == "" {
		configPath = "internal/config/config.yaml" // Or determine dynamically
	}

	log.Printf("Loading configuration from: %s", configPath)

	// Read the YAML file
	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Error reading config file %s: %v", configPath, err)
		return nil, err
	}

	// Parse the YAML file
	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Printf("Error parsing config file %s: %v", configPath, err)
		return nil, err
	}

	// Optional: Override with environment variables (example for server port)
	if portEnv := os.Getenv("SERVER_PORT"); portEnv != "" {
		config.Server.Port = portEnv
	}
	if apiKeyEnv := os.Getenv("GOOGLE_API_KEY"); apiKeyEnv != "" {
		config.Google.APIKey = apiKeyEnv
	}
	if openAIAPIKeyEnv := os.Getenv("OPENAI_API_KEY"); openAIAPIKeyEnv != "" {
		config.OpenAIAPIKey = openAIAPIKeyEnv
	}
    // Add more environment variable overrides as needed

	log.Println("Configuration loaded successfully")
	return config, nil
}