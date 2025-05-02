# DeepWiki-Go

DeepWiki-Go is an automatic code knowledge base generation tool implemented in Go. It can analyze any GitHub or GitLab code repository and automatically generate a structured, interactive Wiki document.

## 🔍 Core Features

- **Code Repository Analysis**: Automatically analyze repository structure and code dependencies
- **Wiki Auto-generation**: Generate module introductions, architecture documentation, and API documentation
- **Visualization Charts**: Automatically create architecture diagrams and flowcharts to explain code relationships
- **RAG Search**: Intelligent code search based on Retrieval-Augmented Generation
- **Private Repository Support**: Support for accessing private repositories via access tokens
- **Multi-language Support**: Analysis supports over 20 mainstream programming languages

## 🛠️ Technical Architecture

```
deepwiki-go/
├── cmd/                  # Program entry point
│   └── main.go           # Main program entry
├── internal/             # Internal application code
│   ├── api/              # API server
│   │   ├── handlers.go   # API handlers
│   │   ├── middleware.go # Middleware
│   │   └── routes.go     # Route configuration
│   ├── config/           # Configuration management
│   │   └── config.go     # Configuration structure and initialization
│   ├── data/             # Data processing
│   │   ├── database.go   # Database operations
│   ├── models/           # Data models
│   │   └── models.go     # Model definitions
│   └── rag/              # Retrieval Augmented Generation
│       ├── memory.go     # Memory cache
│       └── rag.go        # RAG implementation
└── pkg/                  # Public utilities
    └── utils/            # Utility functions
        ├── git.go        # Git operations
        └── token.go      # Token handling
```

## 🚀 Quick Start

### Prerequisites

- Go 1.18+
- Git
- Google API key (for AI generation)
- OpenAI API key (for text embeddings)

### Environment Setup

1. Clone the repository
```bash
git clone https://github.com/yourusername/deepwiki-go.git
cd deepwiki-go
```

2. Create a `.env` file
```
GOOGLE_API_KEY=your_google_api_key
OPENAI_API_KEY=your_openai_api_key
PORT=8001  # Optional, default is 8001
```

### Build and Run

1. Build the application
```bash
go build -o deepwiki ./cmd/
```

2. Run the application
```bash
./deepwiki
```

The application will start the API server at http://localhost:8001.

### Docker Deployment

1. Build Docker image
```bash
docker build -t deepwiki-go .
```

2. Run container
```bash
docker run -d -p 8001:8001 --env-file .env --name deepwiki deepwiki-go
```

## 📝 API Usage

### Generate Wiki

```bash
curl -X POST http://localhost:8001/api/v1/wiki/generate \
  -H "Content-Type: application/json" \
  -d '{"repo_url": "https://github.com/username/repo", "github_token": "your_token"}'
```

### Search Documents

```bash
curl -X POST http://localhost:8001/api/v1/vectors/search \
  -H "Content-Type: application/json" \
  -d '{"query": "how to implement user authentication", "repo_url": "https://github.com/username/repo"}'
```

## 🤝 Contributing

Issues and Pull Requests are welcome! Please check out the [Contributing Guide](CONTRIBUTING.md) for more information.

## 📄 License

This project is licensed under the [MIT License](LICENSE).

