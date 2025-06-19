
## 后端代码 (Go)

### 1. 项目初始化
```bash
mkdir deepwiki-backend
cd deepwiki-backend
go mod init deepwiki-backend
```

### 2. go.mod
```go
module deepwiki-backend

go 1.21

require (
    github.com/gin-contrib/cors v1.4.0
    github.com/gin-gonic/gin v1.9.1
    github.com/go-git/go-git/v5 v5.11.0
    github.com/sashabaranov/go-openai v1.17.9
    go.mongodb.org/mongo-driver v1.13.1
    github.com/joho/godotenv v1.4.0
)
```

### 3. main.go
```go
// cmd/server/main.go
package main

import (
    "log"
    "deepwiki-backend/internal/api"
    "deepwiki-backend/internal/config"
    "deepwiki-backend/internal/storage"
    "deepwiki-backend/internal/services"
)

func main() {
    // 加载配置
    cfg := config.Load()
    
    // 初始化数据库
    db, err := storage.NewMongoDB(cfg.MongoURL)
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }
    defer db.Close()
    
    // 初始化服务
    services := services.NewServices(db, cfg)
    
    // 启动API服务器
    server := api.NewServer(services)
    log.Printf("Server starting on port %s", cfg.Port)
    if err := server.Run(":" + cfg.Port); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}
```

### 4. 配置管理
```go
// internal/config/config.go
package config

import (
    "os"
    "github.com/joho/godotenv"
)

type Config struct {
    Port        string
    MongoURL    string
    OpenAIKey   string
    GitHubToken string
}

func Load() *Config {
    godotenv.Load()
    
    return &Config{
        Port:        getEnv("PORT", "8080"),
        MongoURL:    getEnv("MONGO_URL", "mongodb://localhost:27017/deepwiki"),
        OpenAIKey:   getEnv("OPENAI_API_KEY", ""),
        GitHubToken: getEnv("GITHUB_TOKEN", ""),
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### 5. 数据模型
```go
// internal/models/repository.go
package models

import (
    "time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type Repository struct {
    ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    URL         string            `bson:"url" json:"url"`
    Name        string            `bson:"name" json:"name"`
    Description string            `bson:"description" json:"description"`
    Language    string            `bson:"language" json:"language"`
    Stars       int               `bson:"stars" json:"stars"`
    Status      string            `bson:"status" json:"status"` // processing, completed, failed
    Structure   *RepoStructure    `bson:"structure,omitempty" json:"structure,omitempty"`
    CreatedAt   time.Time         `bson:"created_at" json:"created_at"`
    UpdatedAt   time.Time         `bson:"updated_at" json:"updated_at"`
}

type RepoStructure struct {
    Files        []FileInfo     `bson:"files" json:"files"`
    Dependencies []string       `bson:"dependencies" json:"dependencies"`
    Modules      []ModuleInfo   `bson:"modules" json:"modules"`
}

type FileInfo struct {
    Path     string `bson:"path" json:"path"`
    Type     string `bson:"type" json:"type"`
    Language string `bson:"language" json:"language"`
    Size     int64  `bson:"size" json:"size"`
}

type ModuleInfo struct {
    Name      string   `bson:"name" json:"name"`
    Path      string   `bson:"path" json:"path"`
    Functions []string `bson:"functions" json:"functions"`
    Classes   []string `bson:"classes" json:"classes"`
}

type CodeChunk struct {
    ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    RepoID   primitive.ObjectID `bson:"repo_id" json:"repo_id"`
    FilePath string            `bson:"file_path" json:"file_path"`
    Content  string            `bson:"content" json:"content"`
    Language string            `bson:"language" json:"language"`
    StartLine int              `bson:"start_line" json:"start_line"`
    EndLine   int              `bson:"end_line" json:"end_line"`
}
```

### 6. 数据库连接
```go
// internal/storage/mongodb.go
package storage

import (
    "context"
    "time"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
    client   *mongo.Client
    database *mongo.Database
}

func NewMongoDB(uri string) (*MongoDB, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
    if err != nil {
        return nil, err
    }
    
    if err := client.Ping(ctx, nil); err != nil {
        return nil, err
    }
    
    return &MongoDB{
        client:   client,
        database: client.Database("deepwiki"),
    }, nil
}

func (m *MongoDB) Close() error {
    return m.client.Disconnect(context.Background())
}

func (m *MongoDB) GetCollection(name string) *mongo.Collection {
    return m.database.Collection(name)
}
```

### 7. GitHub 服务
```go
// internal/services/github.go
package services

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "github.com/go-git/go-git/v5"
    "deepwiki-backend/internal/models"
)

type GitHubService struct {
    token string
}

func NewGitHubService(token string) *GitHubService {
    return &GitHubService{token: token}
}

func (s *GitHubService) CloneRepository(repoURL string) (string, error) {
    // 创建临时目录
    tempDir := filepath.Join("/tmp", "repos", extractRepoName(repoURL))
    os.MkdirAll(tempDir, 0755)
    
    // 克隆仓库
    _, err := git.PlainClone(tempDir, false, &git.CloneOptions{
        URL: repoURL,
    })
    
    return tempDir, err
}

func (s *GitHubService) AnalyzeRepository(repoPath string) (*models.RepoStructure, error) {
    structure := &models.RepoStructure{
        Files:        []models.FileInfo{},
        Dependencies: []string{},
        Modules:      []models.ModuleInfo{},
    }
    
    err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        if !info.IsDir() && isCodeFile(path) {
            relPath, _ := filepath.Rel(repoPath, path)
            fileInfo := models.FileInfo{
                Path:     relPath,
                Type:     "file",
                Language: detectLanguage(path),
                Size:     info.Size(),
            }
            structure.Files = append(structure.Files, fileInfo)
        }
        
        return nil
    })
    
    return structure, err
}

func extractRepoName(url string) string {
    parts := strings.Split(url, "/")
    return strings.TrimSuffix(parts[len(parts)-1], ".git")
}

func isCodeFile(path string) bool {
    ext := filepath.Ext(path)
    codeExts := []string{".go", ".js", ".py", ".java", ".cpp", ".c", ".ts", ".jsx", ".tsx"}
    
    for _, codeExt := range codeExts {
        if ext == codeExt {
            return true
        }
    }
    return false
}

func detectLanguage(path string) string {
    ext := filepath.Ext(path)
    langMap := map[string]string{
        ".go":  "Go",
        ".js":  "JavaScript",
        ".ts":  "TypeScript",
        ".jsx": "React",
        ".tsx": "React",
        ".py":  "Python",
        ".java": "Java",
        ".cpp": "C++",
        ".c":   "C",
    }
    
    if lang, ok := langMap[ext]; ok {
        return lang
    }
    return "Unknown"
}
```

### 8. LLM 服务
```go
// internal/services/llm.go
package services

import (
    "context"
    "fmt"
    "github.com/sashabaranov/go-openai"
)

type LLMService struct {
    client *openai.Client
}

func NewLLMService(apiKey string) *LLMService {
    return &LLMService{
        client: openai.NewClient(apiKey),
    }
}

func (s *LLMService) QueryCode(codeContext, question string) (string, error) {
    prompt := fmt.Sprintf(`
Based on the following code context, please answer the question:

Code Context:
%s

Question: %s

Please provide a detailed and accurate answer based on the code.
`, codeContext, question)

    resp, err := s.client.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4o,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: prompt,
                },
            },
        },
    )

    if err != nil {
        return "", err
    }

    return resp.Choices[0].Message.Content, nil
}
```

### 9. 仓库服务
```go
// internal/services/repository.go
package services

import (
    "context"
    "fmt"
    "io/ioutil"
    "path/filepath"
    "time"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "deepwiki-backend/internal/models"
    "deepwiki-backend/internal/storage"
)

type RepositoryService struct {
    db     *storage.MongoDB
    github *GitHubService
    llm    *LLMService
}

func NewRepositoryService(db *storage.MongoDB, github *GitHubService, llm *LLMService) *RepositoryService {
    return &RepositoryService{
        db:     db,
        github: github,
        llm:    llm,
    }
}

