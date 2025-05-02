// internal/data/database.go
package data

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/deepwiki-go/internal/models"
	"github.com/deepwiki-go/pkg/utils"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	collectionName     = "deepwiki_documents"
	embeddingDimension = 768               // Example dimension, replace with your model's dimension
	milvusAddress      = "localhost:19530" // Default Milvus address
)

// DatabaseManager 管理文档数据库
type DatabaseManager struct {
	milvusClient  client.Client
	repoURLOrPath string
	repoPaths     map[string]string
	mu            sync.RWMutex // To protect access to internal state if needed
	initialized   bool
}

// NewDatabaseManager 创建一个新的数据库管理器
func NewDatabaseManager() (*DatabaseManager, error) {

	log.Printf("Connecting to Milvus at %s", milvusAddress)
	milvusClient, err := client.NewClient(context.Background(), client.Config{
		Address: milvusAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %w", err)
	}

	dm := &DatabaseManager{
		milvusClient: milvusClient,
		repoPaths:    make(map[string]string),
	}

	err = dm.ensureCollectionExists()
	if err != nil {
		// Close client if initialization fails
		dm.milvusClient.Close()
		return nil, fmt.Errorf("failed to ensure Milvus collection: %w", err)
	}

	log.Println("DatabaseManager initialized successfully with Milvus")
	return dm, nil
}

// ensureCollectionExists checks if the collection exists and creates it if not.
func (dm *DatabaseManager) ensureCollectionExists() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.initialized {
		return nil
	}

	ctx := context.Background()
	has, err := dm.milvusClient.HasCollection(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("failed to check if collection exists: %w", err)
	}

	if !has {
		log.Printf("Collection '%s' does not exist. Creating...", collectionName)
		// Define schema
		schema := &entity.Schema{
			CollectionName: collectionName,
			Description:    "DeepWiki document collection",
			AutoID:         false,
			Fields: []*entity.Field{
				{
					Name:       "doc_id",
					DataType:   entity.FieldTypeInt64,
					PrimaryKey: true,
					AutoID:     false,
				},
				{
					Name:     "file_path", // Store original file path
					DataType: entity.FieldTypeVarChar,
				},
				{
					Name:     "embedding",
					DataType: entity.FieldTypeFloatVector,
					TypeParams: map[string]string{
						"dim": fmt.Sprintf("%d", embeddingDimension),
					},
				},
				{
					Name:     "raw_text", // Store the raw text chunk
					DataType: entity.FieldTypeVarChar,
				},
				{
					Name:     "metadata_json", // Store metadata as JSON string
					DataType: entity.FieldTypeVarChar,
				},
			},
		}

		err = dm.milvusClient.CreateCollection(ctx, schema, entity.DefaultShardNumber)
		if err != nil {
			return fmt.Errorf("failed to create collection '%s': %w", collectionName, err)
		}
		log.Printf("Collection '%s' created successfully.", collectionName)

		// Create index for the embedding field after creating the collection
		log.Printf("Creating index for embedding field...")
		index, err := entity.NewIndexHNSW(entity.L2, 8, 200) // Example HNSW params
		if err != nil {
			return fmt.Errorf("failed to create HNSW index parameters: %w", err)
		}
		err = dm.milvusClient.CreateIndex(ctx, collectionName, "embedding", index, false)
		if err != nil {
			return fmt.Errorf("failed to create index on 'embedding': %w", err)
		}
		log.Printf("Index created successfully for embedding field.")
	} else {
		log.Printf("Collection '%s' already exists.", collectionName)
	}

	// Load collection into memory for searching
	log.Printf("Loading collection '%s' into memory...", collectionName)
	err = dm.milvusClient.LoadCollection(ctx, collectionName, false)
	if err != nil {
		return fmt.Errorf("failed to load collection '%s': %w", collectionName, err)
	}
	log.Printf("Collection '%s' loaded successfully.", collectionName)

	dm.initialized = true
	return nil
}

// Close cleans up the Milvus connection
func (dm *DatabaseManager) Close() {
	if dm.milvusClient != nil {
		dm.milvusClient.Close()
		log.Println("Milvus connection closed.")
	}
}

