# Knowvia LLM

`llm/` 是 Knowvia 的内部 `Evidence Reasoning Engine`。

它现在只服务内部编排链路，不再提供旧版公开登录、公开聊天和调试 WebUI。新的实现按照 `docs/llm-technical-design.md`、`docs/llm-database-and-api-design.md` 和 `docs/llm-module-and-class-design.md` 重建，目标是把知识索引、检索、流式回答和报告生成统一到一套认知层里。

## 当前实现

- 新代码主包：`llm/qqa_llm/`
- HTTP 入口：`llm/qqa_llm/main.py`
- 兼容入口：`llm/server.py`
- 存储后端：文件后端（`data/internal`）或 `Postgres + pgvector`
- 检索链路：`lexical + vector + rerank`
- 生成后端：`OpenAI-compatible / Ollama / fallback`
- Embedding 后端：`OpenAI-compatible / BCEmbedding / hash fallback`
- Rerank 后端：`BCEmbedding / embedding-backed / simple fallback`
- 模型选择：支持按 `chat_fast / chat_quality / report_writer` profile 解析

## 关键路由

新路由：

- `GET /internal/v1/health`
- `GET /internal/v1/model-profiles`
- `GET /internal/v1/model-profiles/{profile_id}`
- `POST /internal/v1/model-profiles/upsert`
- `DELETE /internal/v1/model-profiles/{profile_id}`
- `GET /internal/v1/request-traces`
- `GET /internal/v1/request-traces/{trace_id}`
- `GET /internal/v1/eval/quality/suites`
- `POST /internal/v1/eval/quality/run`
- `POST /internal/v1/index/upsert-batch`
- `GET /internal/v1/index/jobs/{job_id}`
- `POST /internal/v1/index/sync-source`
- `POST /internal/v1/retrieve`
- `POST /internal/v1/chat/stream`
- `POST /internal/v1/report/generate`
- `POST /internal/v1/skills/upsert`
- `DELETE /internal/v1/skills/{skill_id}`

兼容路由：

- `POST /internal/skills/upsert`
- `DELETE /internal/skills/{user_id}/{skill_id}`
- `POST /internal/knowledge/upsert`
- `POST /internal/knowledge/sync-source`
- `DELETE /internal/knowledge/{user_id}/{connection_id}`
- `POST /internal/chat/stream`

## 索引任务模式

`index/upsert-batch` 和 `index/sync-source` 现在支持两种执行方式：

- 默认同步：保持兼容，接口会直接返回 `document_count / chunk_count / changed_documents`
- 显式异步：请求体传 `runAsync: true`，接口先返回 `status=queued` 和 `job_id`，再通过 `GET /internal/v1/index/jobs/{job_id}` 轮询状态

异步模式的典型返回形状会是：

```json
{
  "job_id": "job_xxx",
  "status": "queued",
  "execution_mode": "async",
  "document_count": 0,
  "chunk_count": 0
}
```

## 启动

```bash
cd llm
./run_server.sh
```

`run_server.sh` 会自动加载 `llm/.env`。当前默认配置已经指向你本机的 PostgreSQL 和本地 Ollama：

```bash
QQA_INDEX_BACKEND=postgres
QQA_POSTGRES_DSN=postgresql://chenhaibin@127.0.0.1:5432/quickque_agent
QQA_GENERATOR_BACKEND=ollama
QQA_DEFAULT_CHAT_TEMPERATURE=0.05
QQA_DEFAULT_CHAT_QUALITY_TEMPERATURE=0.05
QQA_DEFAULT_REPORT_TEMPERATURE=0.1
OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_MODEL=gemma3n:e4b
```

上面三条 `QQA_DEFAULT_*_TEMPERATURE` 只作用在**生成阶段**：

- `chat_fast` / `chat_quality`：控制知识库问答回答时的随机性
- `report_writer`：控制报告生成时的随机性

它们**不作用于检索阶段**。当前的 embedding / lexical / vector / rerank 检索链本身没有 `temperature` 参数。

如果你后面要切回 file backend、切到别的数据库，或者换本地模型，只需要改 `llm/.env`，不需要再手敲启动参数。

启动时，`llm` 还会自动执行一次 trace retention cleanup：

- `chat / retrieve / evidence_merge` traces 默认保留 `30` 天
- `report / quality_benchmark` traces 默认保留 `90` 天

可以通过下面两个环境变量调整：

```bash
QQA_TRACE_RETENTION_DAYS=30
QQA_REPORT_TRACE_RETENTION_DAYS=90
```

如果要显式切到 `Postgres + pgvector`，至少需要设置：

```bash
QQA_INDEX_BACKEND=postgres
QQA_POSTGRES_DSN=postgresql://user:pass@127.0.0.1:5432/quickque
QQA_POSTGRES_SCHEMA=qqa_llm
```

第一次启动时，`llm` 会自动创建 `vector` 扩展、schema 和索引表。

如果希望启用真实 embedding + rerank，而不是本地 fallback，建议至少设置：

```bash
QQA_EMBEDDING_BACKEND=auto
QQA_EMBEDDING_MODEL=text-embedding-3-small
QQA_RERANK_BACKEND=auto
OPENAI_API_BASE=https://your-openai-compatible-endpoint/v1
OPENAI_API_KEY=sk-xxx
OPENAI_MODEL=gpt-4.1-mini
OPENAI_CHAT_QUALITY_MODEL=gpt-4.1
OPENAI_REPORT_MODEL=gpt-4.1
```

