## 1. 运行时记录（`internal/runtime`）

- [ ] 1.1 实现记录结构（v1）、`<state_dir>/run/` 0700 目录、0600 原子替换写入、读取与格式版本校验。验证：`go test -race ./internal/runtime` 覆盖权限、原子替换、未知版本、损坏 JSON
- [ ] 1.2 实现 `server.lock` 独占锁获取/释放；锁被占用时返回可识别错误。验证：同进程两次获取与子进程持锁的测试
- [ ] 1.3 实现 `ProcessIdentity(pid)`：Linux 读 `/proc/<pid>/stat` starttime 与 `/proc/<pid>/exe`（去 ` (deleted)`），darwin 用 `proc_pidinfo` 启动时间。验证：对自身与已退出子进程的测试在 Linux 与 macOS CI 上通过
- [ ] 1.4 实现 Token 指纹与启动方识别（`WEBMUX_LAUNCHER`、`INVOCATION_ID`、`XPC_SERVICE_NAME`）。验证：表驱动单元测试

## 2. 服务端接线（`cmd/webmux`、`internal/session`）

- [ ] 2.1 子命令分发：无参数/`-` 开头 → serve，`serve`、`version`、`status`、`stop`、`config`，未知子命令退出码 2。验证：`cmd/webmux` 测试覆盖 `webmux --config x`、`webmux serve --listen ...`、`webmux statsu`
- [ ] 2.2 `serve --log-file`：`~/` 展开、`.1` 轮转、0600/0700、fd 1/2 重定向（linux `dup3`/darwin `dup2`）；`--config` 支持 `~/`。验证：子进程测试中关闭父端管道后服务继续写日志并存活
- [ ] 2.3 监听成功后写记录、退出清理后删除；锁被占用时警告并跳过。验证：集成测试启动 `--listen 127.0.0.1:0`，断言记录中的实际端口、SIGTERM 后记录消失、第二实例不覆盖记录
- [ ] 2.4 `sessionEnv` 过滤 `WEBMUX_LAUNCHER`。验证：`internal/session` 测试断言会话环境不含该变量

## 3. 客户端核心与命令（`internal/contract`、`internal/client`）

- [ ] 3.0 新增无依赖的 `internal/contract`：`StatusV1`/`StopV1`/`ConfigV1` 与健康分类。验证：`GOOS=windows go build ./internal/contract` 成功，且 `go list -deps ./internal/contract` 只含标准库

- [ ] 3.1 `Locate`：复用 `config.Load`/`Validate` 规则解析配置，读取并校验记录，返回目标地址、Token 与记录。验证：单元测试覆盖缺失配置、显式路径不存在、失效记录（PID 不存在、启动标识不符）
- [ ] 3.2 `Links`：本机/远程链接、Tailscale 置顶、排除回环/链路本地/停用网卡与 WSL NAT 虚拟网卡、通配无 Token 警告。验证：表驱动测试覆盖 `127.0.0.1`、`localhost`、`[::1]`、`0.0.0.0`、`::`、`:7681`、具体地址，结果不含 Token
- [ ] 3.3 `Health`：3 秒超时、Bearer、禁用代理，分类 `ok/unauthorized/refused/timeout/not-webmux/error`。验证：`httptest` 与真实 `api.New` 处理器（带/不带 Token）
- [ ] 3.4 `status`（人类可读与 `--json` v1、`--reveal-token`、`drift`、`warnings`、退出码 0/1/3）。验证：golden JSON 测试；断言无 `--reveal-token` 时 stdout/stderr 均不含 Token
- [ ] 3.5 `stop`（SIGTERM → `--timeout` → SIGKILL、systemd/launchd 拒绝与 `--force`、幂等、无记录不发信号、`--json`）。验证：以真实 `webmux serve` 子进程测试正常停止、忽略 SIGTERM 的替身进程被强制结束、无记录时不发信号

## 4. 配置命令

- [ ] 4.1 `config path` 与 `config show [--json]`（契约 v1，复用 `config.Load`/`Validate`，不含 Token）。验证：golden JSON 测试覆盖无文件、默认值与自定义预设，断言输出无 Token
- [ ] 4.2 `config validate [--json]`（TOML 行列号、校验错误、不写文件）。验证：语法错误与非法预设样例测试
- [ ] 4.3 `config init [--listen] [--force]`：经新增的 `deploy/embed.go` 嵌入 `deploy/config.example.toml` 为模板、`crypto/rand` 32 字节 base64url Token、0600/0700、已存在拒绝、`--force` 生成 `.bak`、输出只含路径。验证：单元测试断言权限、Token 长度与唯一性、模板其余内容不变、标准输出不含 Token；`config validate` 能通过生成的文件

## 5. 文档与验收

- [ ] 5.1 `docs/architecture.md` 增加“客户端与通讯平面”一节（决策 0 的摘要）；`docs/protocol.md` 增加本机控制面 `status`/`stop`/`config` v1 契约与兼容规则；`docs/deployment.md` 增加 `status`/`stop`/`config init`/`--log-file` 用法。验证：`make format-check` 通过
- [ ] 5.2 运行 `make format-check`、`go vet ./...`、`go test -race ./...`（Linux 与 macOS CI）全部通过
