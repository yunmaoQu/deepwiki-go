// internal/data/database.go
package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deepwiki-go/internal/models"
	"github.com/deepwiki-go/pkg/utils"
)

// DatabaseManager 管理文档数据库
type DatabaseManager struct {
	db            map[string]models.Document
	repoURLOrPath string
	repoPaths     map[string]string
}

// NewDatabaseManager 创建一个新的数据库管理器
func NewDatabaseManager() *DatabaseManager {
	return &DatabaseManager{
		db:        make(map[string]models.Document),
		repoPaths: make(map[string]string),
	}
}

// PrepareDatabase 从仓库创建一个新的数据库
func (dm *DatabaseManager) PrepareDatabase(repoURLOrPath string, accessToken string) ([]models.Document, error) {
	dm.resetDatabase()
	if err := dm.createRepo(repoURLOrPath, accessToken); err != nil {
		return nil, err
	}
	return dm.prepareDBIndex()
}

// resetDatabase 将数据库重置为初始状态
func (dm *DatabaseManager) resetDatabase() {
	dm.db = make(map[string]models.Document)
	dm.repoURLOrPath = ""
	dm.repoPaths = make(map[string]string)
}

// createRepo 下载并准备所有路径
func (dm *DatabaseManager) createRepo(repoURLOrPath string, accessToken string) error {
	log.Printf("为 %s 准备仓库存储...", repoURLOrPath)

	// 获取根路径
	rootPath := utils.GetDefaultRootPath()

	os.MkdirAll(rootPath, 0755)

	// 处理URL或本地路径
	if strings.HasPrefix(repoURLOrPath, "https://") || strings.HasPrefix(repoURLOrPath, "http://") {
		// 根据 URL 格式提取仓库名称
		var repoName string

		if strings.Contains(repoURLOrPath, "github.com") {
			// GitHub URL 格式: https://github.com/owner/repo
			repoName = strings.Split(repoURLOrPath, "/")[len(strings.Split(repoURLOrPath, "/"))-1]
			repoName = strings.TrimSuffix(repoName, ".git")
		} else if strings.Contains(repoURLOrPath, "gitlab.com") {
			// GitLab URL 格式: https://gitlab.com/owner/repo 或 https://gitlab.com/group/subgroup/repo
			repoName = strings.Split(repoURLOrPath, "/")[len(strings.Split(repoURLOrPath, "/"))-1]
			repoName = strings.TrimSuffix(repoName, ".git")
		} else {
			// 通用处理其他 Git URL
			repoName = strings.Split(repoURLOrPath, "/")[len(strings.Split(repoURLOrPath, "/"))-1]
			repoName = strings.TrimSuffix(repoName, ".git")
		}

		saveRepoDir := filepath.Join(rootPath, "repos", repoName)

		// 检查仓库目录是否已存在且非空
		if !dm.directoryExistsAndNotEmpty(saveRepoDir) {
			// 仅当仓库不存在或为空时下载
			if err := utils.DownloadRepo(repoURLOrPath, saveRepoDir, accessToken); err != nil {
				return err
			}
		} else {
			log.Printf("仓库已存在于 %s。使用现有仓库。", saveRepoDir)
		}

		dm.repoPaths = map[string]string{
			"save_repo_dir": saveRepoDir,
			"save_db_file":  filepath.Join(rootPath, "databases", fmt.Sprintf("%s.json", repoName)),
		}
	} else { // 本地路径
		repoName := filepath.Base(repoURLOrPath)
		saveRepoDir := repoURLOrPath

		dm.repoPaths = map[string]string{
			"save_repo_dir": saveRepoDir,
			"save_db_file":  filepath.Join(rootPath, "databases", fmt.Sprintf("%s.json", repoName)),
		}
	}

	dm.repoURLOrPath = repoURLOrPath
	os.MkdirAll(dm.repoPaths["save_repo_dir"], 0755)
	os.MkdirAll(filepath.Dir(dm.repoPaths["save_db_file"]), 0755)

	log.Printf("仓库路径: %v", dm.repoPaths)
	return nil
}

// directoryExistsAndNotEmpty 检查目录是否存在且非空
func (dm *DatabaseManager) directoryExistsAndNotEmpty(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return false
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	return len(entries) > 0
}

