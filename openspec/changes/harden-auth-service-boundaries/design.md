## 修复设计

### 认证与配置

`issueSession` 生成独立 access/refresh JWT，分别写入 `typ`、`jti` 和配置 TTL；session 的有效期与 refresh TTL 对齐。`Authenticate` 解析并验证 access 类型、exp，并通过 access token 查询 session，拒绝已撤销或过期会话。密码改为 bcrypt（兼容旧 SHA-256：成功登录后升级哈希）。默认配置改为 loopback；已知 secret/默认管理员仅在显式开发模式允许，否则启动配置校验失败。

### 文件与导入边界

ScopeRepository 统一通过 `resolve_safe_path` 验证单一非空路径组件，并确认解析路径严格位于 base_dir 下。GitHub URL 仅接受 `https://github.com/<owner>/<repo>`，下载客户端禁用自动跳转并对每个跳转目标重复校验。ZIP 在读取成员前累计检查成员数、展开字节数和压缩比。

### 可靠性与完整性

Redis 任务失败处理使用 Lua 原子脚本完成 processing→retry/failed 的转移。流式代理跟踪 done 并映射 trace/metrics；Run detail 对 steps/artifacts/sources 返回首个读取错误；chunk builder 将超长段落拆分为不超过配置上限的片段。file backend 的 job JSON 采用锁内 read-modify-write + 临时文件原子替换，scope 快照采用同目录临时版本后替换。

### 验证策略

每个任务先加入最小 RED 回归测试，再实现最小修复并运行定向 Go/Python 测试；完成后运行 `go test ./...`、`go vet ./...`、`go test -race ./...` 和 Python compile/test 可用性检查。Python 本机依赖架构不匹配时保留准确阻塞证据，并使用可运行的纯模块测试覆盖路径与 chunk 行为。
