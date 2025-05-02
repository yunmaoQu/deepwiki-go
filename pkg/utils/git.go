// pkg/utils/git.go
package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DownloadRepo 将 Git 仓库（GitHub 或 GitLab）下载到指定的本地路径
func DownloadRepo(repoURL string, localPath string, accessToken string) error {
	// 检查 Git 是否已安装
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("Git 未安装: %v", err)
	}

	// 确保本地路径存在
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 准备带访问令牌的克隆 URL（如果提供）
	cloneURL := repoURL
	if accessToken != "" {
		// 根据仓库类型格式化 URL
		if strings.Contains(repoURL, "github.com") {
			// 格式: https://{token}@github.com/owner/repo.git
			cloneURL = strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", accessToken), 1)
		} else if strings.Contains(repoURL, "gitlab.com") {
			// 格式: https://oauth2:{token}@gitlab.com/owner/repo.git
			cloneURL = strings.Replace(repoURL, "https://", fmt.Sprintf("https://oauth2:%s@", accessToken), 1)
		}
	}

	// 克隆仓库
	cmd := exec.Command("git", "clone", cloneURL, localPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 清理错误消息中的任何令牌
		outputStr := string(output)
		if accessToken != "" && strings.Contains(outputStr, accessToken) {
			outputStr = strings.ReplaceAll(outputStr, accessToken, "***TOKEN***")
		}
		return fmt.Errorf("克隆期间出错: %s", outputStr)
	}

	return nil
}

// FindFiles 递归查找具有指定扩展名的所有文件
func FindFiles(root string, ext string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ext) {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// GetDefaultRootPath 获取默认的根路径
func GetDefaultRootPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 如果无法获取主目录，使用当前目录
		return filepath.Join(".", ".deepwiki")
	}
	return filepath.Join(homeDir, ".deepwiki")
}
