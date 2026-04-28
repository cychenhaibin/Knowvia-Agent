# Knowvia LLM 数据库与 API 设计

## 1. 文档目的

本文是 [LLM 技术方案](./llm-technical-design.md) 的落地细化版，聚焦两个问题：

1. QuickQue 的 LLM 子系统最终应该存什么数据
2. Go 与 LLM 之间最终应该通过什么内部 API 交互

本文只描述 **LLM 子系统自己的目标态设计**，不是对当前 `quickque-agent/llm` 实现的注释，也不是对旧版 `yuque-rag` 接口的兼容说明。

## 2. 设计前提

### 2.1 产品前提

Knowvia 的最终产品主链不是“聊天”，而是：

- 用户创建 Run
- 系统执行显式 research workflow
- 收集内部知识与外部网页证据
- 生成 Timeline、Sources、Artifacts、Final Report

因此，LLM 的数据与接口设计必须同时支持：

- `Knowledge Chat`
- `Run / Report Generation`

### 2.2 系统职责前提

Go 是产品主后端，LLM 是内部认知引擎。

因此：

- Go 维护业务主存储
- LLM 维护推理必需的本地索引和 trace
- Go 负责同步编排
- LLM 负责索引、检索、重排、合成和生成

### 2.3 部署前提

推荐部署方式：

- 使用 **同一个 Postgres 实例**
- Go 继续使用现有默认 `public` schema
- LLM 使用 **独立的 `llm` schema**

这样做的原因：

- 复用现有 `infra` 中的 Postgres/pgvector
- 避免拆分成两个数据库导致运维复杂度上升
- 保持逻辑隔离，避免和 Go 的业务表混用

## 3. 数据分层

LLM 数据分为三层：

1. **配置与镜像层**
   - scope 配置
   - skill 镜像
   - model profile

2. **知识索引层**
   - 文档
   - 文档版本
   - chunk
   - 词法索引
   - 向量索引

3. **推理观测层**
   - retrieval traces
   - chat traces
   - report traces
   - index jobs

LLM 不存以下业务主数据：

- 用户公开会话主表
- run 主表
- run_artifacts 主表
- run_sources 主表
- knowledge connection 主表

这些仍由 Go 维护。

## 4. 与现有 Go 数据的关系

当前 Go 侧已经存在下列表与模型：

- `knowledge_connections`
- `knowledge_connection_yuque_configs`
- `knowledge_connection_feishu_configs`
- `knowledge_source_documents`
- `knowledge_document_meta`
- `skill_runtime_snapshots`
- `runs`
- `run_steps`
- `run_artifacts`
- `run_sources`

对应参考：

- [000001_init.sql](/Users/chenhaibin/Desktop/yuque-rag/quickque-agent/server/migrations/000001_init.sql:75)
- [000002_knowledge_metadata.sql](/Users/chenhaibin/Desktop/yuque-rag/quickque-agent/server/migrations/000002_knowledge_metadata.sql:1)
- [000003_skill_runtime.sql](/Users/chenhaibin/Desktop/yuque-rag/quickque-agent/server/migrations/000003_skill_runtime.sql:38)
- [000014_provider_specific_knowledge_connection_configs.sql](/Users/chenhaibin/Desktop/yuque-rag/quickque-agent/server/migrations/000014_provider_specific_knowledge_connection_configs.sql:1)
- [000016_knowledge_source_documents.sql](/Users/chenhaibin/Desktop/yuque-rag/quickque-agent/server/migrations/000016_knowledge_source_documents.sql:1)

目标态中，LLM 不复制这些表的主职责，而是只消费 Go 发来的标准化输入。

推荐关系如下：

- Go 的 `knowledge_connections.id` -> LLM 的 `scope_external_id`
- Go 的 `skill_runtime_snapshots.id` -> LLM 请求中的 `skill_snapshot_id`
- Go 的 `run.id` -> LLM trace 中的 `run_id`
- Go 的 `knowledge_source_documents` -> LLM 的 indexing input

## 5. 目标数据库设计

以下所有表默认位于 `llm` schema 中。

### 5.1 `llm.index_scopes`

用途：

