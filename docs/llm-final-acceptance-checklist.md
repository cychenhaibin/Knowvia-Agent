# Knowvia LLM 最终验收清单

## 1. 文档目的

本文用于回答一个非常具体的问题：

> 现在 `llm/` 的具体实现，距离既定技术方案设计还差什么？

判断基准来自以下三份设计文档：

- [LLM 技术方案](./llm-technical-design.md)
- [LLM 数据库与 API 设计](./llm-database-and-api-design.md)
- [LLM 模块与类设计](./llm-module-and-class-design.md)

本文不再描述“理想目标应该是什么”，而是直接给出：

1. 设计项是否已经实现
2. 是否已经被本地回归验证
3. 是否仍依赖外部环境完成最终验收
4. 哪些属于可选生产强化，而不是设计缺口

## 2. 总体结论

截至当前实现，`llm/` 已经满足设计文档中的主体能力要求。

更准确地说：

- **架构与主链能力：已完成**
- **数据库、检索、报告、trace、benchmark：已完成**
- **真实本地 BCE + pgvector 路径：已完成验证**
- **OpenAI-compatible 真实路径：代码已完成，是否通过取决于部署时凭证和 endpoint**

因此，当前 `llm/` 剩余的主要事项已经不再是“设计缺口”，而是：

- 外部环境验收
- 更严格的生产化强化

## 3. 验收状态约定

- `已完成`：代码与设计对齐，且主链已接通
- `已验证`：已通过本地回归或真实环境测试
- `待环境验收`：代码已完成，但需要真实外部依赖才能最终确认
- `可选强化`：不属于设计缺口，只是更强的生产形态

## 4. 架构与边界验收

### 4.1 Go / Python 职责边界

- 状态：`已完成`
- 设计要求：
  - Go 负责产品编排
  - Python `llm` 负责索引、检索、重排、合成和生成
- 现状：
  - Go 主链通过内部 API 调用 `llm`
  - `llm` 不承担公开登录、公开聊天、知识源连接管理
- 对应实现：
  - `llm/qqaat_llm/api/internal.py`
  - `server/internal/provider/python_proxy.go`
  - `server/internal/tools/knowledge.go`
  - `server/internal/tools/evidence.go`
  - `server/internal/tools/report.go`

### 4.2 模块分层

- 状态：`已完成`
- 设计要求：
  - `api / core / ingest / index / retrieve / inference / prompt / services / storage`
- 对应实现：
  - [llm/qqaat_llm](../llm/qqaat_llm)

## 5. 数据与存储验收

### 5.1 Postgres + pgvector 主实现

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - 使用独立 schema
  - 文档、chunk、trace、index jobs 可持久化
  - `pgvector` 承载向量检索
- 对应实现：
  - `llm/qqaat_llm/storage/db.py`
  - `llm/qqaat_llm/index/pgvector_index.py`
  - `llm/tests/test_postgres_backend.py`

### 5.2 版本化文档模型

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - `knowledge_documents`
  - `knowledge_document_versions`
  - `latest_version_id`
  - 软删除与版本追踪
- 对应实现：
  - `llm/qqaat_llm/storage/db.py`
  - `llm/qqaat_llm/services/indexing_service.py`

### 5.3 skill mirrors / model profiles / request traces

- 状态：`已完成`
- 本地验证：`已验证`
- 对应实现：
  - `llm/qqaat_llm/storage/skill_repo.py`
  - `llm/qqaat_llm/storage/model_profile_repo.py`
  - `llm/qqaat_llm/storage/trace_repo.py`
  - `llm/qqaat_llm/services/model_profile_service.py`

## 6. 索引与检索验收

### 6.1 索引输入与同步模式

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - `merge / replace`
  - `changed_documents`
  - scope 级删除
- 对应实现：
  - `llm/qqaat_llm/api/schemas.py`
  - `llm/qqaat_llm/services/indexing_service.py`

### 6.2 异步索引任务

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - `index_jobs`
  - job 状态查询
  - scope 串行
  - 中断恢复
- 对应实现：
  - `llm/qqaat_llm/services/indexing_service.py`
  - `llm/qqaat_llm/storage/index_job_repo.py`
  - `llm/tests/test_internal_api.py`

说明：

当前实现已经支持：

- `runAsync: true`
- 启动恢复 `queued/running` job
- job 持久化

它满足设计目标；如果要继续提升，只剩“独立 worker 进程化”，那属于生产强化项，不再是设计缺口。

### 6.3 混合检索

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - lexical recall
  - vector recall
  - fusion
  - rerank
- 对应实现：
  - `llm/qqaat_llm/retrieve/pipeline.py`
  - `llm/qqaat_llm/retrieve/lexical.py`
  - `llm/qqaat_llm/retrieve/vector.py`
  - `llm/qqaat_llm/retrieve/fusion.py`
  - `llm/qqaat_llm/retrieve/rerank.py`

说明：

这部分实现明确参考了两个开源项目的后端思路：

- `YuqueSyncPlatform`：关键词检索 + 向量检索 + 融合
- `yuque-rag`：BCE embedding + BCE rerank 两段式结构

## 7. 模型与推理验收