func (s *RepositoryService) SubmitRepository(repoURL string) (*models.Repository, error) {
    // 创建仓库记录
    repo := &models.Repository{
        ID:        primitive.NewObjectID(),
        URL:       repoURL,
        Name:      extractRepoName(repoURL),
        Status:    "processing",
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    
    // 保存到数据库
    collection := s.db.GetCollection("repositories")
    _, err := collection.InsertOne(context.Background(), repo)
    if err != nil {
        return nil, err
    }
    
    // 异步处理仓库分析
    go s.processRepository(repo)
    
    return repo, nil
}

func (s *RepositoryService) processRepository(repo *models.Repository) {
    // 克隆仓库
    repoPath, err := s.github.CloneRepository(repo.URL)
    if err != nil {
        s.updateRepositoryStatus(repo.ID, "failed")
        return
    }
    
    // 分析仓库结构
    structure, err := s.github.AnalyzeRepository(repoPath)
    if err != nil {
        s.updateRepositoryStatus(repo.ID, "failed")
        return
    }
    
    // 提取代码块
    s.extractCodeChunks(repo.ID, repoPath, structure)
    
    // 更新仓库信息
    s.updateRepository(repo.ID, structure)
}

func (s *RepositoryService) extractCodeChunks(repoID primitive.ObjectID, repoPath string, structure *models.RepoStructure) {
    collection := s.db.GetCollection("code_chunks")
    
    for _, file := range structure.Files {
        if isCodeFile(file.Path) {
            fullPath := filepath.Join(repoPath, file.Path)
            content, err := ioutil.ReadFile(fullPath)
            if err != nil {
                continue
            }
            
            chunk := &models.CodeChunk{
                ID:       primitive.NewObjectID(),
                RepoID:   repoID,
                FilePath: file.Path,
                Content:  string(content),
                Language: file.Language,
                StartLine: 1,
                EndLine:   len(string(content)),
            }
            
            collection.InsertOne(context.Background(), chunk)
        }
    }
}

func (s *RepositoryService) updateRepository(repoID primitive.ObjectID, structure *models.RepoStructure) {
    collection := s.db.GetCollection("repositories")
    update := bson.M{
        "$set": bson.M{
            "structure":  structure,
            "status":     "completed",
            "updated_at": time.Now(),
        },
    }
    collection.UpdateOne(context.Background(), bson.M{"_id": repoID}, update)
}

func (s *RepositoryService) updateRepositoryStatus(repoID primitive.ObjectID, status string) {
    collection := s.db.GetCollection("repositories")
    update := bson.M{
        "$set": bson.M{
            "status":     status,
            "updated_at": time.Now(),
        },
    }
    collection.UpdateOne(context.Background(), bson.M{"_id": repoID}, update)
}

func (s *RepositoryService) GetRepository(repoID string) (*models.Repository, error) {
    objID, err := primitive.ObjectIDFromHex(repoID)
    if err != nil {
        return nil, err
    }
    
    collection := s.db.GetCollection("repositories")
    var repo models.Repository
    err = collection.FindOne(context.Background(), bson.M{"_id": objID}).Decode(&repo)
    return &repo, err
}

func (s *RepositoryService) QueryRepository(repoID, question string) (string, error) {
    objID, err := primitive.ObjectIDFromHex(repoID)
    if err != nil {
        return "", err
    }
    
    // 获取相关代码块
    collection := s.db.GetCollection("code_chunks")
    cursor, err := collection.Find(context.Background(), bson.M{"repo_id": objID})
    if err != nil {
        return "", err
    }
    defer cursor.Close(context.Background())
    
    var codeContext string
    for cursor.Next(context.Background()) {
        var chunk models.CodeChunk
        if err := cursor.Decode(&chunk); err != nil {
            continue
        }
        codeContext += fmt.Sprintf("File: %s\n%s\n\n", chunk.FilePath, chunk.Content)
    }
    
    // 调用 LLM
    return s.llm.QueryCode(codeContext, question)
}

func (s *RepositoryService) ListRepositories() ([]*models.Repository, error) {
    collection := s.db.GetCollection("repositories")
    cursor, err := collection.Find(context.Background(), bson.M{})
    if err != nil {
        return nil, err
    }
    defer cursor.Close(context.Background())
    
    var repos []*models.Repository
    for cursor.Next(context.Background()) {
        var repo models.Repository
        if err := cursor.Decode(&repo); err != nil {
            continue
        }
        repos = append(repos, &repo)
    }
    
    return repos, nil
}
```

### 10. 服务集合
```go
// internal/services/services.go
package services

import (
    "deepwiki-backend/internal/config"
    "deepwiki-backend/internal/storage"
)

type Services struct {
    Repository *RepositoryService
    GitHub     *GitHubService
    LLM        *LLMService
}

func NewServices(db *storage.MongoDB, cfg *config.Config) *Services {
    github := NewGitHubService(cfg.GitHubToken)
    llm := NewLLMService(cfg.OpenAIKey)
    repository := NewRepositoryService(db, github, llm)
    
    return &Services{
        Repository: repository,
        GitHub:     github,
        LLM:        llm,
    }
}
```

### 11. API 处理器
```go
// internal/api/handlers/repository.go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "deepwiki-backend/internal/services"
)

type RepositoryHandler struct {
    services *services.Services
}

func NewRepositoryHandler(services *services.Services) *RepositoryHandler {
    return &RepositoryHandler{services: services}
}

type SubmitRepoRequest struct {
    RepoURL string `json:"repo_url" binding:"required"`
}

type QueryRepoRequest struct {
    Question string `json:"question" binding:"required"`
}

func (h *RepositoryHandler) SubmitRepository(c *gin.Context) {
    var req SubmitRepoRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    repo, err := h.services.Repository.SubmitRepository(req.RepoURL)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusAccepted, repo)
}

func (h *RepositoryHandler) GetRepository(c *gin.Context) {
    repoID := c.Param("id")
    
    repo, err := h.services.Repository.GetRepository(repoID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
        return
    }
    
    c.JSON(http.StatusOK, repo)
}

func (h *RepositoryHandler) QueryRepository(c *gin.Context) {
    repoID := c.Param("id")
    
    var req QueryRepoRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    answer, err := h.services.Repository.QueryRepository(repoID, req.Question)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "question": req.Question,
        "answer":   answer,
    })
}

func (h *RepositoryHandler) ListRepositories(c *gin.Context) {
    repos, err := h.services.Repository.ListRepositories()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"repositories": repos})
}
```

### 12. API 路由
```go
// internal/api/routes/routes.go
package routes

import (
    "github.com/gin-gonic/gin"
    "deepwiki-backend/internal/api/handlers"
    "deepwiki-backend/internal/services"
)

func SetupRoutes(r *gin.Engine, services *services.Services) {
    repoHandler := handlers.NewRepositoryHandler(services)
    
    api := r.Group("/api/v1")
    {
        api.POST("/repositories", repoHandler.SubmitRepository)
        api.GET("/repositories", repoHandler.ListRepositories)
        api.GET("/repositories/:id", repoHandler.GetRepository)
        api.POST("/repositories/:id/query", repoHandler.QueryRepository)
    }
}
```

### 13. API 服务器
```go
// internal/api/server.go
package api

import (
    "github.com/gin-contrib/cors"
    "github.com/gin-gonic/gin"
    "deepwiki-backend/internal/api/routes"
    "deepwiki-backend/internal/services"
)

func NewServer(services *services.Services) *gin.Engine {
    r := gin.Default()
    
    // CORS 配置
    config := cors.DefaultConfig()
    config.AllowOrigins = []string{"http://localhost:3000"}
    config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
    config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
    r.Use(cors.New(config))
    
    // 设置路由
    routes.SetupRoutes(r, services)
    
    return r
}
```

### 14. 环境变量文件
```bash
# .env
PORT=8080
MONGO_URL=mongodb://localhost:27017/deepwiki
OPENAI_API_KEY=your_openai_api_key_here
GITHUB_TOKEN=your_github_token_here
```

## 前端代码 (React)

### 1. 项目初始化
```bash
npx create-react-app deepwiki-frontend
cd deepwiki-frontend
npm install axios react-router-dom styled-components prism-react-renderer
```

### 2. package.json 添加依赖
```json
{
  "dependencies": {
    "axios": "^1.6.0",
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.8.0",
    "styled-components": "^6.1.0",
    "prism-react-renderer": "^2.3.0"
  }
}
```

### 3. API 服务
```jsx
// src/services/api.js
import axios from 'axios';

const API_BASE_URL = 'http://localhost:8080/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

export const repositoryAPI = {
  // 提交仓库
  submit: (repoUrl) => 
    api.post('/repositories', { repo_url: repoUrl }),
  
  // 获取仓库列表
  list: () => 
    api.get('/repositories'),
  
  // 获取单个仓库
  get: (id) => 
    api.get(`/repositories/${id}`),
  
  // 查询仓库
  query: (id, question) => 
    api.post(`/repositories/${id}/query`, { question }),
};

export default api;
```

### 4. 主要组件

#### App.js
```jsx
// src/App.js
import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import styled from 'styled-components';
import Header from './components/Header';
import HomePage from './pages/HomePage';
import RepositoryPage from './pages/RepositoryPage';
import RepositoryDetail from './pages/RepositoryDetail';

const AppContainer = styled.div`
  min-height: 100vh;
  background-color: #f5f5f5;
`;

const MainContent = styled.main`
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
`;

function App() {
  return (
    <Router>
      <AppContainer>
        <Header />
        <MainContent>
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/repositories" element={<RepositoryPage />} />
            <Route path="/repositories/:id" element={<RepositoryDetail />} />
          </Routes>
        </MainContent>
      </AppContainer>
    </Router>
  );
}

export default App;
```

#### Header 组件
```jsx
// src/components/Header.js
import React from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

const HeaderContainer = styled.header`
  background-color: #2c3e50;
  color: white;
  padding: 1rem 0;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
`;

const HeaderContent = styled.div`
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

const Logo = styled(Link)`
  font-size: 1.5rem;
  font-weight: bold;
  text-decoration: none;
  color: white;
  
  &:hover {
    color: #3498db;
  }
`;

const Nav = styled.nav`
  display: flex;
  gap: 2rem;