- 表示一个可索引知识范围
- 通常与 Go 的一个 `knowledge connection` 对应

字段：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `TEXT PK` | LLM 内部 scope id |
| `user_id` | `TEXT NOT NULL` | 所属用户 |
| `scope_type` | `TEXT NOT NULL` | `knowledge_connection` / `ad_hoc_bundle` |
| `scope_external_id` | `TEXT NOT NULL` | 外部主键，通常是 Go 侧 connection id |
| `provider` | `TEXT NOT NULL` | `yuque` / `feishu` / `web` / `mixed` |
| `name` | `TEXT NOT NULL` | 作用域显示名 |
| `status` | `TEXT NOT NULL` | `active` / `disabled` / `deleted` |
| `latest_index_version` | `TEXT NOT NULL DEFAULT ''` | 当前索引版本 |
| `document_count` | `INTEGER NOT NULL DEFAULT 0` | 文档数 |
| `chunk_count` | `INTEGER NOT NULL DEFAULT 0` | chunk 数 |
| `last_indexed_at` | `TIMESTAMPTZ NULL` | 最近成功索引时间 |
| `created_at` | `TIMESTAMPTZ NOT NULL` | 创建时间 |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | 更新时间 |

约束与索引：

- `UNIQUE (user_id, scope_type, scope_external_id)`
- `INDEX (user_id, updated_at DESC)`
- `INDEX (status, last_indexed_at DESC)`

### 5.2 `llm.knowledge_documents`

用途：

- 表示逻辑文档
- 不直接保存所有版本正文

字段：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `TEXT PK` | 文档内部 id |
| `scope_id` | `TEXT NOT NULL` | 引用 `llm.index_scopes.id` |
| `user_id` | `TEXT NOT NULL` | 所属用户 |
| `provider` | `TEXT NOT NULL` | 来源 provider |
| `external_id` | `TEXT NOT NULL` | 上游文档 id |
| `repo` | `TEXT NOT NULL` | 文档所在知识库/空间 |
| `title` | `TEXT NOT NULL` | 文档标题 |
| `doc_ref` | `TEXT NOT NULL` | slug / path / token |
| `source_url` | `TEXT NOT NULL DEFAULT ''` | 原始来源链接 |
| `source_updated_at` | `TIMESTAMPTZ NOT NULL` | 上游更新时间 |
| `content_hash` | `TEXT NOT NULL` | 标准化正文 hash |
| `latest_version_id` | `TEXT NOT NULL DEFAULT ''` | 最近版本 |
| `is_deleted` | `BOOLEAN NOT NULL DEFAULT FALSE` | 软删除 |
| `created_at` | `TIMESTAMPTZ NOT NULL` | 创建时间 |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | 更新时间 |

约束与索引：

- `UNIQUE (scope_id, external_id)`
- `INDEX (scope_id, source_updated_at DESC)`
- `INDEX (scope_id, is_deleted, updated_at DESC)`

### 5.3 `llm.knowledge_document_versions`

用途：

- 保存文档标准化后的版本快照
- 支持重建索引与追踪

字段：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `TEXT PK` | 版本 id |
| `document_id` | `TEXT NOT NULL` | 引用文档 |
| `version_no` | `INTEGER NOT NULL` | 版本号 |
| `raw_body_ref` | `TEXT NOT NULL DEFAULT ''` | 原始正文存储引用，可为空 |
| `normalized_body` | `TEXT NOT NULL` | 标准化正文 |
| `outline_json` | `JSONB NOT NULL DEFAULT '{}'` | 标题结构/结构化摘要 |
| `body_hash` | `TEXT NOT NULL` | 版本 hash |
| `token_count` | `INTEGER NOT NULL DEFAULT 0` | token 数 |
| `created_at` | `TIMESTAMPTZ NOT NULL` | 创建时间 |

约束与索引：

- `UNIQUE (document_id, version_no)`
- `UNIQUE (document_id, body_hash)`
- `INDEX (document_id, created_at DESC)`

### 5.4 `llm.knowledge_chunks`

用途：

- 检索最小单位
- 同时承载 lexical 与 vector 检索字段

