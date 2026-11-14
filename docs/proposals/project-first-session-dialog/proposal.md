> 设计提案，尚未实施。为遵守在途变更目录数量上限，本方案暂存于文档区；实施前需转为正式 OpenSpec 变更。

## Why

目前新建会话把目录、仓库、工作树与启动命令并排暴露，用户需要先理解运行环境才能开始任务。改为 T3 Code 式的项目列表与逐层选择，沿用 WebMux 的中性色、线条和绿色主操作，让用户先确定项目，再启动会话。

## What Changes

- 新建会话弹窗先显示可搜索的项目列表，已有项目显示名称、路径及不可用原因。
- 添加项目提供「新建项目」「本地文件夹」「Git URL」「GitHub 仓库」四个来源，在同一弹窗内逐层前进和返回。
- 普通非 Git 文件夹也作为持久项目；Git 项目继续复用仓库和工作树能力。
- 项目选定后显示会话启动页；工作位置默认为项目目录，工作树作为项目内的可选项。
- 保留启动配置、自定义命令、首条消息、分屏及工作树失败恢复能力。
- 新增项目创建、克隆和 GitHub 仓库发现能力，保持现有仓库登记接口不自动 init/clone 的约定。

## Capabilities

### New Capabilities

- `project-catalog`: 持久项目目录、来源接入、创建与克隆、非 Git 文件夹及 GitHub 仓库发现。

### Modified Capabilities

- `new-session-dialog`: 项目优先、分层列表、项目内工作位置与会话启动、返回保留草稿。

## Impact

- 前端：NewSessionDialog、LocationPicker、DirectoryPicker、LaunchPicker、AppShell，以及相关组件测试和 Storybook。
- 服务端：项目清单与接入操作，复用 workspace 的仓库归一化和 files 的目录能力；由 cmd 构造服务并注入 api。
- 契约：新增项目接口时同步 internal/contract、internal/wire、schemas.ts 和 wire 快照，不改变现有 sessions 创建字段互斥规则。
- 不重做整个侧栏，不迁移 QuickStart 页面；此次统一范围为新建会话弹窗及其上下文入口。
- 本次交付为设计与实施计划，尚未改变产品行为；用户故事在实施时更新。

## Review Artifacts

- [弹窗布局与交互设计](design.md)
- [项目目录行为草案](specs/project-catalog/spec.md)
- [会话弹窗变更草案](specs/new-session-dialog/spec.md)
- [实施与验收任务](tasks.md)