`;

const NavLink = styled(Link)`
  color: white;
  text-decoration: none;
  
  &:hover {
    color: #3498db;
  }
`;

function Header() {
  return (
    <HeaderContainer>
      <HeaderContent>
        <Logo to="/">DeepWiki</Logo>
        <Nav>
          <NavLink to="/">首页</NavLink>
          <NavLink to="/repositories">仓库</NavLink>
        </Nav>
      </HeaderContent>
    </HeaderContainer>
  );
}

export default Header;
```

#### RepoSubmit 组件
```jsx
// src/components/RepoSubmit.js
import React, { useState } from 'react';
import styled from 'styled-components';
import { repositoryAPI } from '../services/api';

const FormContainer = styled.div`
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  margin-bottom: 2rem;
`;

const Title = styled.h2`
  margin-bottom: 1rem;
  color: #2c3e50;
`;

const Form = styled.form`
  display: flex;
  flex-direction: column;
  gap: 1rem;
`;

const Input = styled.input`
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  
  &:focus {
    outline: none;
    border-color: #3498db;
  }
`;

const Button = styled.button`
  padding: 0.75rem 1.5rem;
  background-color: #3498db;
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 1rem;
  cursor: pointer;
  
  &:hover {
    background-color: #2980b9;
  }
  
  &:disabled {
    background-color: #bdc3c7;
    cursor: not-allowed;
  }
`;

const Message = styled.div`
  padding: 0.75rem;
  border-radius: 4px;
  ${props => props.error ? `
    background-color: #e74c3c;
    color: white;
  ` : `
    background-color: #2ecc71;
    color: white;
  `}
`;

function RepoSubmit({ onSubmitSuccess }) {
  const [repoUrl, setRepoUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!repoUrl.trim()) return;

    setLoading(true);
    setMessage('');
    setError(false);

    try {
      const response = await repositoryAPI.submit(repoUrl);
      setMessage('仓库提交成功，正在分析中...');
      setRepoUrl('');
      if (onSubmitSuccess) {
        onSubmitSuccess(response.data);
      }
    } catch (err) {
      setError(true);
      setMessage(err.response?.data?.error || '提交失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <FormContainer>
      <Title>提交 GitHub 仓库</Title>
      <Form onSubmit={handleSubmit}>
        <Input
          type="url"
          value={repoUrl}
          onChange={(e) => setRepoUrl(e.target.value)}
          placeholder="https://github.com/user/repository"
          disabled={loading}
        />
        <Button type="submit" disabled={loading || !repoUrl.trim()}>
          {loading ? '提交中...' : '提交'}
        </Button>
      </Form>
      {message && (
        <Message error={error}>
          {message}
        </Message>
      )}
    </FormContainer>
  );
}

export default RepoSubmit;
```

#### RepositoryList 组件
```jsx
// src/components/RepositoryList.js
import React from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

const ListContainer = styled.div`
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
`;

const Title = styled.h2`
  padding: 1.5rem;
  margin: 0;
  border-bottom: 1px solid #eee;
  color: #2c3e50;
`;

const RepoItem = styled(Link)`
  display: block;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid #eee;
  text-decoration: none;
  color: inherit;
  transition: background-color 0.2s;
  
  &:hover {
    background-color: #f8f9fa;
  }
  
  &:last-child {
    border-bottom: none;
  }
`;

const RepoName = styled.h3`
  margin: 0 0 0.5rem 0;
  color: #3498db;
`;

const RepoInfo = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.9rem;
  color: #666;
`;

const Status = styled.span`
  padding: 0.25rem 0.5rem;
  border-radius: 12px;
  font-size: 0.8rem;
  ${props => {
    switch(props.status) {
      case 'completed':
        return 'background-color: #2ecc71; color: white;';
      case 'processing':
        return 'background-color: #f39c12; color: white;';
      case 'failed':
        return 'background-color: #e74c3c; color: white;';
      default:
        return 'background-color: #95a5a6; color: white;';
    }
  }}
`;

const EmptyState = styled.div`
  padding: 3rem;
  text-align: center;
  color: #666;
`;

function RepositoryList({ repositories }) {
  if (!repositories || repositories.length === 0) {
    return (
      <ListContainer>
        <Title>仓库列表</Title>
        <EmptyState>
          暂无仓库，请先提交一个 GitHub 仓库
        </EmptyState>
      </ListContainer>
    );
  }

  return (
    <ListContainer>
      <Title>仓库列表</Title>
      {repositories.map((repo) => (
        <RepoItem key={repo.id} to={`/repositories/${repo.id}`}>
          <RepoName>{repo.name}</RepoName>
          <RepoInfo>
            <span>{repo.language}</span>
            <Status status={repo.status}>
              {repo.status === 'completed' ? '已完成' : 
               repo.status === 'processing' ? '分析中' : '失败'}
            </Status>
          </RepoInfo>
        </RepoItem>
      ))}
    </ListContainer>
  );
}

export default RepositoryList;
```

#### CodeQuery 组件
```jsx
// src/components/CodeQuery.js
import React, { useState } from 'react';
import styled from 'styled-components';
import { repositoryAPI } from '../services/api';

const QueryContainer = styled.div`
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  margin-bottom: 2rem;
`;

const Title = styled.h3`
  margin-bottom: 1rem;
  color: #2c3e50;
`;

const QueryForm = styled.form`
  display: flex;
  flex-direction: column;
  gap: 1rem;
`;

const TextArea = styled.textarea`
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  min-height: 100px;
  resize: vertical;
  
  &:focus {
    outline: none;
    border-color: #3498db;
  }
`;

const Button = styled.button`
  padding: 0.75rem 1.5rem;
  background-color: #3498db;
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 1rem;
  cursor: pointer;
  align-self: flex-start;
  
  &:hover {
    background-color: #2980b9;
  }
  
  &:disabled {
    background-color: #bdc3c7;
    cursor: not-allowed;
  }
`;

const AnswerContainer = styled.div`
  margin-top: 2rem;
  padding: 1.5rem;
  background-color: #f8f9fa;
  border-radius: 4px;
  border-left: 4px solid #3498db;
`;

const AnswerTitle = styled.h4`
  margin: 0 0 1rem 0;
  color: #2c3e50;
`;

const AnswerText = styled.div`
  line-height: 1.6;
  white-space: pre-wrap;
`;

function CodeQuery({ repositoryId }) {
  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!question.trim()) return;

    setLoading(true);
    try {
      const response = await repositoryAPI.query(repositoryId, question);
      setAnswer(response.data.answer);
    } catch (err) {
      setAnswer('查询失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <QueryContainer>
      <Title>询问关于此仓库的问题</Title>
      <QueryForm onSubmit={handleSubmit}>
        <TextArea
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          placeholder="例如：这个项目的主要架构是什么？主要的类和函数有哪些？"
          disabled={loading}
        />
        <Button type="submit" disabled={loading || !question.trim()}>
          {loading ? '查询中...' : '提问'}
        </Button>
      </QueryForm>
      
      {answer && (
        <AnswerContainer>
          <AnswerTitle>回答：</AnswerTitle>
          <AnswerText>{answer}</AnswerText>
        </AnswerContainer>
      )}
    </QueryContainer>
  );
}

export default CodeQuery;
```

### 5. 页面组件

#### HomePage
```jsx
// src/pages/HomePage.js
import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import RepoSubmit from '../components/RepoSubmit';
import RepositoryList from '../components/RepositoryList';
import { repositoryAPI } from '../services/api';

const PageContainer = styled.div`
  max-width: 800px;
  margin: 0 auto;
`;

const Welcome = styled.div`
  text-align: center;
  margin-bottom: 3rem;
`;

const Title = styled.h1`
  color: #2c3e50;
  margin-bottom: 1rem;
`;

const Subtitle = styled.p`
  color: #666;
  font-size: 1.2rem;
`;