字段：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `TEXT PK` | chunk id |
| `scope_id` | `TEXT NOT NULL` | 引用 scope |
| `user_id` | `TEXT NOT NULL` | 所属用户 |
| `document_id` | `TEXT NOT NULL` | 引用文档 |
| `version_id` | `TEXT NOT NULL` | 引用文档版本 |
| `provider` | `TEXT NOT NULL` | 来源 provider |
| `repo` | `TEXT NOT NULL` | 知识空间名 |
| `title` | `TEXT NOT NULL` | 文档标题 |
| `heading_path` | `TEXT NOT NULL DEFAULT ''` | 标题路径 |
| `source_url` | `TEXT NOT NULL DEFAULT ''` | 来源链接 |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | 文档更新时间 |
| `chunk_index` | `INTEGER NOT NULL` | chunk 序号 |
| `content` | `TEXT NOT NULL` | chunk 正文 |
| `content_hash` | `TEXT NOT NULL` | chunk hash |
| `token_count` | `INTEGER NOT NULL DEFAULT 0` | token 数 |
| `lexical_text` | `TEXT NOT NULL` | 词法索引文本 |
| `tsv` | `TSVECTOR` | PostgreSQL 全文索引字段 |
| `embedding` | `VECTOR(1536)` | 向量字段，维度按模型配置 |
| `created_at` | `TIMESTAMPTZ NOT NULL` | 创建时间 |

约束与索引：

- `UNIQUE (version_id, chunk_index)`
- `INDEX (scope_id, updated_at DESC)`
- `GIN INDEX (tsv)`
- `HNSW/IVFFLAT INDEX (embedding)`
- `INDEX (document_id, chunk_index)`

说明：

- 如果 embedding 维度可能变化，推荐为每个 index_version 使用固定模型，不混写。
- 也可以将 `embedding` 拆到独立表，但第一版不建议过度拆分。

### 5.5 `llm.skill_mirrors`

用途：

- LLM 本地运行期 skill 镜像
- 不作为主权威源

字段：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `skill_id` | `TEXT PK` | skill id |
| `user_id` | `TEXT NOT NULL` | 所属用户 |
| `scope` | `TEXT NOT NULL DEFAULT 'global'` | 当前先按全局镜像处理 |
| `definition_id` | `TEXT NOT NULL DEFAULT ''` | skill definition |
| `revision_id` | `TEXT NOT NULL DEFAULT ''` | 当前 revision |
| `kind` | `TEXT NOT NULL DEFAULT 'chat_profile'` | skill kind |
| `title` | `TEXT NOT NULL DEFAULT ''` | 标题 |
| `description` | `TEXT NOT NULL DEFAULT ''` | 描述 |
| `mode` | `TEXT NOT NULL DEFAULT 'answer'` | 输出模式 |
| `prompt` | `TEXT NOT NULL DEFAULT ''` | skill prompt |
| `runtime_spec_json` | `JSONB NOT NULL DEFAULT '{}'` | 运行时 spec |
| `enabled` | `BOOLEAN NOT NULL DEFAULT TRUE` | 是否启用 |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | 更新时间 |

约束与索引：

- `INDEX (user_id, updated_at DESC)`
- `INDEX (user_id, enabled, mode)`

### 5.6 `llm.model_profiles`

用途：

- 管理 LLM 内部使用的模型 profile
- 支持 chat/report 分离

字段：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `TEXT PK` | profile id |
| `user_id` | `TEXT NOT NULL DEFAULT ''` | 空表示系统默认 |
| `purpose` | `TEXT NOT NULL` | `chat_fast` / `chat_quality` / `report_writer` |
| `provider` | `TEXT NOT NULL` | `openai_compatible` / `ollama` |
| `name` | `TEXT NOT NULL` | profile 名称 |
| `base_url` | `TEXT NOT NULL DEFAULT ''` | API base |
| `api_key_ref` | `TEXT NOT NULL DEFAULT ''` | 密钥引用，不存明文 |
| `model_name` | `TEXT NOT NULL` | 模型名 |
| `temperature` | `DOUBLE PRECISION NOT NULL DEFAULT 0.2` | 温度 |
| `max_tokens` | `INTEGER NOT NULL DEFAULT 4096` | token 上限 |
| `is_default` | `BOOLEAN NOT NULL DEFAULT FALSE` | 是否默认 |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | 更新时间 |

