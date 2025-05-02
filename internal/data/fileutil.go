// internal/data/fileutil.go
package data

import (
        "encoding/base64"
        "encoding/json"
        "errors"
        "fmt"
        "io/ioutil"
        "net/http"
        "strings"
)

// GetFileContent 从 Git 仓库（GitHub 或 GitLab）获取文件内容
func GetFileContent(repoURL string, filePath string, accessToken string) (string, error) {
        if strings.Contains(repoURL, "github.com") {
                return GetGitHubFileContent(repoURL, filePath, accessToken)
        } else if strings.Contains(repoURL, "gitlab.com") {
                return GetGitLabFileContent(repoURL, filePath, accessToken)
        } else {
                return "", errors.New("不支持的仓库 URL。仅支持 GitHub 和 GitLab")
        }
}

// GetGitHubFileContent 使用 GitHub API 获取文件内容
func GetGitHubFileContent(repoURL string, filePath string, accessToken string) (string, error) {
        // 检查 URL 是否是有效的 GitHub URL
        if !strings.HasPrefix(repoURL, "https://github.com/") && !strings.HasPrefix(repoURL, "http://github.com/") {
                return "", errors.New("不是有效的 GitHub 仓库 URL")
        }
        
        // 从 GitHub URL 提取所有者和仓库名
        parts := strings.Split(strings.TrimRight(repoURL, "/"), "/")
        if len(parts) < 5 {
                return "", errors.New("无效的 GitHub URL 格式")
        }
        
        owner := parts[len(parts)-2]
        repo := strings.TrimSuffix(parts[len(parts)-1], ".git")
        
        // 使用 GitHub API 获取文件内容
        apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, filePath)
        
        // 创建请求
        req, err := http.NewRequest("GET", apiURL, nil)
        if err != nil {
                return "", err
        }
        
        // 如果提供了访问令牌，添加认证
        if accessToken != "" {
                req.Header.Add("Authorization", "token "+accessToken)
        }
        
        // 发送请求
        client := &http.Client{}
        resp, err := client.Do(req)
        if err != nil {
                return "", err
        }
        defer resp.Body.Close()
        
        // 读取响应
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                return "", err
        }
        
        // 检查是否收到错误响应
        if resp.StatusCode != http.StatusOK {
                var errorResp struct {
                        Message string `json:"message"`
                }
                if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Message != "" {
                        return "", fmt.Errorf("GitHub API 错误: %s", errorResp.Message)
                }
                return "", fmt.Errorf("GitHub API 返回状态码: %d", resp.StatusCode)
        }
        
        // 解析 JSON 响应
        var contentData struct {
                Content  string `json:"content"`
                Encoding string `json:"encoding"`
        }
        
        if err := json.Unmarshal(body, &contentData); err != nil {
                return "", err
        }
        
        // GitHub API 返回 base64 编码的文件内容
        if contentData.Encoding == "base64" {
                // 内容可能被分成多行，先连接它们
                contentBase64 := strings.ReplaceAll(contentData.Content, "\n", "")
                content, err := base64.StdEncoding.DecodeString(contentBase64)
                if err != nil {
                        return "", err
                }
                return string(content), nil
        }
        
        return "", fmt.Errorf("意外的编码: %s", contentData.Encoding)
}

// GetGitLabFileContent 使用 GitLab API 获取文件内容
func GetGitLabFileContent(repoURL string, filePath string, accessToken string) (string, error) {
        // 检查 URL 是否是有效的 GitLab URL
        if !strings.HasPrefix(repoURL, "https://gitlab.com/") && !strings.HasPrefix(repoURL, "http://gitlab.com/") {
                return "", errors.New("不是有效的 GitLab 仓库 URL")
        }
        
        // 从 GitLab URL 提取项目路径
        parts := strings.Split(strings.TrimRight(repoURL, "/"), "/")
        if len(parts) < 5 {
                return "", errors.New("无效的 GitLab URL 格式")
        }
        
        // 移除域名部分
        pathParts := parts[3:]
        // 连接剩余部分以获取项目路径
        projectPath := strings.Join(pathParts, "/")
        projectPath = strings.TrimSuffix(projectPath, ".git")
        // URL 编码路径以用于 API
        encodedProjectPath := strings.ReplaceAll(projectPath, "/", "%2F")
        
        // URL 编码文件路径
        encodedFilePath := strings.ReplaceAll(filePath, "/", "%2F")
        
        // 使用 GitLab API 获取文件内容
        apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/repository/files/%s/raw?ref=main", 
                encodedProjectPath, encodedFilePath)
        
        // 创建请求
        req, err := http.NewRequest("GET", apiURL, nil)
        if err != nil {
                return "", err
        }
        
        // 如果提供了访问令牌，添加认证
        if accessToken != "" {
                req.Header.Add("PRIVATE-TOKEN", accessToken)
        }
        
        // 发送请求
        client := &http.Client{}
        resp, err := client.Do(req)
        if err != nil {
                return "", err
        }
        defer resp.Body.Close()
        
        // 读取响应
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                return "", err
        }
        
        // 检查是否收到错误响应
        if resp.StatusCode != http.StatusOK {
                // 尝试使用 master 分支而不是 main
                apiURL = fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/repository/files/%s/raw?ref=master", 
                        encodedProjectPath, encodedFilePath)
                
                req, err := http.NewRequest("GET", apiURL, nil)
                if err != nil {
                        return "", err
                }
                
                if accessToken != "" {
                        req.Header.Add("PRIVATE-TOKEN", accessToken)
                }
                
                resp2, err := client.Do(req)
                if err != nil {
                        return "", err
                }
                defer resp2.Body.Close()
                
                body2, err := ioutil.ReadAll(resp2.Body)
                if err != nil {
                        return "", err
                }
                
                if resp2.StatusCode != http.StatusOK {
                        // 检查是否是 JSON 错误响应
                        if strings.HasPrefix(string(body2), "{") && strings.Contains(string(body2), "\"message\":") {
                                var errorResp struct {
                                        Message string `json:"message"`
                                }
                                if err := json.Unmarshal(body2, &errorResp); err == nil && errorResp.Message != "" {
                                        return "", fmt.Errorf("GitLab API 错误: %s", errorResp.Message)
                                }
                        }
                        return "", fmt.Errorf("GitLab API 返回状态码: %d", resp2.StatusCode)
                }
                
                return string(body2), nil
        }
        
        return string(body), nil
}