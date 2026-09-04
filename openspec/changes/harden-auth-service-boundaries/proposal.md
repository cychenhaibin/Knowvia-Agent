## Why

评审发现认证、文件存储、外部导入和流式代理存在可直接利用的安全漏洞，以及会造成任务/数据丢失的边界错误。当前项目默认配置即可对外监听，且关键路径缺少生命周期、路径和资源上限保护，需在发布前收敛为可验证的安全行为。

## What Changes

- 为 access/refresh token 写入不同类型、JTI 和过期时间；认证时校验签名、过期、类型及会话撤销状态，注销后 access token 立即失效。
- 用带盐的慢哈希保存密码，并拒绝生产环境使用已知 JWT secret、默认管理员密码和公开监听地址。
- 对 Python file backend 的 user/scope 标识做路径组件校验和 base-dir containment 校验，阻断穿越与越界删除。
- 将 GitHub 导入限制为 HTTPS、GitHub 主机及安全重定向，拒绝私网/本地地址。
- 为 ZIP 导入增加成员数、累计展开大小和压缩比上限。
- Redis worker 在 retry/failure 转移前后保持任务可恢复，避免 processing 任务永久丢失。
- Go Python 流式代理在未收到 done 事件时返回不完整流错误并保留 trace/metrics；Run detail 子资源读取错误向上返回；长段落按 chunk_size 切分。
- file backend 的 job/scope 写入增加进程内互斥和原子替换，避免并发覆盖或读取半写文件。

## Capabilities

### New Capabilities

- `service-boundary-hardening`: 认证生命周期、输入路径、外部导入、资源上限与流式完整性约束。

### Modified Capabilities

无。仓库当前没有已登记的主 spec；本次仅修复既有行为，不新增公开 capability。

## Impact

影响 Go auth/config、auth store、skill import、Redis queue、run/python proxy 服务，以及 Python scope/job storage/chunking。会新增内部 access-session 查询和配置校验错误，但不改变 HTTP 路由形状；数据库 schema 不变。Postgres 密钥加密、Redis 跨进程 EventBroker 与 Go/Python 跨存储 outbox 需要新的密钥/事件/补偿协议，记录为后续独立架构 change。

## RED Evidence

`env -u GOROOT /usr/local/bin/go test ./internal/auth -run 'Test(IssueSessionUsesConfiguredTTLsAndDistinctPurposes|RefreshTokenCannotAuthenticateAsAccessToken|LogoutInvalidatesAccessToken|AuthenticateRejectsExpiredAccessToken)'` 当前失败：access/refresh 无 token purpose，refresh token 可被当作 access 使用，注销后 access 仍可认证。过期 JWT 的解析校验用例已通过，说明现有库校验可复用，缺陷集中在签发与会话撤销边界。