约束与索引：

- `UNIQUE (user_id, purpose, lower(name))`
- `UNIQUE (user_id, purpose) WHERE is_default`

### 5.7 `llm.index_jobs`

用途：

- 记录批量 upsert / rebuild / delete 的任务过程

字段：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `TEXT PK` | 任务 id |
| `user_id` | `TEXT NOT NULL` | 所属用户 |
| `scope_id` | `TEXT NOT NULL` | 作用域 |
| `job_type` | `TEXT NOT NULL` | `upsert_batch` / `rebuild_scope` / `delete_scope` |
| `status` | `TEXT NOT NULL` | `queued` / `running` / `completed` / `failed` |
| `document_count` | `INTEGER NOT NULL DEFAULT 0` | 文档数 |
| `chunk_count` | `INTEGER NOT NULL DEFAULT 0` | chunk 数 |
| `error_code` | `TEXT NOT NULL DEFAULT ''` | 错误码 |
| `error_message` | `TEXT NOT NULL DEFAULT ''` | 错误信息 |
| `started_at` | `TIMESTAMPTZ NULL` | 开始时间 |
| `finished_at` | `TIMESTAMPTZ NULL` | 结束时间 |
| `created_at` | `TIMESTAMPTZ NOT NULL` | 创建时间 |

约束与索引：

- `INDEX (scope_id, created_at DESC)`
- `INDEX (status, created_at DESC)`

### 5.8 `llm.request_traces`

用途：

- 统一记录 retrieve/chat/report 的推理链路

字段：

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `TEXT PK` | trace id |
| `request_type` | `TEXT NOT NULL` | `retrieve` / `chat` / `report` |
| `user_id` | `TEXT NOT NULL` | 所属用户 |
| `run_id` | `TEXT NOT NULL DEFAULT ''` | 可选 run id |
| `message_id` | `TEXT NOT NULL DEFAULT ''` | 可选 chat message id |
| `scope_ids` | `TEXT[] NOT NULL DEFAULT '{}'` | 作用域集合 |
| `query_text` | `TEXT NOT NULL DEFAULT ''` | 原问题/goal |
| `mode` | `TEXT NOT NULL DEFAULT ''` | answer/summary/actions/report |
| `skill_id` | `TEXT NOT NULL DEFAULT ''` | skill id |
| `model_profile_id` | `TEXT NOT NULL DEFAULT ''` | 模型 profile |
| `retrieved_chunk_ids` | `TEXT[] NOT NULL DEFAULT '{}'` | 初始召回结果 |
| `reranked_chunk_ids` | `TEXT[] NOT NULL DEFAULT '{}'` | 精排结果 |
| `used_chunk_ids` | `TEXT[] NOT NULL DEFAULT '{}'` | 真正送入 prompt 的 chunk |
| `latency_retrieve_ms` | `INTEGER NOT NULL DEFAULT 0` | 检索耗时 |
| `latency_generate_ms` | `INTEGER NOT NULL DEFAULT 0` | 生成耗时 |
| `latency_total_ms` | `INTEGER NOT NULL DEFAULT 0` | 总耗时 |
| `output_preview` | `TEXT NOT NULL DEFAULT ''` | 输出预览 |
| `error_code` | `TEXT NOT NULL DEFAULT ''` | 错误码 |
| `created_at` | `TIMESTAMPTZ NOT NULL` | 创建时间 |

约束与索引：

- `INDEX (user_id, created_at DESC)`
- `INDEX (request_type, created_at DESC)`
- `INDEX (run_id, created_at DESC) WHERE run_id <> ''`

## 6. 数据生命周期

### 6.1 文档生命周期

1. Go 将标准化文档包发给 LLM
2. LLM 先确认 scope 是否存在，不存在则创建 `index_scopes`
3. 对每篇文档计算 `content_hash`
4. 如果 hash 未变化，只更新状态和索引版本，不重建 chunk
5. 如果 hash 变化：
   - 插入新 version
   - 删除旧 version 关联 chunk
   - 重建 chunk 与 embedding