// generateDocID creates a unique Int64 ID from a string (e.g., file path)
func generateDocID(identifier string) int64 {
	hasher := sha256.New()
	hasher.Write([]byte(identifier))
	hash := hasher.Sum(nil)
	// Use the first 8 bytes of the hash to create an int64
	// Note: This can lead to collisions, although unlikely for typical repo sizes.
	// A more robust approach might involve a central ID registry or UUIDs if collisions are a concern.
	return int64(binary.BigEndian.Uint64(hash[:8]))
}

// PrepareDatabase prepares the Milvus collection for the given repository.
// It reads documents, generates embeddings, and inserts them into Milvus.
func (dm *DatabaseManager) PrepareDatabase(repoURLOrPath string, accessToken string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Assume repoURLOrPath is a valid local path to the documents.
	// TODO: Add validation to ensure repoURLOrPath is a directory.
	localRepoPath := repoURLOrPath // Treat the input as the local path directly.
	dm.repoPaths["save_repo_dir"] = localRepoPath

	// Check if this repo has already been indexed in Milvus
	// (We might need a way to track this, e.g., checking a few sample doc IDs)
	// For now, we'll re-index every time, which is inefficient.
	// A better approach would be incremental updates or checking existence.

	log.Printf("Starting document processing for %s", dm.repoPaths["save_repo_dir"])
	documents, err := dm.readAllDocuments(dm.repoPaths["save_repo_dir"])
	if err != nil {
		return fmt.Errorf("failed to read documents: %w", err)
	}

	log.Printf("Read %d documents. Generating embeddings and inserting into Milvus...", len(documents))
	addedCount := 0
	for _, doc := range documents {
		if err := dm.addDocumentInternal(&doc); err != nil {
			// Log error but continue processing other documents
			log.Printf("Error adding document '%s' to Milvus: %v", doc.MetaData["file_path"], err)
		} else {
			addedCount++
		}
	}

	// Ensure data is flushed
	err = dm.milvusClient.Flush(context.Background(), collectionName, false)
	if err != nil {
		log.Printf("Warning: failed to flush collection '%s': %v", collectionName, err)
		// Not returning error here, as inserts might still succeed later
	}

	log.Printf("Finished processing. Added %d documents to Milvus for %s", addedCount, repoURLOrPath)
	return nil
}

func (dm *DatabaseManager) createRepo(repoURLOrPath string, accessToken string) any {
	panic("unimplemented")
}

// addDocumentInternal adds a single document to Milvus (used internally by PrepareDatabase)
// Assumes lock is already held if called from PrepareDatabase
func (dm *DatabaseManager) addDocumentInternal(doc *models.Document) error {
	ctx := context.Background()

	// Generate embedding
	embedding, err := dm.getEmbedding(doc.Text)
	if err != nil {
		return fmt.Errorf("failed to get embedding for '%s': %w", doc.MetaData["file_path"], err)
	}

	// Generate ID
	filePath := doc.MetaData["file_path"].(string) // Assuming file_path exists and is string
	docID := generateDocID(filePath)

	// Prepare data for Milvus
	idCol := entity.NewColumnInt64("doc_id", []int64{docID})
	pathCol := entity.NewColumnVarChar("file_path", []string{filePath})
	embeddingCol := entity.NewColumnFloatVector("embedding", embeddingDimension, [][]float32{embedding})
	textCol := entity.NewColumnVarChar("raw_text", []string{doc.Text})

	metadataBytes, err := json.Marshal(doc.MetaData)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata for '%s': %w", filePath, err)
	}
	metadataCol := entity.NewColumnVarChar("metadata_json", []string{string(metadataBytes)})

	_, err = dm.milvusClient.Insert(ctx, collectionName, "", idCol, pathCol, embeddingCol, textCol, metadataCol)
	if err != nil {
		return fmt.Errorf("failed to insert document '%s' (ID: %d) into Milvus: %w", filePath, docID, err)
	}

	return nil
}