// prepareDBIndex 为仓库准备索引数据库
func (dm *DatabaseManager) prepareDBIndex() ([]models.Document, error) {
	// 检查数据库
	if dm.repoPaths != nil && dm.fileExists(dm.repoPaths["save_db_file"]) {
		log.Println("加载现有数据库...")

		documents, err := dm.loadDocumentsFromFile(dm.repoPaths["save_db_file"])
		if err == nil && len(documents) > 0 {
			log.Printf("从现有数据库加载了 %d 个文档", len(documents))
			return documents, nil
		}

		log.Printf("加载现有数据库出错: %v", err)
		// 继续创建新数据库
	}

	// 准备数据库
	log.Println("创建新数据库...")
	documents, err := dm.readAllDocuments(dm.repoPaths["save_repo_dir"])
	if err != nil {
		return nil, err
	}

	// 处理文档 (分割和嵌入)
	processedDocs, err := dm.processDocuments(documents)
	if err != nil {
		return nil, err
	}

	// 保存到磁盘
	if err := dm.saveDocumentsToFile(processedDocs, dm.repoPaths["save_db_file"]); err != nil {
		log.Printf("保存数据库出错: %v", err)
	}

	log.Printf("总文档数: %d", len(processedDocs))
	return processedDocs, nil
}