6. 更新 `knowledge_documents.latest_version_id`
7. 更新 `index_scopes.latest_index_version`

### 6.2 软删除策略

对于上游删除：

- `knowledge_documents.is_deleted = true`
- 删除对应 active chunks
- 保留 document/version 以支持审计与 trace

### 6.3 Trace 保留策略

- chat/retrieve/report traces 默认保留 30 天
- report traces 可保留更久，用于质量复盘

## 7. API 设计原则

### 7.1 原则

- 只提供内部 API
- 前端不直接访问
- 统一使用 `Authorization: Bearer <internal-token>`
- 所有请求必须显式带 `user_id`
- 所有知识查询必须显式带 `scope_ids`
- SSE 事件协议稳定，不随内部实现改变

### 7.2 版本策略

建议使用：

- `/internal/v1/...`

避免未来改 schema 时直接破坏 Go 侧调用。

## 8. 统一数据对象

### 8.1 `ScopeRef`

```json
{
  "scopeId": "conn_123",
  "provider": "yuque",
  "name": "产品知识库"
}
```

### 8.2 `InputDocument`

```json
{
  "externalId": "456789",
  "provider": "yuque",
  "repo": "group/repo",
  "title": "发布说明",
  "docRef": "release-notes",
  "sourceUrl": "https://www.yuque.com/group/repo/release-notes",
  "sourceUpdatedAt": "2026-04-25T08:30:00Z",
  "rawBody": "<p>...</p>",
  "normalizedBody": "",
  "metadata": {
    "authorName": "Alice",
    "sourceType": "doc"
  }
}
```

说明：

- `normalizedBody` 允许为空，由 LLM 内部生成
- 若上游已有高质量 normalize 结果，也允许直接传入

### 8.3 `RetrievedSource`

```json
{
  "type": "knowledge_base",
  "provider": "yuque",
  "scopeId": "conn_123",
  "documentId": "doc_abc",
  "chunkId": "chk_xyz",
  "title": "发布说明",
  "repo": "group/repo",
  "url": "https://www.yuque.com/group/repo/release-notes",
  "snippet": "......",
  "score": 0.92
}
```

### 8.4 `SkillContext`

```json
{
  "skillId": "skill_123",
  "mode": "summary",
  "prompt": "重点输出变更摘要和建议动作",
  "runtimeSpec": {
    "instructions": "对内部产品问题优先引用知识库依据",
    "responseMode": "summary"
  }
}
```

## 9. API 规范

## 9.1 `POST /internal/v1/index/upsert-batch`

用途：

- 批量写入一个 scope 下的文档并构建索引

请求体：

```json
{
  "userId": "user_123",
  "scope": {
    "scopeId": "conn_123",
    "provider": "yuque",
    "name": "产品知识库"
  },
  "documents": [
    {
      "externalId": "456789",
      "provider": "yuque",
      "repo": "group/repo",
      "title": "发布说明",
      "docRef": "release-notes",
      "sourceUrl": "https://www.yuque.com/group/repo/release-notes",
      "sourceUpdatedAt": "2026-04-25T08:30:00Z",
      "rawBody": "<p>...</p>",
      "normalizedBody": ""
    }
  ],
  "syncMode": "replace"
}
```

字段说明：

- `syncMode`
  - `replace`: 以请求中的文档全集替换该 scope 当前文档集
  - `merge`: 只做增量 upsert，不删除未出现的文档

响应体：

```json
{
  "jobId": "job_123",
  "scopeId": "conn_123",
  "documentCount": 128,
  "chunkCount": 2431,
  "indexVersion": "2026-04-25T09:00:00Z#bce-v1",
  "changedDocuments": {
    "added": 8,
    "updated": 3,
    "deleted": 1,
    "unchanged": 116
  }
}
```

错误码：

- `400` 请求参数非法
- `401` 内部 token 无效
- `409` scope 正在重建
- `500` 索引写入失败

## 9.2 `DELETE /internal/v1/index/scopes/{scopeId}`

用途：

- 删除一个 scope 的全部知识索引