// fileExists checks if a file exists
func (dm *DatabaseManager) fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// readAllDocuments reads all documents from a directory
func (dm *DatabaseManager) readAllDocuments(path string) ([]models.Document, error) {
	var documents []models.Document

	// Define file extensions to look for
	codeExtensions := []string{".py", ".js", ".ts", ".java", ".cpp", ".c", ".go", ".rs",
		".jsx", ".tsx", ".html", ".css", ".php", ".swift", ".cs"}
	docExtensions := []string{".md", ".txt", ".rst", ".json", ".yaml", ".yml"}

	// Define excluded directories and files
	excludedDirs := []string{".venv", "node_modules", ".git", "__pycache__"}
	excludedFiles := []string{"package-lock.json", "yarn.lock"}

	log.Printf("Reading documents from %s", path)

	// Process code files
	for _, ext := range codeExtensions {
		files, err := utils.FindFiles(path, ext)
		if err != nil {
			continue
		}

		for _, filePath := range files {
			// Skip excluded directories and files
			isExcluded := false
			for _, excludedDir := range excludedDirs {
				if strings.Contains(filePath, excludedDir) {
					isExcluded = true
					break
				}
			}

			if !isExcluded {
				for _, excludedFile := range excludedFiles {
					if strings.HasSuffix(filePath, excludedFile) {
						isExcluded = true
						break
					}
				}
			}

			if isExcluded {
				continue
			}

			content, err := os.ReadFile(filePath)
			if err != nil {
				log.Printf("Failed to read %s: %v", filePath, err)
				continue
			}

			relativePath, err := filepath.Rel(path, filePath)
			if err != nil {
				log.Printf("Failed to get relative path for %s: %v", filePath, err)
				continue
			}

			// Determine if this is an implementation file
			isImplementation := !strings.HasPrefix(filepath.Base(relativePath), "test_") &&
				!strings.HasPrefix(filepath.Base(relativePath), "app_") &&
				!strings.Contains(strings.ToLower(relativePath), "test")

			// Check token count
			tokenCount := utils.CountTokens(string(content), "gpt-4o")
			if tokenCount > 8192 { // Maximum embedding token limit
				log.Printf("Skipping large file %s: Token count (%d) exceeds limit", relativePath, tokenCount)
				continue
			}

			doc := models.Document{
				Text: string(content),
				MetaData: map[string]interface{}{
					"file_path":         relativePath,
					"type":              strings.TrimPrefix(ext, "."),
					"is_code":           true,
					"is_implementation": isImplementation,
					"title":             relativePath,
					"token_count":       tokenCount,
				},
			}
			documents = append(documents, doc)
		}
	}

	// Process document files
	for _, ext := range docExtensions {
		files, err := utils.FindFiles(path, ext)
		if err != nil {
			continue
		}

		for _, filePath := range files {
			// Skip excluded directories and files
			isExcluded := false
			for _, excludedDir := range excludedDirs {
				if strings.Contains(filePath, excludedDir) {
					isExcluded = true
					break
				}
			}

			if !isExcluded {
				for _, excludedFile := range excludedFiles {
					if strings.HasSuffix(filePath, excludedFile) {
						isExcluded = true
						break
					}
				}
			}

			if isExcluded {
				continue
			}

			content, err := os.ReadFile(filePath)
			if err != nil {
				log.Printf("Failed to read %s: %v", filePath, err)
				continue
			}

			relativePath, err := filepath.Rel(path, filePath)
			if err != nil {
				log.Printf("Failed to get relative path for %s: %v", filePath, err)
				continue
			}

			// Check token count
			tokenCount := utils.CountTokens(string(content), "gpt-4o")
			if tokenCount > 8192 { // Maximum embedding token limit
				log.Printf("Skipping large file %s: Token count (%d) exceeds limit", relativePath, tokenCount)
				continue
			}

			doc := models.Document{
				Text: string(content),
				MetaData: map[string]interface{}{
					"file_path":         relativePath,
					"type":              strings.TrimPrefix(ext, "."),
					"is_code":           false,
					"is_implementation": false,
					"title":             relativePath,
					"token_count":       tokenCount,
				},
			}
			documents = append(documents, doc)
		}
	}

	log.Printf("Found %d documents", len(documents))
	return documents, nil
}

