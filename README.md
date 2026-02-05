# Tachigoma

<img src="images/tachigoma.png" alt="tachigoma" width="256"/>

Tachigoma 是一个在终端中与大语言模型（LLM）交互的 Agent，出于个人学习目的而创建。

本项目受 Gemini CLI, Codex, Claude Code 等现代化 AI 终端工具启发，旨在用 Go 语言构建一个功能强大、体验优秀且易于扩展的 LLM 交互应用。

它既是一个实用的工具，也是一个学习 Go 语言进行应用开发的范例，尤其适合对 TUI（文本用户界面）、API 客户端设计和 Agent 开发感兴趣的开发者。

---

## ✨ 功能特性

- **双交互模式**:
  - **直接模式**: 通过 `tachigoma -p "你的问题"` 实现快速问答，支持流式输出。
  - **交互模式**: 直接运行 `tachigoma` 进入沉浸式 TUI 界面，支持多轮上下文对话。
- **管道输入**: 支持 Unix 管道，可将文件或命令输出传递给 LLM 分析。
- **灵活的配置**: 支持 XDG Base Directory 规范，配置可通过环境变量、用户配置文件或系统配置文件进行管理。
- **优雅的 TUI**: 基于 `charmbracelet/bubbletea` 构建，提供流畅的、带状态的对话体验。
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
git clone https://github.com/R0sin/tachigoma.git

# 2. 进入项目目录
cd tachigoma

# 3. 下载项目依赖
go mod tidy
```

### 3. 配置

Tachigoma 支持灵活的配置方式，遵循 XDG Base Directory 规范，适用于个人开发和多用户服务器部署场景。

#### 配置文件位置

配置文件按以下优先级顺序搜索（优先级从高到低）：

| 优先级 | 路径 | 说明 |
|:------:|------|------|
| 1 | `./config.yaml` | 当前目录，开发调试时使用 |
| 2 | `~/.config/tachigoma/config.yaml` | 用户级配置 (Linux/macOS) |
| 2 | `%APPDATA%\tachigoma\config.yaml` | 用户级配置 (Windows) |
| 3 | `~/.tachigoma.yaml` | 向后兼容旧配置 |
| 4 | `/etc/tachigoma/config.yaml` | 系统级配置 (Linux/macOS) |

#### 环境变量

所有配置项都可以通过环境变量覆盖，使用 `TACHIGOMA_` 前缀：

```bash
export TACHIGOMA_API_KEY="sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export TACHIGOMA_API_URL="https://api.openai.com/v1"
export TACHIGOMA_MODEL="gpt-4"
```

**配置优先级**: 环境变量 > 配置文件 > 默认值

#### 配置文件示例

创建 `~/.config/tachigoma/config.yaml`（Linux/macOS）或 `%APPDATA%\tachigoma\config.yaml`（Windows）：

```yaml
# Tachigoma 配置文件

# 你的 OpenAI 标准 API 地址
api_url: "https://api.openai.com/v1"

# 你的 API 密钥 (建议通过环境变量 TACHIGOMA_API_KEY 设置)
api_key: "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# 你希望使用的模型名称
model: "gpt-4"
```

#### 多用户服务器部署

对于多用户服务器场景，推荐以下配置策略：

1. **系统管理员**: 在 `/etc/tachigoma/config.yaml` 设置共享的 `api_url` 和默认 `model`
2. **各用户**: 通过环境变量 `TACHIGOMA_API_KEY` 设置个人 API 密钥（敏感信息不落盘）
3. **用户个性化**: 可选在 `~/.config/tachigoma/config.yaml` 覆盖个人偏好设置

> **安全提示**: 强烈建议通过环境变量传递 `api_key`，避免将敏感信息写入文件。如必须使用配置文件，请确保文件权限为 `600`。

### 4. 运行

**直接模式**（流式输出）:

```bash
tachigoma -p "你的问题"
```

**管道输入**:

```bash
# 分析文件内容
cat file.txt | tachigoma -p "解释这个文件"

# 代码审查
git diff | tachigoma -p "review this change"

# 分析命令输出
ls -la | tachigoma -p "解释这些文件"
```

**交互模式**:

```bash
tachigoma
```

在 TUI 界面中，输入你的问题后按 `Enter` 发送。按 `Ctrl+C` 或 `Esc` 退出。

## 🗺️ 开发计划

- [x] **Markdown 渲染**: 使用 `charmbracelet/glamour` 实现对模型返回的 Markdown 格式内容进行美化渲染。
- [x] **流式响应**: 支持 LLM 的流式输出，实现打字机效果，提升响应体验。
- [x] **Agent 1.0**: 实现工具调用支持等基本 Agent 能力。
- [ ] **对话历史管理**: 实现保存和加载对话历史的功能。
- [ ] **上下文压缩**: 优化上下文结构以支持复杂任务。
- [ ] **多渠道支持**: 添加主流 LLM API 渠道支持。
- [ ] **更丰富的配置**: 增加更多可配置项，如温度、上下文长度等。