function HomePage() {
  const [repositories, setRepositories] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchRepositories();
  }, []);

  const fetchRepositories = async () => {
    try {
      const response = await repositoryAPI.list();
      setRepositories(response.data.repositories || []);
    } catch (err) {
      console.error('Failed to fetch repositories:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmitSuccess = () => {
    fetchRepositories();
  };

  if (loading) {
    return <div>加载中...</div>;
  }

  return (
    <PageContainer>
      <Welcome>
        <Title>欢迎使用 DeepWiki</Title>
        <Subtitle>
          快速理解和探索 GitHub 仓库，通过 AI 获得深入的代码洞察
        </Subtitle>
      </Welcome>
      
      <RepoSubmit onSubmitSuccess={handleSubmitSuccess} />
      <RepositoryList repositories={repositories} />
    </PageContainer>
  );
}

export default HomePage;
```

#### RepositoryDetail
```jsx
// src/pages/RepositoryDetail.js
import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import styled from 'styled-components';
import CodeQuery from '../components/CodeQuery';
import { repositoryAPI } from '../services/api';

const PageContainer = styled.div`
  max-width: 1000px;
  margin: 0 auto;
`;

const RepoHeader = styled.div`
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  margin-bottom: 2rem;
`;

const RepoName = styled.h1`
  margin: 0 0 1rem 0;
  color: #2c3e50;
`;

const RepoInfo = styled.div`
  display: flex;
  gap: 2rem;
  align-items: center;
  color: #666;
`;

const Status = styled.span`
  padding: 0.5rem 1rem;
  border-radius: 20px;
  font-size: 0.9rem;
  ${props => {
    switch(props.status) {
      case 'completed':
        return 'background-color: #2ecc71; color: white;';
      case 'processing':
        return 'background-color: #f39c12; color: white;';
      case 'failed':
        return 'background-color: #e74c3c; color: white;';
      default:
        return 'background-color: #95a5a6; color: white;';
    }
  }}
`;

const StructureContainer = styled.div`
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  margin-bottom: 2rem;
`;

const FileList = styled.div`
  max-height: 400px;
  overflow-y: auto;
`;

const FileItem = styled.div`
  padding: 0.5rem;
  border-bottom: 1px solid #eee;
  display: flex;
  justify-content: space-between;
  
  &:last-child {
    border-bottom: none;
  }
`;

const LoadingMessage = styled.div`
  text-align: center;
  padding: 3rem;
  color: #666;
`;

function RepositoryDetail() {
  const { id } = useParams();
  const [repository, setRepository] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchRepository();
  }, [id]);

  const fetchRepository = async () => {
    try {
      const response = await repositoryAPI.get(id);
      setRepository(response.data);
    } catch (err) {
      console.error('Failed to fetch repository:', err);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return <LoadingMessage>加载中...</LoadingMessage>;
  }

  if (!repository) {
    return <LoadingMessage>仓库未找到</LoadingMessage>;
  }

  return (
    <PageContainer>
      <RepoHeader>
        <RepoName>{repository.name}</RepoName>
        <RepoInfo>
          <span>语言: {repository.language}</span>
          <Status status={repository.status}>
            {repository.status === 'completed' ? '已完成' : 
             repository.status === 'processing' ? '分析中' : '失败'}
          </Status>
        </RepoInfo>
      </RepoHeader>

      {repository.status === 'completed' && (
        <>
          <CodeQuery repositoryId={id} />
          
          {repository.structure && (
            <StructureContainer>
              <h3>仓库结构</h3>
              <FileList>
                {repository.structure.files.map((file, index) => (
                  <FileItem key={index}>
                    <span>{file.path}</span>
                    <span>{file.language}</span>
                  </FileItem>
                ))}
              </FileList>
            </StructureContainer>
          )}
        </>
      )}
      
      {repository.status === 'processing' && (
        <LoadingMessage>
          仓库正在分析中，请稍后刷新页面...
        </LoadingMessage>
      )}
      
      {repository.status === 'failed' && (
        <LoadingMessage>
          仓库分析失败，请重新提交
        </LoadingMessage>
      )}
    </PageContainer>
  );
}

export default RepositoryDetail;
```

### 6. 启动脚本

#### 后端启动
```bash
# 在 deepwiki-backend 目录下
go run cmd/server/main.go
```

#### 前端启动
```bash
# 在 deepwiki-frontend 目录下
npm start
```

## 优化后的AI架构

### 1. 向量数据库集成 (使用Qdrant)

```go
// internal/storage/vectordb.go
package storage

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/qdrant/go-client/qdrant"
)

type VectorDB struct {
    client *qdrant.Client
}

type CodeVector struct {
    ID       string                 `json:"id"`
    Vector   []float32             `json:"vector"`
    Payload  map[string]interface{} `json:"payload"`
}

func NewVectorDB(url string) (*VectorDB, error) {
    client, err := qdrant.NewClient(&qdrant.Config{
        Host: url,
    })
    if err != nil {
        return nil, err
    }
    
    return &VectorDB{client: client}, nil
}

func (v *VectorDB) CreateCollection(collectionName string, vectorSize int) error {
    return v.client.CreateCollection(context.Background(), &qdrant.CreateCollection{
        CollectionName: collectionName,
        VectorsConfig: &qdrant.VectorsConfig{
            Params: &qdrant.VectorParams{
                Size:     uint64(vectorSize),
                Distance: qdrant.Distance_Cosine,
            },
        },
    })
}

func (v *VectorDB) UpsertVectors(collectionName string, vectors []CodeVector) error {
    points := make([]*qdrant.PointStruct, len(vectors))
    
    for i, vector := range vectors {
        points[i] = &qdrant.PointStruct{
            Id:      &qdrant.PointId{PointIdOptions: &qdrant.PointId_Uuid{Uuid: vector.ID}},
            Vectors: &qdrant.Vectors{VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: vector.Vector}}},
            Payload: vector.Payload,
        }
    }
    
    _, err := v.client.Upsert(context.Background(), &qdrant.UpsertPoints{
        CollectionName: collectionName,
        Points:         points,
    })
    
    return err
}

func (v *VectorDB) SearchSimilar(collectionName string, queryVector []float32, limit int) ([]*qdrant.ScoredPoint, error) {
    response, err := v.client.Search(context.Background(), &qdrant.SearchPoints{
        CollectionName: collectionName,
        Vector:         queryVector,
        Limit:         uint64(limit),
        WithPayload:   &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
    })
    
    if err != nil {
        return nil, err
    }
    
    return response.Result, nil
}
```

### 2. 增强的代码分析器

```go
// internal/services/code_analyzer.go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "go/ast"
    "go/parser"
    "go/token"
    "path/filepath"
    "strings"
    
    "github.com/sashabaranov/go-openai"
)

type CodeAnalyzer struct {
    openaiClient *openai.Client
    vectorDB     *storage.VectorDB
}

type CodeBlock struct {
    FilePath    string            `json:"file_path"`
    Language    string            `json:"language"`
    Content     string            `json:"content"`
    StartLine   int               `json:"start_line"`
    EndLine     int               `json:"end_line"`
    Type        string            `json:"type"` // function, class, interface, etc.
    Name        string            `json:"name"`
    Summary     string            `json:"summary"`
    Dependencies []string         `json:"dependencies"`
    Metadata    map[string]interface{} `json:"metadata"`
}

type ArchitectureAnalysis struct {
    Modules      []ModuleInfo      `json:"modules"`
    Dependencies []DependencyInfo  `json:"dependencies"`
    Patterns     []PatternInfo     `json:"patterns"`
    Summary      string           `json:"summary"`
}

type ModuleInfo struct {
    Name         string   `json:"name"`
    Path         string   `json:"path"`
    Purpose      string   `json:"purpose"`
    MainClasses  []string `json:"main_classes"`
    MainFunctions []string `json:"main_functions"`
    Exports      []string `json:"exports"`
}

type DependencyInfo struct {
    From   string `json:"from"`
    To     string `json:"to"`
    Type   string `json:"type"` // import, call, inheritance
    Weight int    `json:"weight"`
}

type PatternInfo struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Files       []string `json:"files"`
    Confidence  float64 `json:"confidence"`
}

func NewCodeAnalyzer(openaiClient *openai.Client, vectorDB *storage.VectorDB) *CodeAnalyzer {
    return &CodeAnalyzer{
        openaiClient: openaiClient,
        vectorDB:     vectorDB,
    }
}

func (ca *CodeAnalyzer) AnalyzeRepository(repoPath string, repoID string) (*ArchitectureAnalysis, error) {
    // 1. 提取代码块
    codeBlocks, err := ca.extractCodeBlocks(repoPath)
    if err != nil {
        return nil, fmt.Errorf("failed to extract code blocks: %w", err)
    }
    
    // 2. 生成代码嵌入向量
    err = ca.generateCodeEmbeddings(codeBlocks, repoID)
    if err != nil {
        return nil, fmt.Errorf("failed to generate embeddings: %w", err)
    }
    
    // 3. 分析架构
    architecture, err := ca.analyzeArchitecture(codeBlocks)
    if err != nil {
        return nil, fmt.Errorf("failed to analyze architecture: %w", err)
    }
    
    return architecture, nil
}

func (ca *CodeAnalyzer) extractCodeBlocks(repoPath string) ([]CodeBlock, error) {
    var codeBlocks []CodeBlock
    
    err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        if !isCodeFile(path) || info.IsDir() {
            return nil
        }
        
        language := detectLanguage(path)
        content, err := ioutil.ReadFile(path)
        if err != nil {
            return err
        }
        
        relPath, _ := filepath.Rel(repoPath, path)
        
        // 根据语言类型解析代码块
        switch language {
        case "Go":
            blocks, err := ca.parseGoFile(string(content), relPath)
            if err == nil {
                codeBlocks = append(codeBlocks, blocks...)
            }
        case "JavaScript", "TypeScript":
            blocks, err := ca.parseJSFile(string(content), relPath, language)
            if err == nil {
                codeBlocks = append(codeBlocks, blocks...)
            }
        default:
            // 对于其他语言，按文件整体处理
            block := CodeBlock{
                FilePath:  relPath,
                Language:  language,
                Content:   string(content),
                StartLine: 1,
                EndLine:   strings.Count(string(content), "\n") + 1,
                Type:      "file",
                Name:      filepath.Base(relPath),
                Metadata:  make(map[string]interface{}),
            }
            codeBlocks = append(codeBlocks, block)
        }
        
        return nil
    })
    
    return codeBlocks, err
}