// SearchDocuments searches Milvus for documents similar to the query.
func (dm *DatabaseManager) SearchDocuments(query string, topK int) ([]models.Document, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if !dm.initialized {
		return nil, errors.New("DatabaseManager not initialized")
	}

	ctx := context.Background()

	// 1. Get query embedding
	queryEmbedding, err := dm.getEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get query embedding: %w", err)
	}

	// 2. Prepare search parameters
	searchParam, _ := entity.NewIndexHNSWSearchParam(10) // ef parameter for HNSW
	vector := []entity.Vector{entity.FloatVector(queryEmbedding)}

	// 3. Perform search
	log.Printf("Searching Milvus (topK=%d)...", topK)
	searchResult, err := dm.milvusClient.Search(
		ctx,                                                // context
		collectionName,                                     // Collection name
		[]string{},                                         // Partition names (empty for all)
		"",                                                 // Filter expression (empty for none)
		[]string{"file_path", "raw_text", "metadata_json"}, // Output fields
		vector,                                             // Query vectors
		"embedding",                                        // Vector field name
		entity.L2,                                          // Metric type
		topK,                                               // Top K results
		searchParam,                                        // Search parameters
	)
	if err != nil {
		return nil, fmt.Errorf("Milvus search failed: %w", err)
	}

	// Search returns a slice of results, one per query vector. We sent one vector.
	if len(searchResult) == 0 {
		log.Println("Milvus search returned no result sets.")
		return []models.Document{}, nil // Return empty list, not an error
	}

	// Access the results for the first query vector
	singleQueryResult := searchResult[0]

	log.Printf("Milvus search returned %d results.", singleQueryResult.ResultCount)

	// 4. Process results
	var documents []models.Document
	// Extract columns from the Fields slice by name
	var filePathCol, rawTextCol, metadataJSONCol entity.Column
	for _, field := range singleQueryResult.Fields {
		switch field.Name() {
		case "file_path":
			filePathCol = field
		case "raw_text":
			rawTextCol = field
		case "metadata_json":
			metadataJSONCol = field
		}
	}

	// Check if all required columns were found
	if filePathCol == nil || rawTextCol == nil || metadataJSONCol == nil {
		return nil, fmt.Errorf("Milvus search result missing expected columns (file_path, raw_text, metadata_json) in Fields")
	}

	// Perform type assertion
	filePathData, ok1 := filePathCol.(*entity.ColumnVarChar)
	rawTextData, ok2 := rawTextCol.(*entity.ColumnVarChar)
	metadataJSONData, ok3 := metadataJSONCol.(*entity.ColumnVarChar)

	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("Milvus search result columns have unexpected types (expected VarChar)")
	}

	for i := 0; i < int(singleQueryResult.ResultCount); i++ { // Use int() conversion for loop range
		// Check index bounds just in case, though ResultCount should match column length
		if i >= filePathData.Len() || i >= rawTextData.Len() || i >= metadataJSONData.Len() {
			log.Printf("Warning: Milvus result index %d out of bounds for column length", i)
			continue
		}

		filePath, err1 := filePathData.ValueByIdx(i)
		rawText, err2 := rawTextData.ValueByIdx(i)
		metadataJSON, err3 := metadataJSONData.ValueByIdx(i)

		if err1 != nil || err2 != nil || err3 != nil {
			log.Printf("Warning: failed to retrieve values for index %d: %v, %v, %v", i, err1, err2, err3)
			continue
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			log.Printf("Warning: failed to unmarshal metadata for '%s': %v", filePath, err)
			metadata = make(map[string]interface{})
			metadata["error"] = "failed to parse stored metadata"
			metadata["file_path"] = filePath // Ensure file_path is present
		}

		doc := models.Document{
			Text:     rawText,
			MetaData: metadata,
			// Score: singleQueryResult.Scores[i],
		}
		documents = append(documents, doc)
	}

	return documents, nil
}

// getEmbedding generates a placeholder embedding for text.
// Replace this with your actual embedding model call.
func (dm *DatabaseManager) getEmbedding(text string) ([]float32, error) {
	// Placeholder: Generate a random vector
	// In a real application, call your embedding model API (e.g., OpenAI, Sentence-Transformers)
	vec := make([]float32, embeddingDimension)
	for i := range vec {
		vec[i] = rand.Float32()
	}
	// Normalize the vector (optional, but often recommended for cosine similarity)
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec, nil
}