请求参数：

- path: `scopeId`
- body:

```json
{
  "userId": "user_123"
}
```

响应体：

```json
{
  "status": "ok",
  "scopeId": "conn_123"
}
```

## 9.3 `POST /internal/v1/retrieve`

用途：

- 调试 retrieval pipeline
- 支持评估与可视化分析

请求体：

```json
{
  "userId": "user_123",
  "query": "四月版本有哪些权限变化",
  "scopeIds": ["conn_123", "conn_456"],
  "topK": 8,
  "returnTrace": true
}
```

响应体：

```json
{
  "traceId": "trace_123",
  "sources": [
    {
      "type": "knowledge_base",
      "provider": "yuque",
      "scopeId": "conn_123",
      "documentId": "doc_abc",
      "chunkId": "chk_xyz",
      "title": "权限模型说明",
      "repo": "group/repo",
      "url": "https://www.yuque.com/group/repo/auth",
      "snippet": "......",
      "score": 0.92
    }
  ]
}
```

## 9.4 `POST /internal/v1/chat/stream`

用途：

- 基于知识范围执行 grounded chat

请求体：

```json
{
  "userId": "user_123",
  "message": "这个月权限系统有哪些变更",
  "scopeIds": ["conn_123"],
  "skillContext": {
    "skillId": "skill_123",
    "mode": "answer",
    "prompt": "尽量给出明确结论并附依据",
    "runtimeSpec": {
      "instructions": "优先根据内部资料作答"
    }
  },
  "modelProfile": {
    "purpose": "chat_fast",
    "profileId": "profile_chat_fast_default"
  },
  "trace": {
    "runId": "",
    "messageId": "msg_123"
  }
}
```

SSE 事件：

### `retrieval`

```json
{
  "type": "retrieval",
  "traceId": "trace_123",
  "sources": [
    {
      "type": "knowledge_base",
      "provider": "yuque",
      "scopeId": "conn_123",
      "documentId": "doc_abc",
      "chunkId": "chk_xyz",
      "title": "权限模型说明",
      "repo": "group/repo",
      "url": "https://www.yuque.com/group/repo/auth",
      "snippet": "......",
      "score": 0.92
    }
  ]
}
```

### `chunk`

```json
{
  "type": "chunk",
  "traceId": "trace_123",
  "content": "根据知识库资料，四月的主要变化包括..."
}
```

### `done`

```json
{
  "type": "done",
  "traceId": "trace_123",
  "answer": "根据知识库资料，四月的主要变化包括...",
  "sources": [
    {
      "type": "knowledge_base",
      "provider": "yuque",
      "scopeId": "conn_123",
      "documentId": "doc_abc",
      "chunkId": "chk_xyz",
      "title": "权限模型说明",
      "repo": "group/repo",
      "url": "https://www.yuque.com/group/repo/auth",
      "snippet": "......",
      "score": 0.92
    }
  ],
  "metrics": {
    "retrieveMs": 180,
    "generateMs": 1220,
    "totalMs": 1450
  }
}
```

### `error`

```json
{
  "type": "error",
  "traceId": "trace_123",
  "error": {
    "code": "MODEL_TIMEOUT",
    "message": "generation timeout",
    "retryable": true
  }
}
```

## 9.5 `POST /internal/v1/report/generate`

用途：

- 为 Run 的 `report writer` 提供统一报告生成能力

请求体：

```json
{
  "userId": "user_123",
  "runId": "run_123",
  "goal": "分析四月权限系统更新对企业客户的影响",
  "mode": "hybrid",
  "skillSnapshot": {
    "snapshotId": "snap_123",
    "installationId": "inst_123",
    "definitionId": "def_123",
    "revisionId": "rev_123",
    "kind": "agent_workflow",
    "title": "产品研究员",
    "description": "偏产品分析",
    "mode": "report",
    "prompt": "更关注业务影响和建议动作",
    "runtimeSpec": {
      "instructions": "报告必须区分内部依据和外部来源"
    }
  },
  "evidences": [
    {
      "provider": "yuque",
      "connectionId": "conn_123",
      "documentId": "doc_abc",
      "chunkId": "chk_xyz",
      "title": "权限模型说明",
      "repo": "group/repo",
      "url": "https://www.yuque.com/group/repo/auth",
      "snippet": "......",
      "body": "",
      "score": 0.92
    },
    {
      "provider": "web",
      "connectionId": "",
      "documentId": "",
      "chunkId": "",
      "title": "Industry Security Trends",
      "repo": "",
      "url": "https://example.com/security",
      "snippet": "......",
      "body": "",
      "score": 0.74
    }
  ],
  "modelProfile": {
    "purpose": "report_writer",
    "profileId": "profile_report_default"
  }
}
```

