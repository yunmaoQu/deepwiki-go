```
deepwiki-go/
├── cmd/
│   └── server/
│       └── main.go       # Entry point for the server
├── internal/
│   ├── api/              # API handlers
│   │   ├── router.go     # Router setup
│   │   ├── handlers.go   # Request handlers
│   │   └── middleware.go # Middleware functions
│   ├── config/           # Configuration
│   │   └── config.go     # Environment variables and config
│   ├── rag/              # Retrieval Augmented Generation
│   │   ├── rag.go        # RAG implementation
│   │   └── memory.go     # Conversation memory
│   └── data/             # Data processing
│       ├── repository.go # Repository cloning and management
│       ├── embedding.go  # Document embedding
│       └── storage.go    # Local storage management
├── pkg/
│   ├── models/           # Data models
│   │   └── models.go     # Shared data structures
│   └── utils/            # Utility functions
│       ├── tokens.go     # Token counting
│       └── files.go      # File handling
├── go.mod
└── go.sum
```