func (ca *CodeAnalyzer) parseGoFile(content, filePath string) ([]CodeBlock, error) {
    fset := token.NewFileSet()
    file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
    if err != nil {
        return nil, err
    }
    
    var blocks []CodeBlock
    lines := strings.Split(content, "\n")
    
    // 解析函数
    for _, decl := range file.Decls {
        if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.IsExported() {
            start := fset.Position(fn.Pos()).Line
            end := fset.Position(fn.End()).Line
            
            funcContent := strings.Join(lines[start-1:end], "\n")
            
            block := CodeBlock{
                FilePath:  filePath,
                Language:  "Go",
                Content:   funcContent,
                StartLine: start,
                EndLine:   end,
                Type:      "function",
                Name:      fn.Name.Name,
                Metadata: map[string]interface{}{
                    "package":   file.Name.Name,
                    "exported":  fn.Name.IsExported(),
                    "receiver":  ca.extractReceiver(fn),
                },
            }
            
            blocks = append(blocks, block)
        }
    }
    
    return blocks, nil
}

func (ca *CodeAnalyzer) parseJSFile(content, filePath, language string) ([]CodeBlock, error) {
    // 使用正则表达式或简单的解析来提取函数和类
    // 这里简化实现，实际项目中可以使用专门的JS/TS解析器
    
    lines := strings.Split(content, "\n")
    var blocks []CodeBlock
    
    // 简单的函数提取（仅作示例）
    for i, line := range lines {
        if strings.Contains(line, "function ") || strings.Contains(line, "const ") && strings.Contains(line, "=>") {
            // 提取函数名和内容（简化版）
            funcName := ca.extractJSFunctionName(line)
            if funcName != "" {
                block := CodeBlock{
                    FilePath:  filePath,
                    Language:  language,
                    Content:   line, // 简化版，实际应该提取完整函数体
                    StartLine: i + 1,
                    EndLine:   i + 1,
                    Type:      "function",
                    Name:      funcName,
                    Metadata:  make(map[string]interface{}),
                }
                blocks = append(blocks, block)
            }
        }
    }
    
    return blocks, nil
}

func (ca *CodeAnalyzer) generateCodeEmbeddings(codeBlocks []CodeBlock, repoID string) error {
    var vectors []storage.CodeVector
    
    for _, block := range codeBlocks {
        // 生成代码摘要用于embedding
        prompt := fmt.Sprintf(`
Analyze this code block and provide a concise summary of its functionality:

Language: %s
Type: %s
Name: %s

Code:
%s

Provide a brief, technical summary focusing on what this code does.
`, block.Language, block.Type, block.Name, block.Content)

        resp, err := ca.openaiClient.CreateChatCompletion(
            context.Background(),
            openai.ChatCompletionRequest{
                Model: openai.GPT3Dot5Turbo,
                Messages: []openai.ChatCompletionMessage{
                    {Role: openai.ChatMessageRoleUser, Content: prompt},
                },
                MaxTokens: 150,
            },
        )
        
        if err != nil {
            continue // 跳过错误的块
        }
        
        summary := resp.Choices[0].Message.Content
        block.Summary = summary
        
        // 生成embedding向量
        embeddingResp, err := ca.openaiClient.CreateEmbeddings(
            context.Background(),
            openai.EmbeddingRequest{
                Model: openai.AdaEmbeddingV2,
                Input: []string{fmt.Sprintf("%s\n%s\n%s", block.Name, block.Summary, block.Content)},
            },
        )
        
        if err != nil {
            continue
        }
        
        vector := storage.CodeVector{
            ID:     fmt.Sprintf("%s_%s_%d", repoID, block.FilePath, block.StartLine),
            Vector: embeddingResp.Data[0].Embedding,
            Payload: map[string]interface{}{
                "repo_id":    repoID,
                "file_path":  block.FilePath,
                "language":   block.Language,
                "type":       block.Type,
                "name":       block.Name,
                "summary":    block.Summary,
                "content":    block.Content,
                "start_line": block.StartLine,
                "end_line":   block.EndLine,
            },
        }
        
        vectors = append(vectors, vector)
    }
    
    // 批量插入向量数据库
    if len(vectors) > 0 {
        return ca.vectorDB.UpsertVectors(fmt.Sprintf("repo_%s", repoID), vectors)
    }
    
    return nil
}

func (ca *CodeAnalyzer) analyzeArchitecture(codeBlocks []CodeBlock) (*ArchitectureAnalysis, error) {
    // 准备分析数据
    analysisData := make(map[string]interface{})
    analysisData["files"] = len(codeBlocks)
    
    // 统计语言分布
    langCount := make(map[string]int)
    for _, block := range codeBlocks {
        langCount[block.Language]++
    }
    analysisData["languages"] = langCount
    
    // 提取模块信息
    modules := ca.extractModules(codeBlocks)
    
    // 分析依赖关系
    dependencies := ca.analyzeDependencies(codeBlocks)
    
    // 识别设计模式
    patterns := ca.identifyPatterns(codeBlocks)
    
    // 生成架构摘要
    summary, err := ca.generateArchitectureSummary(modules, dependencies, patterns)
    if err != nil {
        summary = "Failed to generate architecture summary"
    }
    
    return &ArchitectureAnalysis{
        Modules:      modules,
        Dependencies: dependencies,
        Patterns:     patterns,
        Summary:      summary,
    }, nil
}

func (ca *CodeAnalyzer) extractModules(codeBlocks []CodeBlock) []ModuleInfo {
    moduleMap := make(map[string]*ModuleInfo)
    
    for _, block := range codeBlocks {
        dir := filepath.Dir(block.FilePath)
        if dir == "." {
            dir = "root"
        }
        
        if module, exists := moduleMap[dir]; exists {
            if block.Type == "function" {
                module.MainFunctions = append(module.MainFunctions, block.Name)
            } else if block.Type == "class" {
                module.MainClasses = append(module.MainClasses, block.Name)
            }
        } else {
            module := &ModuleInfo{
                Name: dir,
                Path: dir,
                MainFunctions: []string{},
                MainClasses:   []string{},
                Exports:       []string{},
            }
            
            if block.Type == "function" {
                module.MainFunctions = append(module.MainFunctions, block.Name)
            } else if block.Type == "class" {
                module.MainClasses = append(module.MainClasses, block.Name)
            }
            
            moduleMap[dir] = module
        }
    }
    
    // 转换为slice
    var modules []ModuleInfo
    for _, module := range moduleMap {
        modules = append(modules, *module)
    }
    
    return modules
}

func (ca *CodeAnalyzer) analyzeDependencies(codeBlocks []CodeBlock) []DependencyInfo {
    var dependencies []DependencyInfo
    
    // 简化的依赖分析
    for _, block := range codeBlocks {
        // 分析导入语句
        imports := ca.extractImports(block.Content, block.Language)
        for _, imp := range imports {
            dep := DependencyInfo{
                From:   block.FilePath,
                To:     imp,
                Type:   "import",
                Weight: 1,
            }
            dependencies = append(dependencies, dep)
        }
    }
    
    return dependencies
}

func (ca *CodeAnalyzer) identifyPatterns(codeBlocks []CodeBlock) []PatternInfo {
    var patterns []PatternInfo
    
    // 识别常见设计模式（简化版）
    singletonFiles := []string{}
    factoryFiles := []string{}
    
    for _, block := range codeBlocks {
        content := strings.ToLower(block.Content)
        if strings.Contains(content, "singleton") {
            singletonFiles = append(singletonFiles, block.FilePath)
        }
        if strings.Contains(content, "factory") || strings.Contains(content, "create") {
            factoryFiles = append(factoryFiles, block.FilePath)
        }
    }
    
    if len(singletonFiles) > 0 {
        patterns = append(patterns, PatternInfo{
            Name:        "Singleton Pattern",
            Description: "Ensures a class has only one instance",
            Files:       singletonFiles,
            Confidence:  0.7,
        })
    }
    
    if len(factoryFiles) > 0 {
        patterns = append(patterns, PatternInfo{
            Name:        "Factory Pattern",
            Description: "Creates objects without specifying exact classes",
            Files:       factoryFiles,
            Confidence:  0.6,
        })
    }
    
    return patterns
}

func (ca *CodeAnalyzer) generateArchitectureSummary(modules []ModuleInfo, dependencies []DependencyInfo, patterns []PatternInfo) (string, error) {
    prompt := fmt.Sprintf(`
Based on the following code analysis, provide a comprehensive architecture summary:

Modules (%d):
%s

Dependencies (%d):
%s

Design Patterns (%d):
%s

Please provide a detailed architecture summary including:
1. Overall system structure
2. Key modules and their responsibilities
3. Main dependencies and data flow
4. Identified design patterns
5. Architectural strengths and potential issues
`, len(modules), ca.formatModules(modules), len(dependencies), ca.formatDependencies(dependencies), len(patterns), ca.formatPatterns(patterns))

    resp, err := ca.openaiClient.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4o,
            Messages: []openai.ChatCompletionMessage{
                {Role: openai.ChatMessageRoleUser, Content: prompt},
            },
            MaxTokens: 1000,
        },
    )
    
    if err != nil {
        return "", err
    }
    
    return resp.Choices[0].Message.Content, nil
}