如果你更希望走接近 `yuque-rag` 的本地中文检索路线，也可以显式切到 `BCEmbedding`：

```bash
QQA_EMBEDDING_BACKEND=bce
QQA_BCE_EMBEDDING_MODEL=maidalun1020/bce-embedding-base_v1
QQA_RERANK_BACKEND=bce
QQA_BCE_RERANK_MODEL=maidalun1020/bce-reranker-base_v1
QQA_EMBEDDING_DIM=768
```

这条路线参考了 `yuque-rag` 里 `BCEmbeddingWrapper + RerankRetriever` 的实现，但保留了 QuickQue 现在的系统边界：

- Go 继续负责同步编排和 run/report 主链
- Python `llm` 继续只做索引、检索、重排和生成
- 混合检索仍然沿用我们从 `YuqueSyncPlatform` 借来的 `lexical + vector + fusion` 结构

当前的检索实现思路参考了两个开源后端：

- `YuqueSyncPlatform`：借鉴 `关键词检索 + 向量检索 + RRF 融合` 的混合检索思路
- `yuque-rag`：借鉴 `BCEmbedding 向量初筛 + BCE rerank` 的两段式 RAG 结构

## 测试

```bash
cd llm
./venv/bin/python -m unittest tests.test_internal_api tests.test_backend_selection
```

如果希望用固定夹具回归检索和报告质量，可以运行：

```bash
cd llm
arch -x86_64 ./venv/bin/python -m unittest tests.test_quality_regression
```

这套质量回归不是只检查“接口能不能返回”，而是会验证：

- 混合检索是否还能命中预期文档主题
- `evidence_merge` 是否还能稳定产出去重、主题聚类、provider summary 和冲突提示
- `retrieve` 的 trace 是否记录了 `retrieved / used` chunk
- `report.generate` 是否还能产出稳定的 section、关键词和 grounded sources
- 在 fallback、本地 BCE、或 OpenAI-compatible 路线切换后，核心输出是否发生了明显退化

`quality_runner` 现在还会给每个 case 产出结构化评分和 suite 级汇总，包含：

- retrieval/report 每类 case 的平均分
- 单 case score
- 分项 metric，例如 `title_match / keyword_recall / source_grounding`
- 一份可直接落日志或 artifact 的 Markdown benchmark 报告

如果希望把这套 benchmark 当作 `llm` 的正式内部运维能力来跑，而不是只从 unittest 里执行，也可以直接调用：

```bash
curl -sS \
  -H "Authorization: Bearer $QQA_PYTHON_PROXY_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST http://127.0.0.1:18000/internal/v1/eval/quality/run \
  -d '{
    "userId": "benchmark-user",
    "suiteName": "builtin",
    "cleanup": true
  }'
```

这个接口会：

- 给 benchmark fixture 建立临时 scope
- 运行 retrieval/report 质量评估
- 返回 case 级得分、分类平均分和 Markdown benchmark 报告
- 默认在完成后清理临时 scope，避免污染真实知识库

如果本地已经有可用的 Postgres/pgvector，也可以跑可选集成测试：

```bash
cd llm
QQA_POSTGRES_TEST_DSN=postgresql://user@127.0.0.1:5432/quickque_agent \
  arch -x86_64 ./venv/bin/python -m unittest tests.test_postgres_backend
```

如果本机已经缓存好了 BCE 模型，也可以跑一条可选 smoke test，确认 `BCEmbedding + BCE rerank` 真实后端能够工作：

```bash
cd llm
arch -x86_64 ./venv/bin/python -m unittest tests.test_bce_smoke
```

如果想把“真实后端质量基线”也固化成一条正式回归，而不只是 smoke test，可以显式开启：

`BCEmbedding` 路线：

```bash
cd llm
QQA_RUN_REAL_BACKEND_BENCHMARK=1 \
QQA_REAL_BACKEND_MODE=bce \
arch -x86_64 ./venv/bin/python -m unittest tests.test_real_backend_benchmark
```

如果同时配置了 `QQA_POSTGRES_TEST_DSN`，这条回归会自动切到 `Postgres + pgvector` 路径；否则默认走文件后端。

`OpenAI-compatible` 路线：

```bash
cd llm
QQA_RUN_REAL_BACKEND_BENCHMARK=1 \
QQA_REAL_BACKEND_MODE=openai \
OPENAI_API_BASE=https://your-openai-compatible-endpoint/v1 \
OPENAI_API_KEY=sk-xxx \
OPENAI_MODEL=gpt-4.1-mini \
OPENAI_CHAT_QUALITY_MODEL=gpt-4.1 \
OPENAI_REPORT_MODEL=gpt-4.1 \
arch -x86_64 ./venv/bin/python -m unittest tests.test_real_backend_benchmark
```

这条集成回归会直接调用 `POST /internal/v1/eval/quality/run`，验证：

- retrieval / evidence_merge / report 三段 benchmark 全通过
- `backend_snapshot` 与你选择的真实后端一致
- `quality_benchmark` trace 已经正确落库
