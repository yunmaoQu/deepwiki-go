# 贡献指南

感谢您对DeepWiki-Go项目的关注！我们非常欢迎社区贡献，无论是修复bug、改进文档还是添加新功能。

## 开发环境设置

1. Fork本仓库到您的GitHub账户
2. 克隆您的Fork到本地
   ```bash
   git clone https://github.com/YOUR_USERNAME/deepwiki-go.git
   cd deepwiki-go
   ```
3. 添加上游仓库
   ```bash
   git remote add upstream https://github.com/original-owner/deepwiki-go.git
   ```
4. 创建一个新分支进行开发
   ```bash
   git checkout -b feature/your-feature-name
   ```

## 开发流程

1. 确保已安装Go 1.18或更高版本
2. 复制`.env.example`到`.env`并根据需要配置环境变量
3. 运行测试确认环境正常
   ```bash
   go test ./...
   ```
4. 进行开发
5. 添加必要的测试
6. 确保所有测试通过
   ```bash
   go test ./...
   ```
7. 运行linter检查代码质量
   ```bash
   golangci-lint run
   ```

## 提交规范

请遵循以下提交消息格式：

```
<type>(<scope>): <subject>

<body>

<footer>
```

类型（type）可以是：
- **feat**: 新功能
- **fix**: Bug修复
- **docs**: 文档更改
- **style**: 不影响代码含义的更改（空格、格式化等）
- **refactor**: 既不修复bug也不添加功能的代码更改
- **perf**: 改善性能的代码更改
- **test**: 添加或修改测试
- **chore**: 构建过程或辅助工具的变动

例如：
```
feat(rag): 添加上下文窗口滑动功能

实现了在文档检索中使用上下文窗口滑动功能，
提高了检索的精确度。

Closes #123
```

## 提交PR流程

1. 确保您的分支是最新的
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```
2. 将您的更改推送到您的GitHub仓库
   ```bash
   git push origin feature/your-feature-name
   ```
3. 在GitHub上创建一个Pull Request
4. 在PR描述中描述您的更改，并链接到相关的问题
5. 等待代码审查和CI检查完成

## 代码风格

- 遵循Go官方推荐的[代码规范](https://golang.org/doc/effective_go)
- 使用`gofmt`或`goimports`格式化代码
- 确保添加适当的注释和文档
- 函数和方法应该有清晰的职责，遵循单一职责原则

## 测试要求

- 为所有新功能添加单元测试
- 为API端点添加集成测试
- 尽量保持高测试覆盖率

## 行为准则

- 尊重其他贡献者
- 提供建设性的反馈
- 专注于技术讨论而非个人批评
- 乐于帮助新贡献者

再次感谢您的贡献！如有任何问题，请随时在问题跟踪器中提出或联系维护者。 