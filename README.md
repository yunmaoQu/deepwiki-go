# DeepWiki-Go

DeepWiki-Go是一个基于Go语言实现的代码知识库自动生成工具，它可以分析任何GitHub或GitLab代码仓库，并自动生成一个结构化、交互式的Wiki文档。

## 🔍 核心功能

- **代码库分析**：自动分析仓库结构和代码依赖关系
- **Wiki自动生成**：生成模块介绍、架构说明和API文档
- **可视化图表**：自动创建架构图和流程图以解释代码关系
- **RAG搜索**：基于检索增强生成的智能代码搜索
- **私有仓库支持**：支持通过访问令牌访问私有仓库
- **多语言支持**：分析支持超过20种主流编程语言

## 🛠️ 技术架构

```
deepwiki-go/
├── cmd/                  # 程序入口
│   └── main.go           # 主程序入口
├── internal/             # 内部应用代码
│   ├── api/              # API服务器
│   │   ├── handlers.go   # API处理器
│   │   ├── middleware.go # 中间件
│   │   └── routes.go     # 路由配置
│   ├── config/           # 配置管理
│   │   └── config.go     # 配置结构和初始化
│   ├── data/             # 数据处理
│   │   ├── database.go   # 数据库操作
│   │   ├── embedding.go  # 文本嵌入处理
│   │   ├── repository.go # 仓库管理
│   │   └── storage.go    # 向量存储
│   ├── models/           # 数据模型
│   │   └── models.go     # 模型定义
│   └── rag/              # 检索增强生成
│       ├── memory.go     # 内存缓存
│       └── rag.go        # RAG实现
└── pkg/                  # 公共工具包
    └── utils/            # 工具函数
        ├── git.go        # Git操作
        └── token.go      # 令牌处理
```

## 🚀 快速开始

### 依赖条件

- Go 1.18+
- Git
- Google API密钥（用于AI生成）
- OpenAI API密钥（用于文本嵌入）

### 环境设置

1. 克隆仓库
```bash
git clone https://github.com/yourusername/deepwiki-go.git
cd deepwiki-go
```

2. 创建`.env`文件
```
GOOGLE_API_KEY=your_google_api_key
OPENAI_API_KEY=your_openai_api_key
PORT=8001  # 可选，默认为8001
```

### 构建和运行

1. 构建应用
```bash
go build -o deepwiki ./cmd/
```

2. 运行应用
```bash
./deepwiki
```

应用将在 http://localhost:8001 启动API服务器。

### Docker部署

1. 构建Docker镜像
```bash
docker build -t deepwiki-go .
```

2. 运行容器
```bash
docker run -d -p 8001:8001 --env-file .env --name deepwiki deepwiki-go
```

## 📝 API使用

### 生成Wiki

```bash
curl -X POST http://localhost:8001/api/v1/wiki/generate \
  -H "Content-Type: application/json" \
  -d '{"repo_url": "https://github.com/username/repo", "github_token": "your_token"}'
```

### 搜索文档

```bash
curl -X POST http://localhost:8001/api/v1/vectors/search \
  -H "Content-Type: application/json" \
  -d '{"query": "如何实现用户认证", "repo_url": "https://github.com/username/repo"}'
```

## 🤝 贡献指南

欢迎提交问题和Pull Request！请查看[贡献指南](CONTRIBUTING.md)了解更多信息。

## 📄 许可证

本项目采用[MIT许可证](LICENSE)。