// AddDocument adds a single document to the Milvus database.
// This is likely for adding documents outside the initial batch indexing.
func (dm *DatabaseManager) AddDocument(doc *models.Document) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if !dm.initialized {
		return errors.New("DatabaseManager not initialized")
	}

	err := dm.addDocumentInternal(doc)
	if err != nil {
		return err
	}

	// Flush immediately after single add for consistency?
	err = dm.milvusClient.Flush(context.Background(), collectionName, false)
	if err != nil {
		log.Printf("Warning: failed to flush collection '%s' after single add: %v", collectionName, err)
	}

	log.Printf("Added single document '%s' to Milvus.", doc.MetaData["file_path"])
	return nil
}

// GetDocument retrieves a document by its identifier (e.g., file path).
// Note: This searches based on the file_path field, not the primary key directly.
func (dm *DatabaseManager) GetDocument(filePath string) (*models.Document, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if !dm.initialized {
		return nil, errors.New("DatabaseManager not initialized")
	}

	ctx := context.Background()
	docID := generateDocID(filePath)

	// Query Milvus by primary key (doc_id)
	log.Printf("Querying Milvus for doc_id: %d (path: %s)", docID, filePath)
	results, err := dm.milvusClient.Query(
		ctx,
		collectionName,
		[]string{},                                         // No partition names
		fmt.Sprintf("doc_id == %d", docID),                 // Filter expression by primary key
		[]string{"file_path", "raw_text", "metadata_json"}, // Output fields
	)
	if err != nil {
		return nil, fmt.Errorf("Milvus query for ID %d failed: %w", docID, err)
	}

	if results.Len() == 0 {
		return nil, fmt.Errorf("document with path '%s' (ID: %d) not found in Milvus", filePath, docID)
	}

	// Should only be one result for a primary key query
	// Use GetColumn directly on client.ResultSet
	rawTextField := results.GetColumn("raw_text")
	metadataJSONField := results.GetColumn("metadata_json")

	// Check if all required columns were found
	if rawTextField == nil || metadataJSONField == nil {
		return nil, fmt.Errorf("Milvus query result missing expected columns (raw_text, metadata_json)")
	}

	// Perform type assertion
	rawTextData, ok1 := rawTextField.(*entity.ColumnVarChar)
	metadataJSONData, ok2 := metadataJSONField.(*entity.ColumnVarChar)

	if !ok1 || !ok2 {
		return nil, fmt.Errorf("Milvus query result columns have unexpected types (expected VarChar)")
	}

	// filePath is already declared as function argument, use = instead of :=
	// Also, we query by doc_id derived from filePath, so we don't need to retrieve it again.
	// We only need raw_text and metadata_json.
	if rawTextData.Len() == 0 || metadataJSONData.Len() == 0 {
		return nil, fmt.Errorf("Milvus query result columns are empty for doc_id %d", docID)
	}

	rawText, err1 := rawTextData.ValueByIdx(0)
	metadataJSON, err2 := metadataJSONData.ValueByIdx(0)

	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("failed to retrieve values from Milvus query result for doc_id %d: %v, %v", docID, err1, err2)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		log.Printf("Warning: failed to unmarshal metadata for '%s': %v", filePath, err)
		metadata = make(map[string]interface{})
		metadata["error"] = "failed to parse stored metadata"
		metadata["file_path"] = filePath
	}

	doc := &models.Document{
		Text:     rawText,
		MetaData: metadata,
	}

	return doc, nil
}

// DeleteDocument removes a document by its identifier (e.g., file path).
func (dm *DatabaseManager) DeleteDocument(filePath string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if !dm.initialized {
		return errors.New("DatabaseManager not initialized")
	}

	ctx := context.Background()
	docID := generateDocID(filePath)

	// Delete from Milvus by primary key (doc_id)
	log.Printf("Deleting document from Milvus with doc_id: %d (path: %s)", docID, filePath)
	err := dm.milvusClient.Delete(
		ctx,
		collectionName,
		"",                                 // No partition names
		fmt.Sprintf("doc_id == %d", docID), // Filter expression by primary key
	)
	if err != nil {
		return fmt.Errorf("Milvus delete for ID %d (path: '%s') failed: %w", docID, filePath, err)
	}

	log.Printf("Successfully deleted document '%s' (ID: %d) from Milvus.", filePath, docID)

	// Optionally flush immediately
	err = dm.milvusClient.Flush(context.Background(), collectionName, false)
	if err != nil {
		log.Printf("Warning: failed to flush collection '%s' after delete: %v", collectionName, err)
	}

	return nil
}