// 辅助方法
func (ca *CodeAnalyzer) extractReceiver(fn *ast.FuncDecl) string {
    if fn.Recv != nil && len(fn.Recv.List) > 0 {
        if starExpr, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
            if ident, ok := starExpr.X.(*ast.Ident); ok {
                return ident.Name
            }
        } else if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
            return ident.Name
        }
    }
    return ""
}

func (ca *CodeAnalyzer) extractJSFunctionName(line string) string {
    // 简化的JS函数名提取
    if strings.Contains(line, "function ") {
        parts := strings.Split(line, "function ")
        if len(parts) > 1 {
            name := strings.Fields(parts[1])[0]
            return strings.TrimSuffix(name, "(")
        }
    }
    return ""
}

func (ca *CodeAnalyzer) extractImports(content, language string) []string {
    var imports []string
    lines := strings.Split(content, "\n")
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        switch language {
        case "Go":
            if strings.HasPrefix(line, "import ") {
                // 提取Go导入
                if strings.Contains(line, "\"") {
                    start := strings.Index(line, "\"")
                    end := strings.LastIndex(line, "\"")
                    if start != -1 && end != -1 && start < end {
                        imports = append(imports, line[start+1:end])
                    }
                }
            }
        case "JavaScript", "TypeScript":
            if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "const ") && strings.Contains(line, "require(") {
                // 简化的JS/TS导入提取
                imports = append(imports, line)
            }
        }
    }
    
    return imports
}

func (ca *CodeAnalyzer) formatModules(modules []ModuleInfo) string {
    var result strings.Builder
    for _, module := range modules {
        result.WriteString(fmt.Sprintf("- %s: %d functions, %d classes\n", 
            module.Name, len(module.MainFunctions), len(module.MainClasses)))
    }
    return result.String()
}

func (ca *CodeAnalyzer) formatDependencies(deps []DependencyInfo) string {
    var result strings.Builder
    for _, dep := range deps {
        result.WriteString(fmt.Sprintf("- %s -> %s (%s)\n", dep.From, dep.To, dep.Type))
    }
    return result.String()
}

func (ca *CodeAnalyzer) formatPatterns(patterns []PatternInfo) string {
    var result strings.Builder
    for _, pattern := range patterns {
        result.WriteString(fmt.Sprintf("- %s (%.1f confidence): %s\n", 
            pattern.Name, pattern.Confidence, pattern.Description))
    }
    return result.String()
}
```

### 3. 智能问答服务

```go
// internal/services/smart_qa.go
package services

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/sashabaranov/go-openai"
    "deepwiki-backend/internal/storage"
)

type SmartQAService struct {
    openaiClient    *openai.Client
    vectorDB        *storage.VectorDB
    conversationMgr *ConversationManager
}

type ConversationManager struct {
    conversations map[string][]*ConversationMessage
}

type ConversationMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
    Context string `json:"context,omitempty"`
}

type QueryResult struct {
    Answer           string              `json:"answer"`
    RelevantCode     []CodeReference     `json:"relevant_code"`
    Confidence       float64            `json:"confidence"`
    Sources          []string           `json:"sources"`
    SuggestedQueries []string           `json:"suggested_queries"`
}

type CodeReference struct {
    FilePath    string  `json:"file_path"`
    Language    string  `json:"language"`
    Type        string  `json:"type"`
    Name        string  `json:"name"`
    Content     string  `json:"content"`
    StartLine   int     `json:"start_line"`
    EndLine     int     `json:"end_line"`
    Relevance   float64 `json:"relevance"`
}

func NewSmartQAService(openaiClient *openai.Client, vectorDB *storage.VectorDB) *SmartQAService {
    return &SmartQAService{
        openaiClient: openaiClient,
        vectorDB:     vectorDB,
        conversationMgr: &ConversationManager{
            conversations: make(map[string][]*ConversationMessage),
        },
    }
}

func (qa *SmartQAService) QueryRepository(repoID, question, sessionID string) (*QueryResult, error) {
    // 1. 分析问题类型
    questionType, err := qa.analyzeQuestionType(question)
    if err != nil {
        return nil, fmt.Errorf("failed to analyze question: %w", err)
    }
    
    // 2. 生成查询向量
    queryVector, err := qa.generateQueryEmbedding(question)
    if err != nil {
        return nil, fmt.Errorf("failed to generate query embedding: %w", err)
    }
    
    // 3. 搜索相关代码
    relevantCode, err := qa.searchRelevantCode(repoID, queryVector, questionType)
    if err != nil {
        return nil, fmt.Errorf("failed to search relevant code: %w", err)
    }
    
    // 4. 获取对话历史
    conversationHistory := qa.conversationMgr.GetHistory(sessionID)
    
    // 5. 生成答案
    answer, confidence, err := qa.generateAnswer(question, relevantCode, conversationHistory, questionType)
    if err != nil {
        return nil, fmt.Errorf("failed to generate answer: %w", err)
    }
    
    // 6. 生成建议问题
    suggestedQueries := qa.generateSuggestedQueries(question, relevantCode, questionType)
    
    // 7. 保存对话历史
    qa.conversationMgr.AddMessage(sessionID, &ConversationMessage{
        Role:    "user",
        Content: question,
    })
    qa.conversationMgr.AddMessage(sessionID, &ConversationMessage{
        Role:    "assistant",
        Content: answer,
        Context: qa.formatCodeContext(relevantCode),
    })
    
    // 8. 提取源文件
    sources := qa.extractSources(relevantCode)
    
    return &QueryResult{
        Answer:           answer,
        RelevantCode:     relevantCode,
        Confidence:       confidence,
        Sources:          sources,
        SuggestedQueries: suggestedQueries,
    }, nil
}

func (qa *SmartQAService) analyzeQuestionType(question string) (string, error) {
    prompt := fmt.Sprintf(`
Analyze the following question about a codebase and categorize it. 
Return only one of these categories: architecture, implementation, debugging, usage, pattern, dependency, performance, security

Question: %s

Category:`, question)

    resp, err := qa.openaiClient.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT3Dot5Turbo,
            Messages: []openai.ChatCompletionMessage{
                {Role: openai.ChatMessageRoleUser, Content: prompt},
            },
            MaxTokens: 10,
        },
    )
    
    if err != nil {
        return "general", err
    }
    
    return strings.TrimSpace(strings.ToLower(resp.Choices[0].Message.Content)), nil
}

func (qa *SmartQAService) generateQueryEmbedding(question string) ([]float32, error) {
    resp, err := qa.openaiClient.CreateEmbeddings(
        context.Background(),
        openai.EmbeddingRequest{
            Model: openai.AdaEmbeddingV2,
            Input: []string{question},
        },
    )
    
    if err != nil {
        return nil, err
    }
    
    return resp.Data[0].Embedding, nil
}

func (qa *SmartQAService) searchRelevantCode(repoID string, queryVector []float32, questionType string) ([]CodeReference, error) {
    collectionName := fmt.Sprintf("repo_%s", repoID)
    
    // 根据问题类型调整搜索参数
    limit := qa.getSearchLimit(questionType)
    
    searchResults, err := qa.vectorDB.SearchSimilar(collectionName, queryVector, limit)
    if err != nil {
        return nil, err
    }
    
    var codeRefs []CodeReference
    for _, result := range searchResults {
        payload := result.Payload
        
        codeRef := CodeReference{
            FilePath:  payload["file_path"].(string),
            Language:  payload["language"].(string),
            Type:      payload["type"].(string),
            Name:      payload["name"].(string),
            Content:   payload["content"].(string),
            StartLine: int(payload["start_line"].(float64)),
            EndLine:   int(payload["end_line"].(float64)),
            Relevance: float64(result.Score),
        }
        
        codeRefs = append(codeRefs, codeRef)
    }
    
    return codeRefs, nil
}

func (qa *SmartQAService) generateAnswer(question string, relevantCode []CodeReference, 
    history []*ConversationMessage, questionType string) (string, float64, error) {
    
    // 构建上下文
    context := qa.buildContext(relevantCode, questionType)
    
    // 构建对话历史
    conversationContext := qa.buildConversationContext(history)
    
    // 根据问题类型选择专门的提示模板
    prompt := qa.buildPrompt(question, context, conversationContext, questionType)
    
    messages := []openai.ChatCompletionMessage{
        {
            Role:    openai.ChatMessageRoleSystem,
            Content: qa.getSystemPrompt(questionType),
        },
        {
            Role:    openai.ChatMessageRoleUser,
            Content: prompt,
        },
    }
    
    resp, err := qa.openaiClient.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model:       openai.GPT4,
            Messages:    messages,
            MaxTokens:   1500,
            Temperature: 0.1,
        },
    )
    
    if err != nil {
        return "", 0.0, err
    }
    
    answer := resp.Choices[0].Message.Content
    confidence := qa.calculateConfidence(relevantCode, answer)
    
    return answer, confidence, nil
}

