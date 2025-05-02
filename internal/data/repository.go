// internal/data/repository.go
package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deepwiki-go/internal/config"
	"github.com/deepwiki-go/pkg/utils"
)

// RepositoryManager 处理代码仓库的克隆和分析
type RepositoryManager struct {
	config   *config.Config
	basePath string
}

// NewRepositoryManager 创建新的仓库管理器
func NewRepositoryManager(cfg *config.Config) *RepositoryManager {
	basePath := utils.GetDefaultRootPath()
	return &RepositoryManager{
		config:   cfg,
		basePath: basePath,
	}
}

// CloneRepository 克隆GitHub或GitLab仓库到本地
func (r *RepositoryManager) CloneRepository(repoURL, accessToken string) (string, error) {
	if repoURL == "" {
		return "", errors.New("仓库URL不能为空")
	}

	// 生成本地路径
	repoDir := createRepoDirName(repoURL)
	localPath := filepath.Join(r.basePath, "repos", repoDir)

	// 检查仓库是否已经克隆
	if _, err := os.Stat(localPath); err == nil {
		// 仓库已存在，可以选择更新或使用现有版本
		return localPath, nil
	}

	// 克隆仓库
	if err := utils.DownloadRepo(repoURL, localPath, accessToken); err != nil {
		return "", fmt.Errorf("克隆仓库失败: %v", err)
	}

	return localPath, nil
}

// GetRepositoryFiles 获取仓库中的所有文件
func (r *RepositoryManager) GetRepositoryFiles(repoPath string) ([]string, error) {
	var allFiles []string

	// 遍历仓库目录
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			// 检查是否应该排除此目录
			dirName := filepath.Base(path)
			for _, excludedDir := range r.config.FileFilters.ExcludedDirs {
				if dirName == excludedDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// 跳过排除的文件
		fileName := filepath.Base(path)
		for _, excludedFile := range r.config.FileFilters.ExcludedFiles {
			if fileName == excludedFile {
				return nil
			}
		}

		// 添加文件到列表
		allFiles = append(allFiles, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return allFiles, nil
}

// AnalyzeRepository 分析仓库结构并返回结构摘要
func (r *RepositoryManager) AnalyzeRepository(repoPath string) (map[string]interface{}, error) {
	files, err := r.GetRepositoryFiles(repoPath)
	if err != nil {
		return nil, err
	}

	// 创建仓库结构Map
	result := make(map[string]interface{})
	fileCount := 0
	extCounts := make(map[string]int)

	// 分析文件
	for _, file := range files {
		fileCount++

		// 获取文件扩展名并统计
		ext := filepath.Ext(file)
		if ext != "" {
			extCounts[ext]++
		}

		// 对路径进行相对化处理
		relPath, err := filepath.Rel(repoPath, file)
		if err == nil {
			// 按目录结构组织文件
			parts := strings.Split(relPath, string(filepath.Separator))
			addToStructure(result, parts)
		}
	}

	// 生成摘要
	summary := map[string]interface{}{
		"file_count": fileCount,
		"extensions": extCounts,
		"structure":  result,
	}

	return summary, nil
}

// 辅助函数: 创建仓库目录名
func createRepoDirName(repoURL string) string {
	// 移除协议前缀
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")

	// 移除.git后缀
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// 替换非法文件名字符
	repoURL = strings.ReplaceAll(repoURL, "/", "_")
	repoURL = strings.ReplaceAll(repoURL, "\\", "_")
	return repoURL
}

// 辅助函数: 按层级将文件添加到结构中
func addToStructure(structure map[string]interface{}, pathParts []string) {
	if len(pathParts) == 0 {
		return
	}

	current := pathParts[0]

	if len(pathParts) == 1 {
		// 叶子节点 (文件)
		structure[current] = nil
		return
	}

	// 目录节点
	if _, exists := structure[current]; !exists {
		structure[current] = make(map[string]interface{})
	}

	// 如果是目录，则递归添加子目录/文件
	if subDir, ok := structure[current].(map[string]interface{}); ok {
		addToStructure(subDir, pathParts[1:])
	}
}