// fileExists 检查文件是否存在
func (dm *DatabaseManager) fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// readAllDocuments 递归读取目录中的所有文档
func (dm *DatabaseManager) readAllDocuments(path string) ([]models.Document, error) {
	var documents []models.Document

	// 要查找的文件扩展名，优先考虑代码文件
	codeExtensions := []string{".py", ".js", ".ts", ".java", ".cpp", ".c", ".go", ".rs",
		".jsx", ".tsx", ".html", ".css", ".php", ".swift", ".cs"}
	docExtensions := []string{".md", ".txt", ".rst", ".json", ".yaml", ".yml"}

	// 获取排除的文件和目录
	excludedDirs := []string{".venv", "node_modules", ".git", "__pycache__"}
	excludedFiles := []string{"package-lock.json", "yarn.lock"}

	log.Printf("从 %s 读取文档", path)

	// 处理代码文件
	for _, ext := range codeExtensions {
		files, err := utils.FindFiles(path, ext)
		if err != nil {
			continue
		}

		for _, filePath := range files {
			// 跳过排除的目录和文件
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
				log.Printf("读取 %s 出错: %v", filePath, err)
				continue
			}

			relativePath, err := filepath.Rel(path, filePath)
			if err != nil {
				log.Printf("获取相对路径出错: %v", err)
				continue
			}

			// 确定这是否是实现文件
			isImplementation := !strings.HasPrefix(filepath.Base(relativePath), "test_") &&
				!strings.HasPrefix(filepath.Base(relativePath), "app_") &&
				!strings.Contains(strings.ToLower(relativePath), "test")

			// 检查 token 数量
			tokenCount := utils.CountTokens(string(content))
			if tokenCount > 8192 { // 最大嵌入 token 限制
				log.Printf("跳过大文件 %s: Token 数量 (%d) 超过限制", relativePath, tokenCount)
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

	// 处理文档文件
	for _, ext := range docExtensions {
		files, err := utils.FindFiles(path, ext)
		if err != nil {
			continue
		}

		for _, filePath := range files {
			// 跳过排除的目录和文件
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
				log.Printf("读取 %s 出错: %v", filePath, err)
				continue
			}

			relativePath, err := filepath.Rel(path, filePath)
			if err != nil {
				log.Printf("获取相对路径出错: %v", err)
				continue
			}

			// 检查 token 数量
			tokenCount := utils.CountTokens(string(content))
			if tokenCount > 8192 { // 最大嵌入 token 限制
				log.Printf("跳过大文件 %s: Token 数量 (%d) 超过限制", relativePath, tokenCount)
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

	log.Printf("找到 %d 个文档", len(documents))
	return documents, nil
}

// processDocuments 处理文档（分割和嵌入）
func (dm *DatabaseManager) processDocuments(documents []models.Document) ([]models.Document, error) {
	// 这里应该实现文本分割和嵌入
	// 简化起见，我们假设已经处理好了文档
	return documents, nil
}

// saveDocumentsToFile 将文档保存到文件
func (dm *DatabaseManager) saveDocumentsToFile(documents []models.Document, filePath string) error {
	data, err := json.MarshalIndent(documents, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// loadDocumentsFromFile 从文件加载文档
func (dm *DatabaseManager) loadDocumentsFromFile(filePath string) ([]models.Document, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var documents []models.Document
	if err := json.Unmarshal(data, &documents); err != nil {
		return nil, err
	}

	return documents, nil
}

// DocScore 存储文档及其相关性分数
type DocScore struct {
	Doc   models.Document
	Score float64
}

// SearchDocuments 搜索与查询相关的文档
func (dm *DatabaseManager) SearchDocuments(query string, topK int) ([]models.Document, error) {
	// 加载数据库
	if len(dm.db) == 0 {
		if dm.repoPaths != nil && dm.fileExists(dm.repoPaths["save_db_file"]) {
			docs, err := dm.loadDocumentsFromFile(dm.repoPaths["save_db_file"])
			if err != nil {
				return nil, err
			}
			for i, doc := range docs {
				dm.db[fmt.Sprintf("doc_%d", i)] = doc
			}
		} else {
			return nil, errors.New("数据库为空，无法执行搜索")
		}
	}

	// 使用改进的相似度搜索算法
	scored := dm.scoreDocumentsForQuery(query)

	// 按分数排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// 返回前 topK 个文档
	var result []models.Document
	for i := 0; i < len(scored) && i < topK; i++ {
		result = append(result, scored[i].Doc)
	}

	return result, nil
}

// scoreDocumentsForQuery 为查询对文档进行评分
func (dm *DatabaseManager) scoreDocumentsForQuery(query string) []DocScore {
	var scored []DocScore
	queryLower := strings.ToLower(query)
	queryWords := tokenizeText(queryLower)

	// 文档频率计算
	docFreq := make(map[string]int)
	for _, doc := range dm.db {
		docWords := tokenizeText(strings.ToLower(doc.Text))
		seenWords := make(map[string]bool)

		for word := range docWords {
			if !seenWords[word] {
				docFreq[word]++
				seenWords[word] = true
			}
		}
	}

	totalDocs := float64(len(dm.db))

	// 对每个文档打分
	for _, doc := range dm.db {
		// 基本分数由以下因素决定：
		// 1. TF-IDF 匹配分数
		// 2. 标题匹配分数
		// 3. 重要性修正

		// 计算 TF-IDF 分数
		textLower := strings.ToLower(doc.Text)
		docWords := tokenizeText(textLower)
		titleWords := tokenizeText(strings.ToLower(doc.Title))

		var tfidfScore float64
		var titleMatchScore float64

		// TF-IDF 计算
		for qWord := range queryWords {
			if len(qWord) <= 1 { // 忽略太短的单词
				continue
			}

			// 计算词频 (TF)
			tf := float64(docWords[qWord]) / float64(len(textLower))

			// 计算逆文档频率 (IDF)
			var idf float64
			if df, ok := docFreq[qWord]; ok && df > 0 {
				idf = math.Log(totalDocs / float64(df))
			}

			// TF-IDF 分数
			tfidfScore += tf * idf

			// 标题匹配分数 (更高权重)
			if titleWords[qWord] > 0 {
				titleMatchScore += 2.0 * float64(titleWords[qWord]) / float64(len(doc.Title))
			}
		}

		// 基础相似度分数
		score := tfidfScore + titleMatchScore

		// 考虑文档重要性
		if doc.Importance == "high" {
			score *= 1.5
		} else if doc.Importance == "medium" {
			score *= 1.2
		}

		// 额外考虑完整短语匹配
		if strings.Contains(textLower, queryLower) {
			score += 5.0 // 完整短语匹配奖励
		}

		// 标题完整匹配额外奖励
		if strings.Contains(strings.ToLower(doc.Title), queryLower) {
			score += 10.0
		}

		if score > 0 {
			scored = append(scored, DocScore{Doc: doc, Score: score})
		}
	}

	return scored
}

// tokenizeText 将文本分词并计算词频
func tokenizeText(text string) map[string]int {
	words := strings.Fields(text)
	wordFreq := make(map[string]int)

	// 停用词列表
	stopWords := map[string]bool{
		"的": true, "了": true, "和": true, "是": true, "在": true,
		"这": true, "有": true, "我": true, "们": true, "为": true,
		"the": true, "a": true, "an": true, "in": true, "on": true,
		"at": true, "to": true, "for": true, "with": true, "by": true,
		"of": true, "and": true, "or": true, "is": true, "are": true,
	}

	// 计算词频
	for _, word := range words {
		// 清理词汇
		word = strings.ToLower(strings.Trim(word, ",.!?;:\"'()[]{}"))
		if word != "" && !stopWords[word] {
			wordFreq[word]++
		}
	}

	return wordFreq
}

// AddDocument 添加文档到数据库
func (dm *DatabaseManager) AddDocument(doc *models.Document) error {
	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc_%d", len(dm.db))
	}

	dm.db[doc.ID] = *doc

	// 保存更新后的数据库到文件
	documents := make([]models.Document, 0, len(dm.db))
	for _, d := range dm.db {
		documents = append(documents, d)
	}

	return dm.saveDocumentsToFile(documents, dm.repoPaths["save_db_file"])
}

// GetDocument 获取文档
func (dm *DatabaseManager) GetDocument(id string) (*models.Document, error) {
	doc, exists := dm.db[id]
	if !exists {
		return nil, fmt.Errorf("文档 ID %s 不存在", id)
	}
	return &doc, nil
}

// DeleteDocument 删除文档
func (dm *DatabaseManager) DeleteDocument(id string) error {
	if _, exists := dm.db[id]; !exists {
		return fmt.Errorf("文档 ID %s 不存在", id)
	}

	delete(dm.db, id)

	// 保存更新后的数据库到文件
	documents := make([]models.Document, 0, len(dm.db))
	for _, d := range dm.db {
		documents = append(documents, d)
	}

	return dm.saveDocumentsToFile(documents, dm.repoPaths["save_db_file"])
}