func (qa *SmartQAService) getSystemPrompt(questionType string) string {
    basePrompt := `You are an expert code analyst. Analyze the provided code and answer questions accurately and comprehensively.`
    
    switch questionType {
    case "architecture":
        return basePrompt + ` Focus on system design, module relationships, and overall structure.`
    case "implementation":
        return basePrompt + ` Focus on how specific functionality is implemented in the code.`
    case "debugging":
        return basePrompt + ` Focus on identifying potential issues, bugs, or error conditions.`
    case "usage":
        return basePrompt + ` Focus on how to use the code, APIs, or functions properly.`
    case "pattern":
        return basePrompt + ` Focus on design patterns, coding patterns, and best practices.`
    case "dependency":
        return basePrompt + ` Focus on dependencies, imports, and module relationships.`
    case "performance":
        return basePrompt + ` Focus on performance implications, optimizations, and efficiency.`
    case "security":
        return basePrompt + ` Focus on security considerations, vulnerabilities, and best practices.`
    default:
        return basePrompt
    }
}

func (qa *SmartQAService) buildPrompt(question, context, conversationContext, questionType string) string {
    prompt := fmt.Sprintf(`
Previous conversation:
%s

Code context:
%s

Current question: %s

Please provide a detailed, accurate answer based on the code context. Include:
1. Direct answer to the question
2. Relevant code examples and explanations
3. File references and line numbers when applicable
4. Any important considerations or caveats

`, conversationContext, context, question)

    return prompt
}

func (qa *SmartQAService) buildContext(relevantCode []CodeReference, questionType string) string {
    var context strings.Builder
    
    // 根据问题类型排序和过滤代码
    sortedCode := qa.sortCodeByRelevance(relevantCode, questionType)
    
    for i, code := range sortedCode {
        if i >= 10 { // 限制上下文长度
            break
        }
        
        context.WriteString(fmt.Sprintf(`
File: %s (Lines %d-%d)
Type: %s
Name: %s
Language: %s
Relevance: %.2f

%s

---

`, code.FilePath, code.StartLine, code.EndLine, code.Type, code.Name, code.Language, code.Relevance, code.Content))
    }
    
    return context.String()
}

func (qa *SmartQAService) buildConversationContext(history []*ConversationMessage) string {
    if len(history) == 0 {
        return "No previous conversation."
    }
    
    var context strings.Builder
    
    // 只包含最近的几轮对话
    start := 0
    if len(history) > 6 {
        start = len(history) - 6
    }
    
    for i := start; i < len(history); i++ {
        msg := history[i]
        context.WriteString(fmt.Sprintf("%s: %s\n", strings.Title(msg.Role), msg.Content))
    }
    
    return context.String()
}

func (qa *SmartQAService) sortCodeByRelevance(code []CodeReference, questionType string) []CodeReference {
    // 根据问题类型和相关性分数排序
    sorted := make([]CodeReference, len(code))
    copy(sorted, code)
    
    // 简化的排序逻辑
    for i := 0; i < len(sorted)-1; i++ {
        for j := i + 1; j < len(sorted); j++ {
            if qa.calculateRelevanceScore(sorted[i], questionType) < qa.calculateRelevanceScore(sorted[j], questionType) {
                sorted[i], sorted[j] = sorted[j], sorted[i]
            }
        }
    }
    
    return sorted
}

func (qa *SmartQAService) calculateRelevanceScore(code CodeReference, questionType string) float64 {
    score := code.Relevance
    
    // 根据问题类型调整分数
    switch questionType {
    case "architecture":
        if code.Type == "class" || code.Type == "interface" {
            score *= 1.2
        }
    case "implementation":
        if code.Type == "function" {
            score *= 1.2
        }
    case "pattern":
        if strings.Contains(strings.ToLower(code.Name), "factory") ||
           strings.Contains(strings.ToLower(code.Name), "builder") ||
           strings.Contains(strings.ToLower(code.Name), "singleton") {
            score *= 1.3
        }
    }
    
    return score
}

func (qa *SmartQAService) getSearchLimit(questionType string) int {
    switch questionType {
    case "architecture":
        return 15
    case "implementation":
        return 10
    case "debugging":
        return 8
    default:
        return 10
    }
}

func (qa *SmartQAService) calculateConfidence(relevantCode []CodeReference, answer string) float64 {
    if len(relevantCode) == 0 {
        return 0.1
    }
    
    // 基于相关代码的数量和质量计算置信度
    avgRelevance := 0.0
    for _, code := range relevantCode {
        avgRelevance += code.Relevance
    }
    avgRelevance /= float64(len(relevantCode))
    
    // 基于答案长度和结构调整置信度
    answerQuality := 0.7
    if len(answer) > 200 && strings.Contains(answer, "```") {
        answerQuality = 0.9
    }
    
    return avgRelevance * answerQuality
}

func (qa *SmartQAService) generateSuggestedQueries(question string, relevantCode []CodeReference, questionType string) []string {
    // 基于当前问题和相关代码生成建议问题
    var suggestions []string
    
    // 基于问题类型的通用建议
    switch questionType {
    case "architecture":
        suggestions = append(suggestions, 
            "What are the main design patterns used in this codebase?",
            "How are the modules organized and connected?",
            "What are the key interfaces and abstractions?")
    case "implementation":
        suggestions = append(suggestions,
            "How is error handling implemented?",
            "What are the main data structures used?",
            "How is concurrency handled?")
    case "usage":
        suggestions = append(suggestions,
            "What are the main entry points?",
            "How do I configure this application?",
            "What are the common use cases?")
    }
    
    // 基于相关代码生成特定建议
    for _, code := range relevantCode[:min(3, len(relevantCode))] {
        if code.Type == "function" {
            suggestions = append(suggestions, fmt.Sprintf("How does the %s function work?", code.Name))
        } else if code.Type == "class" {
            suggestions = append(suggestions, fmt.Sprintf("What is the purpose of the %s class?", code.Name))
        }
    }
    
    return suggestions
}

func (qa *SmartQAService) formatCodeContext(relevantCode []CodeReference) string {
    var context strings.Builder
    for _, code := range relevantCode {
        context.WriteString(fmt.Sprintf("%s:%d-%d ", code.FilePath, code.StartLine, code.EndLine))
    }
    return context.String()
}

func (qa *SmartQAService) extractSources(relevantCode []CodeReference) []string {
    sourceMap := make(map[string]bool)
    var sources []string
    
    for _, code := range relevantCode {
        if !sourceMap[code.FilePath] {
            sources = append(sources, code.FilePath)
            sourceMap[code.FilePath] = true
        }
    }
    
    return sources
}

// ConversationManager 方法
func (cm *ConversationManager) GetHistory(sessionID string) []*ConversationMessage {
    return cm.conversations[sessionID]
}

