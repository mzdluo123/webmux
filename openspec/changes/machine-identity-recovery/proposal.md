# Proposal

## Why

本机丢失 Relay 绑定后，服务端仍保存账号的旧宿主，普通配对被拒绝；当前未配对界面也没有恢复入口。
增加独立持久机器 ID，使同账号同机器重新授权后能够恢复绑定，并保留不同机器替换所需的明确确认。

## What Changes

- 本机生成随机机器 ID，保存在独立受控文件中；解绑或丢失 Relay 设置不删除该 ID。
- 配对请求携带机器 ID；它只用于判重，不能替代 GitHub 数字账号授权或邀请检查。
- 同账号、非空且相同机器 ID 允许自动换钥，保留原 host ID 与公共域名。
- 不同机器或没有机器 ID 的旧记录仍要求显式替换；未配对页面提供这一恢复入口。
- 换钥递增绑定代际，并复用既有旧连接、登录授权及派生访问撤销流程。
- 双端固定共享协议版本，先部署服务端再安装客户端；已有旧绑定首次显式迁移。
- Relay 账号控制台增加所有者确认删除机器：撤销访问并移出列表，保留固定域名和必要撤权记录；本次部署不操作当前生产绑定。

## Capabilities

### New Capabilities

无新增能力边界，复用现有 Relay 能力。

### Modified Capabilities

- `official-relay-client`：独立机器身份持久化、未配对恢复入口和配对行为。
- `official-relay-service`：同账号同机器自动恢复与旧记录显式迁移边界。

## Impact

影响公开协议子模块、`internal/relaysetting`、`internal/relay`、`cmd/webmux`、Relay 设置页和存储清单。
配套 Relay 仓库负责 PostgreSQL 迁移及事务授权；两仓不得互相导入 internal 包。
用户故事 US-A09、US-A10 和部署说明同步；不改变公网启用确认、Token、Origin 或 TLS 策略。