响应体：

```json
{
  "traceId": "trace_456",
  "summary": "本次更新整体提升了权限可控性，但带来了迁移与运维成本。",
  "reportMarkdown": "# 结论摘要\n...\n## 关键发现\n...",
  "sources": [
    {
      "type": "knowledge_base",
      "provider": "yuque",
      "scopeId": "conn_123",
      "documentId": "doc_abc",
      "chunkId": "chk_xyz",
      "title": "权限模型说明",
      "repo": "group/repo",
      "url": "https://www.yuque.com/group/repo/auth",
      "snippet": "......",
      "score": 0.92
    }
  ],
  "metrics": {
    "retrieveMs": 0,
    "generateMs": 2330,
    "totalMs": 2410
  }
}
```

## 9.6 `POST /internal/v1/skills/upsert`

用途：

- 同步 Go 主存储里的 skill 镜像到 LLM

请求体：

```json
{
  "skillId": "skill_123",
  "userId": "user_123",
  "definitionId": "def_123",
  "revisionId": "rev_123",
  "kind": "chat_profile",
  "title": "产品总结助手",
  "description": "偏总结风格",
  "mode": "summary",
  "prompt": "强调结论、关键变更和建议动作",
  "runtimeSpec": {
    "instructions": "输出三到五个要点"
  },
  "enabled": true,
  "updatedAt": "2026-04-25T09:00:00Z"
}
```

响应体：

```json
{
  "status": "ok",
  "skillId": "skill_123"
}
```

## 9.7 `DELETE /internal/v1/skills/{skillId}`

请求体：

```json
{
  "userId": "user_123"
}
```

响应体：

```json
{
  "status": "ok",
  "skillId": "skill_123"
}
```

## 9.8 `GET /internal/v1/health`

响应体：

```json
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "vectorIndex": "ok",
    "embeddingModel": "ok",
    "generator": "ok"
  }
}
```

## 10. 与当前接口的映射关系

为了平滑演进，可在过渡期保留当前内部接口，并在 Go 侧逐步切到新版本：

| 当前接口 | 目标接口 |
| :--- | :--- |
| `/internal/knowledge/upsert` | `/internal/v1/index/upsert-batch` |
| `/internal/knowledge/{user}/{connection}` | `/internal/v1/index/scopes/{scopeId}` |
| `/internal/chat/stream` | `/internal/v1/chat/stream` |
| `/internal/skills/upsert` | `/internal/v1/skills/upsert` |
| `/internal/skills/{user}/{skill}` | `/internal/v1/skills/{skillId}` |

说明：

- `sync-source` 仍可保留作为过渡接口
- 目标态更推荐由 Go 统一拉取知识，再调用 `upsert-batch`

## 11. 推荐实施顺序

1. 先落 `llm` schema 与基础表
2. 实现 `index/upsert-batch`
3. 实现 `retrieve`
4. 实现 `chat/stream`
5. 实现 `report/generate`
6. 最后替换 Go 的 report writer 调用路径

## 12. 结论

LLM 子系统的数据库设计，核心不是“存一份知识库副本”，而是：

- 存可重建的知识表示
- 存可检索的 chunk
- 存可追溯的 trace

LLM 子系统的 API 设计，核心不是“暴露一个聊天接口”，而是：

- 接住 Go 编排出来的知识与证据
- 返回 grounded chat 与 grounded report

因此，目标态下的 LLM 数据与 API 都必须围绕 **Evidence Reasoning Engine** 来设计，而不是围绕“一个 Python RAG 服务”来设计。