### 7.1 model profile 路由

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - 支持不同 purpose 的 profile
  - `chat_fast / chat_quality / report_writer`
  - 可查询、可更新、可删除
- 对应实现：
  - `llm/qqaat_llm/services/model_profile_service.py`
  - `llm/qqaat_llm/inference/generator.py`
  - `llm/qqaat_llm/api/internal.py`

### 7.2 embedding / rerank / generator 多后端

- 状态：`已完成`
- 本地验证：
  - BCE：`已验证`
  - file fallback：`已验证`
  - Postgres + BCE：`已验证`
  - OpenAI-compatible：`待环境验收`
- 对应实现：
  - `llm/qqaat_llm/inference/embeddings.py`
  - `llm/qqaat_llm/inference/reranker.py`
  - `llm/qqaat_llm/inference/generator.py`
  - `llm/tests/test_bce_smoke.py`
  - `llm/tests/test_real_backend_benchmark.py`

说明：

真实 `BCEmbedding + BCE rerank + pgvector` 路径已经本地跑通。  
`OpenAI-compatible` 路径的代码和测试入口已完成，但最终通过与否取决于真实 `OPENAI_API_BASE / OPENAI_API_KEY / model` 配置。

## 8. 主链能力验收

### 8.1 Knowledge Chat

- 状态：`已完成`
- 本地验证：`已验证`
- 对应实现：
  - `llm/qqaat_llm/services/chat_service.py`
  - `llm/qqaat_llm/api/internal.py`

### 8.2 Evidence Merge

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - 去重
  - 分组
  - 冲突提示
  - provider summary
  - richer diagnostics
- 对应实现：
  - `llm/qqaat_llm/services/evidence_service.py`

### 8.3 Report Generate

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - `outline -> draft -> final`
  - markdown report
  - summary
  - structured sections
- 对应实现：
  - `llm/qqaat_llm/services/report_service.py`
  - `llm/qqaat_llm/prompt/report_builder.py`

## 9. Trace 与可观测性验收

### 9.1 request traces

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - `trace_id`
  - `request_type`
  - `run_id / message_id / skill_id / model_profile_id`
  - `output_preview`
  - retrieval diagnostics
- 对应实现：
  - `llm/qqaat_llm/storage/trace_repo.py`
  - `llm/qqaat_llm/api/internal.py`
  - `llm/qqaat_llm/services/chat_service.py`
  - `llm/qqaat_llm/services/report_service.py`

### 9.2 trace retention

- 状态：`已完成`
- 本地验证：`已验证`
- 设计要求：
  - 普通 trace 默认 `30` 天
  - report / benchmark trace 更长保留
- 对应实现：
  - `llm/qqaat_llm/storage/trace_repo.py`
  - `llm/qqaat_llm/storage/db.py`
  - `llm/qqaat_llm/main.py`

说明：

当前是“启动时 cleanup”策略，已经满足保留策略本身。  
如果要做成定时后台清理，那属于运维强化，不属于设计缺口。

### 9.3 health / readiness

- 状态：`已完成`
- 本地验证：`已验证`
- 对应实现：
  - `llm/qqaat_llm/api/health.py`

说明：

当前 `health` 已经会返回：

- database
- vectorIndex
- embeddingModel
- generator
- rerankModel
- modelProfiles
- qualityBenchmarks
- degraded_reasons

如果后续要再做“强探活”版本，那是更高标准的生产强化。

## 10. Benchmark 与质量回归验收

### 10.1 固定夹具 benchmark

- 状态：`已完成`
- 本地验证：`已验证`
- 对应实现：
  - `llm/qqaat_llm/eval/quality_runner.py`
  - `llm/qqaat_llm/services/quality_eval_service.py`
  - `llm/tests/test_quality_regression.py`

### 10.2 真实后端 benchmark

- 状态：`已完成`
- 本地验证：
  - BCE + pgvector：`已验证`
  - OpenAI-compatible：`待环境验收`
- 对应实现：
  - `llm/tests/test_real_backend_benchmark.py`

## 11. 仍未完成的事项

严格来说，当前已没有“必须补齐才能算符合设计文档”的功能缺口。

剩余事项只有两类：

### 11.1 外部环境验收

- 用真实 `OpenAI-compatible` 配置跑通 `test_real_backend_benchmark`

这不再是代码设计缺口，而是环境条件问题。

### 11.2 可选生产强化

这些可以继续做，但不属于“设计未实现”：

1. 把当前进程内可恢复 worker 提升为独立 durable worker 进程
2. 把 `health` 再推进成强探活型 readiness
3. 把 trace cleanup 从“启动时清理”升级成周期后台清理

## 12. 最终判断

如果问题是：

> `llm/` 现在离完整技术方案设计还差什么？

那么当前最准确的答案是：

- **离设计实现本身，基本不差了**
- **离更强的生产形态，还可以继续加强**

也就是说：

- `设计完成度`：可以视为 **已完成**
- `本地工程验收`：可以视为 **已完成**
- `外部环境验收`：只剩真实 `OpenAI-compatible` 路径待验证
- `生产强化`：仍有提升空间，但不属于设计缺口
