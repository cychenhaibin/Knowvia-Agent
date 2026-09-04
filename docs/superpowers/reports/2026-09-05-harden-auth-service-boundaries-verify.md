# 验证报告：harden-auth-service-boundaries

## 摘要

| 维度 | 结果 |
|---|---|
| 完整性 | PASS：9/9 任务完成；7/7 requirements 有实现与测试证据 |
| 正确性 | PASS：12/12 scenarios 已由回归测试或故障注入测试覆盖 |
| 一致性 | PASS：实现符合 `proposal.md`、`design.md` 与 delta spec；未发现漂移 |

最终结论：完整验证通过，无 CRITICAL、IMPORTANT、WARNING 或 SUGGESTION 项，可进入归档确认。

## 需求与实现映射

1. 认证生命周期：`server/internal/auth/service_session.go:19` 原子消费 refresh session，`:49` 校验 access session 撤销状态，`:65` 使用配置 TTL 签发，`:100` 写入独立 `jti`/`token_use`；Postgres 原子谓词位于 `server/internal/adapters/store/sql/chat.sql:20`。回归覆盖见 `server/internal/auth/service_session_test.go:67`、`:103`、`:118`、`:136`。
2. 生产配置 fail-closed：`server/internal/config/config.go:60` 仅在显式 development 模式提供开发默认值，`:79` 默认监听 loopback，`:114` 拒绝空/已知 JWT secret 与隐式开发账号。测试见 `server/internal/config/config_test.go:15`、`:29`、`:39`。
3. 文件 scope confinement：`llm/qqa_llm/storage/scope_repo.py:95` 拒绝绝对路径、分隔符、`.`/`..`，并使用解析后的 base containment 阻止 symlink escape。测试见 `llm/tests/test_scope_repo_security.py:21`、`:41`。
4. 外部归档边界：`server/internal/skillimport/github.go:16` 对下载和每次重定向重复执行 HTTPS/GitHub host 校验，`:57` 只接受 canonical repo URL；`server/internal/skillimport/archive_import.go:107` 限制成员数、累计展开大小和压缩比。测试见 `server/internal/skillimport/service_test.go:21`、`:38`、`:60`、`:72`、`:82`。
5. 流与 Run detail 完整性：`server/internal/adapters/provider/pythonproxy/forward.go:137` 解析 SSE，`:189` 仅在 done 返回 trace/metrics，`:215` 拒绝无 done 的 EOF；`server/internal/run/service_access.go:42` 逐项传播 steps/artifacts/sources 错误。测试见 `server/internal/adapters/provider/pythonproxy/client_test.go:182`、`:198` 和 `server/internal/run/service_test.go:43`。
6. 文件状态与 chunk 边界：`llm/qqa_llm/storage/file_repo.py:21` 通过同目录临时文件、fsync 与 replace 原子写；`llm/qqa_llm/storage/index_job_repo.py:11`、`llm/qqa_llm/storage/scope_repo.py:14` 用进程内锁串行化；`llm/qqa_llm/ingest/chunk.py:25` 强制精确 chunk 上限并处理零 overlap。测试见 `llm/tests/test_file_storage_and_chunking.py:30`、`:43`、`:79`、`:85`、`:90`。
7. Redis 失败转移：`server/internal/adapters/queue/redis/support.go:21`、`:32` 使用 Lua 原子完成 processing→retry/failed；调用与结果校验在 `server/internal/adapters/queue/redis/retry.go:80`、`:122`。故障注入覆盖见 `server/internal/adapters/queue/redis/dispatcher_test.go:324`。

## 验证证据

- Go：`env -u GOROOT go build ./...`、`go test -count=1 ./...`、`go vet ./...`、`go test -race -count=1 ./...` 全部退出码 0。
- Python：arm64 Python 3.12 隔离环境运行 `python -m compileall -q qqa_llm tests` 与 `python -m unittest discover -s tests -v`；31 个测试通过，7 个依赖/外部环境测试跳过。
- 规格与格式：`openspec validate harden-auth-service-boundaries`、`git diff --check 7e4f1fc8229650a9a6fd401ef0343d92ea83acda...HEAD` 退出码 0；变更 Go 文件经 `gofmt -l` 检查无输出。
- 安全与边界：逐项对照本次 diff、delta spec 与上述实现/回归测试，未发现新增硬编码生产密钥、unsafe 操作、公开 API 或数据库 schema 变更。
- 代码审查：`.comet.yaml` 的 `review_mode` 为 `off`，按 Hotfix 配置跳过自动审查；本验证仍完成正确性、安全和边界条件的实现对照。
- 验证修复：首次非缓存全量测试暴露 Redis 去重测试在业务回调与清理完成之间抢跑；`server/internal/adapters/queue/redis/dispatcher_test.go:78` 已改为受控阻塞并等待 queue/dedup 清理。修复前 30 次重复出现 6 次失败，修复后 100 次重复、包级普通测试和 race 测试均通过。

## 非阻塞环境说明

- Python 跳过项为可选 BCEmbedding（2）、未配置 Postgres DSN（4）和未启用真实后端 benchmark（1）。
- Go 的实时 Postgres contract 依赖 `QQA_TEST_POSTGRES_DSN`，当前环境未配置；SQL 原子条件已静态核对，store contract 接口、生成代码编译、全量 Go 测试与 race 检查均通过。
- `docs/superpowers/specs/2026-09-04-fluxa-login-design.md` 属于其他 change；本 Hotfix 未关联独立 Design Doc，以当前 `design.md` 和 delta spec 为设计依据。

## 问题分级

- CRITICAL：无。
- IMPORTANT：无。
- WARNING：无。
- SUGGESTION：无。
