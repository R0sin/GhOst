# GhOst 👻

本项目是一个在终端中与大语言模型（LLM）优雅对话的客户端，出于个人学习目的而创建。

本项目受 Gemini CLI, Codex, Claude Code 等现代化 AI 终端工具启发，旨在用 Go 语言构建一个功能强大、体验优秀且易于扩展的 LLM 交互应用。

它既是一个实用的工具，也是一个学习 Go 语言进行应用开发的绝佳范例，尤其适合对 TUI（文本用户界面）、API 客户端设计和 Agent 开发感兴趣的开发者。

---

## ✨ 功能特性

- **双交互模式**:
  - **直接模式**: 通过 `ghost -p "你的问题"` 或 `ghost "你的问题"` 实现快速问答，获取结果后立即退出。
  - **交互模式**: 直接运行 `ghost` 进入沉浸式的 TUI 界面，支持多轮上下文对话。
- **灵活的配置**: 通过项目根目录下的 `.ghost.yaml` 文件管理 API 地址、密钥和模型名称，实现代码与配置分离。
- **优雅的 TUI**: 基于 `charmbracelet/bubbletea` 构建，提供流畅的、带状态（加载中、错误提示）的对话体验。
- **美观的样式**: 使用 `charmbracelet/lipgloss` 对对话角色进行着色，界面清晰易读。
- **健壮的命令结构**: 基于 `spf13/cobra` 构建，命令结构清晰，易于未来扩展。

## 🛠️ 技术栈

- **CLI 框架**: [spf13/cobra](https://github.com/spf13/cobra)
- **配置管理**: [spf13/viper](https://github.com/spf13/viper)
- **TUI 框架**: [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)
- **TUI 组件**: [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles)
- **终端样式**: [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss)

## 🚀 安装与运行

### 1. 环境准备

- **Go**: 确保你已经安装了 Go 语言环境 (推荐版本 1.22 或更高)。
- **Git**: 用于克隆本项目。

### 2. 下载与安装

```bash
# 1. 克隆项目到本地
git clone https://github.com/R0sin/GhOst.git

# 2. 进入项目目录
cd GhOst

# 3. 下载项目依赖
go mod tidy
```

### 3. 配置

在项目根目录下，创建一个名为 `.ghost.yaml` 的文件，并填入以下内容。请将 `api_key` 替换为你自己的密钥。

```yaml
# .ghost.yaml

# 你的 OpenAI 标准 API 地址
api_url: "http://localhost:3000/v1"

# 你的 API 密钥
api_key: "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# 你希望使用的模型名称
model: "gemini-2.0-flash"
```

### 4. 运行

- **直接模式**:

  ```bash
  go run main.go -p "你好，世界！"
  ```

  或者

  ```bash
  go run main.go "你好，世界！"
  ```

- **交互模式**:

  ```bash
  go run main.go
  ```

  在 TUI 界面中，输入你的问题后按 `Enter` 键发送。按 `Ctrl+C` 或 `Esc` 退出程序。

## 🗺️ 开发计划

- [x] **Markdown 渲染**: 使用 `charmbracelet/glamour` 实现对模型返回的 Markdown 格式内容进行美化渲染。
- [x] **流式响应**: 支持 LLM 的流式输出，实现打字机效果，提升响应体验。
- [ ] **Agent 能力扩展（进行中）**: 赋予模型与本地环境交互的能力。
- [ ] **对话历史管理**: 实现保存和加载对话历史的功能。
- [ ] **多渠道支持**: 添加主流 LLM API 渠道支持。
- [ ] **更丰富的配置**: 增加更多可配置项，如温度、上下文长度等。