func (cm *ConversationManager) AddMessage(sessionID string, message *ConversationMessage) {
    if cm.conversations[sessionID] == nil {
        cm.conversations[sessionID] = []*ConversationMessage{}
    }
    
    cm.conversations[sessionID] = append(cm.conversations[sessionID], message)
    
    // 限制历史长度
    if len(cm.conversations[sessionID]) > 20 {
        cm.conversations[sessionID] = cm.conversations[sessionID][2:]
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

### 4. 更新的仓库服务

```go
// internal/services/repository.go (更新版本)
func (s *RepositoryService) processRepository(repo *models.Repository) {
    // 克隆仓库
    repoPath, err := s.github.CloneRepository(repo.URL)
    if err != nil {
        s.updateRepositoryStatus(repo.ID, "failed")
        return
    }
    defer os.RemoveAll(repoPath) // 清理临时文件
    
    // 使用增强的代码分析器
    codeAnalyzer := NewCodeAnalyzer(s.openaiClient, s.vectorDB)
    
    // 分析仓库架构
    architecture, err := codeAnalyzer.AnalyzeRepository(repoPath, repo.ID.Hex())
    if err != nil {
        s.updateRepositoryStatus(repo.ID, "failed")
        return
    }
    
    // 更新仓库信息
    s.updateRepositoryWithArchitecture(repo.ID, architecture)
}

func (s *RepositoryService) QueryRepository(repoID, question, sessionID string) (*SmartQAResult, error) {
    return s.smartQA.QueryRepository(repoID, question, sessionID)
}
```

### 5. 优化的API处理器

```go
// internal/api/handlers/repository.go (新增方法)
type SmartQueryRequest struct {
    Question  string `json:"question" binding:"required"`
    SessionID string `json:"session_id"`
}

func (h *RepositoryHandler) SmartQuery(c *gin.Context) {
    repoID := c.Param("id")
    
    var req SmartQueryRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if req.SessionID == "" {
        req.SessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
    }
    
    result, err := h.services.Repository.SmartQuery(repoID, req.Question, req.SessionID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, result)
}
```

### 6. 前端优化 - 智能问答组件

```jsx
// src/components/SmartCodeQuery.js
import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import { repositoryAPI } from '../services/api';

const QueryContainer = styled.div`
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  margin-bottom: 2rem;
`;

const Header = styled.div`
  padding: 1.5rem;
  border-bottom: 1px solid #eee;
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

const Title = styled.h3`
  margin: 0;
  color: #2c3e50;
`;

const ConfidenceIndicator = styled.div`
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.9rem;
  color: #666;
`;

const ConfidenceBar = styled.div`
  width: 60px;
  height: 8px;
  background: #eee;
  border-radius: 4px;
  overflow: hidden;
`;

const ConfidenceFill = styled.div`
  height: 100%;
  background: ${props => 
    props.confidence > 0.8 ? '#2ecc71' :
    props.confidence > 0.6 ? '#f39c12' : '#e74c3c'
  };
  width: ${props => props.confidence * 100}%;
  transition: width 0.3s ease;
`;

const ChatContainer = styled.div`
  max-height: 600px;
  overflow-y: auto;
  padding: 1rem;
`;

const Message = styled.div`
  margin-bottom: 1.5rem;
  ${props => props.isUser ? `
    text-align: right;
  ` : ''}
`;

const MessageBubble = styled.div`
  display: inline-block;
  max-width: 80%;
  padding: 1rem;
  border-radius: 12px;
  ${props => props.isUser ? `
    background: #3498db;
    color: white;
    text-align: left;
  ` : `
    background: #f8f9fa;
    color: #333;
    border: 1px solid #e9ecef;
  `}
`;

const CodeBlock = styled.pre`
  background: #282c34;
  color: #abb2bf;
  padding: 1rem;
  border-radius: 4px;
  overflow-x: auto;
  margin: 0.5rem 0;
  font-size: 0.9rem;
`;

const SourceList = styled.div`
  margin-top: 1rem;
  padding: 1rem;
  background: #f8f9fa;
  border-radius: 4px;
`;

const SourceItem = styled.div`
  padding: 0.25rem 0;
  font-size: 0.9rem;
  color: #666;
`;

const SuggestedQueries = styled.div`
  margin-top: 1rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
`;

const SuggestionChip = styled.button`
  padding: 0.5rem 1rem;
  background: #e9ecef;
  border: none;
  border-radius: 20px;
  font-size: 0.9rem;
  cursor: pointer;
  transition: background 0.2s;
  
  &:hover {
    background: #dee2e6;
  }
`;

const InputContainer = styled.div`
  padding: 1rem;
  border-top: 1px solid #eee;
  display: flex;
  gap: 1rem;
`;

const QueryInput = styled.textarea`
  flex: 1;
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  resize: vertical;
  min-height: 60px;
  font-family: inherit;
  
  &:focus {
    outline: none;
    border-color: #3498db;
  }
`;

const SendButton = styled.button`
  padding: 0.75rem 1.5rem;
  background: #3498db;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  height: fit-content;
  
  &:hover {
    background: #2980b9;
  }
  
  &:disabled {
    background: #bdc3c7;
    cursor: not-allowed;
  }
`;

function SmartCodeQuery({ repositoryId }) {
  const [messages, setMessages] = useState([]);
  const [currentQuestion, setCurrentQuestion] = useState('');
  const [loading, setLoading] = useState(false);
  const [sessionId, setSessionId] = useState('');
  const [lastResult, setLastResult] = useState(null);

  useEffect(() => {
    // 生成会话ID
    setSessionId(`session_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`);
  }, []);

  const formatMessage = (content) => {
    // 简单的markdown-like格式化
    const lines = content.split('\n');
    const formatted = [];
    let inCodeBlock = false;
    let codeLines = [];

    for (let line of lines) {
      if (line.startsWith('```')) {
        if (inCodeBlock) {
          formatted.push(
            <CodeBlock key={formatted.length}>
              {codeLines.join('\n')}
            </CodeBlock>
          );
          codeLines = [];
        }
        inCodeBlock = !inCodeBlock;
      } else if (inCodeBlock) {
        codeLines.push(line);
      } else {
        formatted.push(<div key={formatted.length}>{line}</div>);
      }
    }

    return formatted;
  };

  const handleSubmit = async (question = currentQuestion) => {
    if (!question.trim() || loading) return;

    const userMessage = {
      id: Date.now(),
      isUser: true,
      content: question,
      timestamp: new Date()
    };

    setMessages(prev => [...prev, userMessage]);
    setCurrentQuestion('');
    setLoading(true);

    try {
      const response = await repositoryAPI.smartQuery(repositoryId, question, sessionId);
      const result = response.data;
      
      const aiMessage = {
        id: Date.now() + 1,
        isUser: false,
        content: result.answer,
        sources: result.sources,
        relevantCode: result.relevant_code,
        confidence: result.confidence,
        suggestedQueries: result.suggested_queries,
        timestamp: new Date()
      };

      setMessages(prev => [...prev, aiMessage]);
      setLastResult(result);
    } catch (error) {
      const errorMessage = {
        id: Date.now() + 1,
        isUser: false,
        content: '抱歉，查询失败。请重试。',
        timestamp: new Date()
      };
      setMessages(prev => [...prev, errorMessage]);
    } finally {
      setLoading(false);
    }
  };

  const handleSuggestionClick = (suggestion) => {
    handleSubmit(suggestion);
  };

  const handleKeyPress = (e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      handleSubmit();
    }
  };

  return (
    <QueryContainer>
      <Header>
        <Title>智能代码问答</Title>
        {lastResult && (
          <ConfidenceIndicator>
            置信度: {Math.round(lastResult.confidence * 100)}%
            <ConfidenceBar>
              <ConfidenceFill confidence={lastResult.confidence} />
            </ConfidenceBar>
          </ConfidenceIndicator>
        )}
      </Header>

      <ChatContainer>
        {messages.length === 0 && (
          <div style={{ textAlign: 'center', color: '#666', padding: '2rem' }}>
            <p>开始询问关于这个代码仓库的问题吧！</p>
            <p>例如："这个项目的架构是怎样的？" 或 "主要的类和函数有哪些？"</p>
          </div>
        )}

        {messages.map(message => (
          <Message key={message.id} isUser={message.isUser}>
            <MessageBubble isUser={message.isUser}>
              {formatMessage(message.content)}
              
              {message.sources && message.sources.length > 0 && (
                <SourceList>
                  <strong>相关文件：</strong>
                  {message.sources.map((source, index) => (
                    <SourceItem key={index}>{source}</SourceItem>
                  ))}
                </SourceList>
              )}
              
              {message.suggestedQueries && message.suggestedQueries.length > 0 && (
                <>
                  <div style={{ marginTop: '1rem', fontSize: '0.9rem', color: '#666' }}>
                    <strong>建议的问题：</strong>
                  </div>
                  <SuggestedQueries>
                    {message.suggestedQueries.slice(0, 3).map((query, index) => (
                      <SuggestionChip 
                        key={index}
                        onClick={() => handleSuggestionClick(query)}
                      >
                        {query}
                      </SuggestionChip>
                    ))}
                  </SuggestedQueries>
                </>
              )}
            </MessageBubble>
          </Message>
        ))}

        {loading && (
          <Message>
            <MessageBubble>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <div className="loading-spinner" />
                正在分析代码并生成回答...
              </div>
            </MessageBubble>
          </Message>
        )}
      </ChatContainer>

      <InputContainer>
        <QueryInput
          value={currentQuestion}
          onChange={(e) => setCurrentQuestion(e.target.value)}
          onKeyPress={handleKeyPress}
          placeholder="询问关于代码的问题... (Ctrl+Enter 发送)"
          disabled={loading}
        />
        <SendButton 
          onClick={() => handleSubmit()}
          disabled={loading || !currentQuestion.trim()}
        >
          {loading ? '分析中...' : '发送'}
        </SendButton>
      </InputContainer>
    </QueryContainer>
  );
}

export default SmartCodeQuery;
```

### 7. API服务更新

```javascript
// src/services/api.js (添加新方法)
export const repositoryAPI = {
  // ... 现有方法

  // 智能查询
  smartQuery: (id, question, sessionId) => 
    api.post(`/repositories/${id}/smart-query`, { 
      question, 
      session_id: sessionId 
    }),
  
  // 获取架构分析
  getArchitecture: (id) => 
    api.get(`/repositories/${id}/architecture`),
};
```

### 8. Docker配置

```dockerfile
# docker-compose.yml
version: '3.8'

services:
  mongodb:
    image: mongo:6.0
    ports:
      - "27017:27017"
    environment:
      MONGO_INITDB_DATABASE: deepwiki
    volumes:
      - mongodb_data:/data/db

  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
    volumes:
      - qdrant_data:/qdrant/storage

  backend:
    build: ./deepwiki-backend
    ports:
      - "8080:8080"
    environment:
      - MONGO_URL=mongodb://mongodb:27017/deepwiki
      - QDRANT_URL=http://qdrant:6333
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - GITHUB_TOKEN=${GITHUB_TOKEN}
    depends_on:
      - mongodb
      - qdrant

  frontend:
    build: ./deepwiki-frontend
    ports:
      - "3000:3000"
    environment:
      - REACT_APP_API_URL=http://localhost:8080/api/v1
    depends_on:
      - backend

volumes:
  mongodb_data:
  qdrant_data:
```

这个优化版本提供了：

**AI功能增强：**
- 向量数据库支持语义搜索
- 多层次代码分析（AST、架构、模式）
- 智能问答系统，支持多轮对话
- 上下文感知的回答生成
- 问题类型分析和专门处理

**更好的用户体验：**
- 置信度指示器
- 代码引用和源文件链接
- 建议问题生成
- 聊天式交互界面
- 实时反馈

**性能优化：**
- 向量化存储和检索
- 智能缓存策略
- 异步处理
- 批量操作


