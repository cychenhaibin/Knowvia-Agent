# Knowvia

[English](README.md) | **简体中文**

Knowvia 是一个面向文档知识库的 AI 研究与交付工作台。它以“知识连接”为核心抽象，将来自不同文档系统的内容同步、索引、检索，并结合大模型完成知识问答、研究分析、证据整理和结构化报告生成。

## 项目概览

Knowvia 不只是一个聊天入口。用户可以围绕一个研究目标创建 run，选择知识库范围和 skill，系统会自动完成规划、检索、证据合并、报告生成和 artifact 交付。每次执行都有可追踪的步骤、来源和最终产物。

## 核心能力

- 文档知识库连接：支持把外部文档内容同步到统一的知识索引中。
- 混合检索：结合 lexical、vector、fusion 和 rerank，提升召回与排序质量。
- Grounded chat：基于选定知识范围生成有来源支撑的回答。
- Research run：围绕目标执行长任务，展示规划、步骤进度、来源和结果。
- Structured report：输出摘要、正文、来源和中间 artifact，而不是只返回一段聊天文本。
- Skill runtime：通过 skill 定义不同回答模式、工具权限和报告风格。
- Multi-client：提供 Expo 移动端、Go API 服务、Python LLM 引擎和独立 Web 站点。

## 架构

```text
app / web
   |
   v
server
   |
   +--> Postgres / Redis
   |
   v
llm
   |
   +--> Postgres + pgvector or file backend
   +--> OpenAI-compatible models / Ollama / local fallbacks
```

| 目录 | 说明 |
| --- | --- |
| `app/` | Expo React Native 移动端，负责登录、聊天、run 展示、知识库选择和 skill 操作。 |
| `web/` | 独立官网/展示站点。 |
| `server/` | Go API 与任务编排层，负责认证、会话、知识连接、skills、run 生命周期、事件流和后台执行。 |
| `llm/` | Python Evidence Reasoning Engine，负责索引、检索、重排、流式知识问答和报告生成。 |
| `infra/` | 本地 Postgres/pgvector 和 Redis。 |
| `docs/` | 架构设计、事件协议、LLM 数据库与 API 设计。 |

## 运行流程

1. 用户在 App 中输入目标，选择知识范围和 skill。
2. Go server 创建 run，保存用户意图、执行模式和上下文。
3. Run service 生成计划并逐步执行知识检索、外部搜索、证据合并和报告生成。
4. LLM service 负责知识索引、混合检索、rerank、grounded chat 和 report writer。
5. App 通过接口和事件流展示 run 进度、sources、artifact 和最终报告。

## 环境要求

- Node.js 20.x
- Go 1.25+
- Python 3.10+
- Docker 和 Docker Compose
- 可选：Ollama 或 OpenAI-compatible 模型服务

## 本地开发

所有命令默认从仓库根目录开始执行。

### 1. 启动基础设施

```bash
cd infra
docker compose up -d
```

这会启动本地 Postgres/pgvector 和 Redis。你也可以改用自己的数据库，只要在 `server/.env` 和 `llm/.env` 中配置对应 DSN。

### 2. 启动 Go Server

```bash
cd server
cp .env.example .env
./run_server.sh
```

默认服务地址由 `QQA_SERVER_ADDR` 控制，通常是 `0.0.0.0:8088`。服务启动时会读取 `server/.env`，并在 Postgres 模式下应用数据库 schema。

### 3. 启动 LLM Engine

```bash
cd llm
cp .env.example .env
python -m venv venv
./venv/bin/pip install -r requirements.txt
./run_server.sh
```

LLM 服务默认监听 `http://127.0.0.1:8000`。如果 `server/.env` 中启用了 `QQA_PYTHON_PROXY_BASE_URL`，Go server 会把知识索引、知识问答和报告生成转发给该服务。

### 4. 启动移动端 App

```bash
cd app
nvm use
npm install
npm run dev
```

常用命令：

```bash
npm run dev
npm run dev:android
npm run android
npm run ios
npm run web
```

### 5. 启动 Web 站点

```bash
cd web
npm install
npm run dev
```

Web 站点默认运行在 `http://127.0.0.1:4174`。

## Android 打包

```bash
cd app
nvm use
npm install
npm run build:android:release
```

可用构建命令：

```bash
npm run build:android
npm run build:android:debug
npm run build:android:release
npm run build:android:aab
```

构建产物会输出到 `app/dist/android/`。

## 配置

### Server 环境变量

`server/.env` 的主要配置：

| 变量 | 说明 |
| --- | --- |
| `QQA_SERVER_ADDR` | Go API 监听地址。 |
| `QQA_STORE_BACKEND` | 存储后端，通常为 `postgres` 或 `memory`。 |
| `QQA_POSTGRES_DSN` | Go 主库连接串。 |
| `QQA_REDIS_ADDR` | Redis 地址，用于队列或后续异步能力。 |
| `QQA_OPENAI_BASE_URL` | OpenAI-compatible API 地址。 |
| `QQA_OPENAI_API_KEY` | 模型服务密钥。 |
| `QQA_PYTHON_PROXY_BASE_URL` | Python LLM 服务地址。 |
| `QQA_PYTHON_PROXY_TOKEN` | Go 到 Python 的内部鉴权 token。 |
| `QQA_DEV_USERS` | 本地开发账号。 |

### LLM 环境变量

`llm/.env` 的主要配置：

| 变量 | 说明 |
| --- | --- |
| `QQA_INDEX_BACKEND` | 索引后端，支持 `auto`、`postgres`、`file`。 |
| `QQA_POSTGRES_DSN` | LLM 索引库连接串。 |
| `QQA_POSTGRES_SCHEMA` | LLM 使用的 Postgres schema。 |
| `QQA_GENERATOR_BACKEND` | 生成后端，支持 OpenAI-compatible、Ollama 或 fallback。 |
| `QQA_EMBEDDING_BACKEND` | Embedding 后端。 |
| `QQA_RERANK_BACKEND` | Rerank 后端。 |
| `OPENAI_API_BASE` | OpenAI-compatible 服务地址。 |
| `OPENAI_API_KEY` | OpenAI-compatible 服务密钥。 |
| `OLLAMA_BASE_URL` | Ollama 服务地址。 |
| `OLLAMA_MODEL` | Ollama 模型名。 |

## 测试

### Go

```bash
cd server
go test ./...
```

### LLM

```bash
cd llm
./venv/bin/python -m unittest tests.test_internal_api tests.test_backend_selection
```

部分本地环境如果 Python 依赖是 x86_64 架构，可能需要：

```bash
cd llm
arch -x86_64 ./venv/bin/python -m unittest tests.test_internal_api tests.test_backend_selection
```

### Web

```bash
cd web
npm run build
```

## 文档

- `docs/events.md`：run event stream protocol。
- `docs/go-python-rag-architecture.md`：Go gateway and Python RAG architecture notes。
- `docs/llm-technical-design.md`：LLM technical design。
- `docs/llm-database-and-api-design.md`：LLM database and API design。
- `docs/llm-module-and-class-design.md`：LLM module and class design。

## 开发说明

- Go 是用户态和任务编排的权威层，负责认证、会话、知识连接、skills 和 run 生命周期。
- Python LLM 只负责认知层能力，包括索引、检索、重排、生成和质量评估。
- App 侧通过 API 与事件流展示任务状态，不直接访问 LLM 服务。
- Postgres + pgvector 是推荐的索引后端；文件后端更适合本地调试或轻量回归。
- Skills 由 Go 主存储管理，再同步到 LLM 侧作为运行期镜像。
