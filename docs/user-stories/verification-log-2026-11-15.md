# 核对记录：2026-11-15 起

本文记录项目优先创建会话开发的增量验证，未声明全部故事重新验收。
此前的项目目录、本地来源和克隆层证据见 [2026-11-12 起记录](verification-log-2026-11-12.md)。

## 验证边界

- 本记录使用隔离服务、临时状态目录和受控外部进程。
- 后端 API 可用不代表弹窗入口已经接入；界面阶段另记截图与交互证据。
- 实际运行结果与未验证范围分别列出。

## 项目来源第 4 层：GitHub 发现与认证克隆

- 范围：US-B12；GitHub 身份、仓库分页/搜索、私有状态、配置降级与 source=github 克隆。尚未接入弹窗。
- 本地通过：完整 api race；workspace、pathutil、pluginapi、pluginwire、localclient、archtest race 回归；cmd 项目/插件/纯核心及 GitHub 能力回归；Go lint 无告警。
- 前端通过：Svelte/TypeScript 无错误或告警、361 条 wire 契约断言、46 条架构断言及完整 check:arch、lint；格式、文档治理和持久化清单检查通过。
- GitHub 测试全部使用受控 gh 假进程，覆盖缺失、认证/权限失败、异常数据、分页搜索、私有仓库、超时取消、来源 URL 限制及 source 幂等身份；未使用个人账号或真实私有仓库。
- 克隆层审查修复：只有提交结果不确定才冻结账本；预发布失败可重试。registering 在目录发布前持久化，保留重启后的单独登记入口；保留服务用户配置的 SSH 传输。
- 隔离手工验证：独立临时服务实际克隆公开 octocat/Hello-World，操作成功、项目可列出且 README 存在，用时约 1.3 秒；关闭服务后清理自有临时状态。此项不验证 GitHub 私有认证或 UI。
- 能力快照经过 localclient 严格校验；缺少 Git/gh、禁用 workspace、克隆账本损坏均不会使普通目录能力失效，能力探测不会执行外部命令。

## 审查补充：克隆恢复与 gh 兼容

- 运行中的状态转换写入失败会让查询明确不可用，避免 worker 已退出仍显示进行中；提交前的普通写入失败仍允许重新提交。
- 发布前保留临时路径；清理时身份不符则清除所有权路径记录并报告拒绝清理，目标数据保持完整；新增状态转换与临时目录替换回归通过。
- GitHub 显式来源与 admission 检查 gh 的 --no-upstream 支持；旧版可读降级且不会入队。受控测试验证未执行个人账号操作。
- 通过子进程编号环境配置限制协议，保留既有配置且不把限制写入克隆仓库；真实 Git 配置读取和生成的 .git/config 回归通过。

## 项目来源第 5 层：列表与来源界面组件

- 范围：US-B10–US-B12 的界面组件与创建贡献；生产弹窗入口尚未切换，后续层接入会话设置和项目内工作位置。
- 真实组件预览包含项目、空清单、四种来源、GitHub 不可用与登记失败共 9 个场景，全部使用隔离演示数据，不连接个人账号。
- 浏览器通过：新组件 6 条交互回归；现有 NewSessionDialog 47 条回归仍通过。覆盖搜索、来源返回草稿、显式本地登记、配置降级、重开只查询原克隆及单独登记重试。
- Storybook 生产构建成功；9 条布局用例各覆盖 12 个视口/主题/双倍字号组合，通过现有溢出检测器与弹窗边界检查。
- 额外实际浏览：1280×900、768×800、390×844、375×667、320×568、667×375，明暗主题及 6 个来源页面共 72 个组合；Esc 逐层返回和根层关闭通过。
- 类型检查（含测试）、lint、完整 check:arch 与 46 条架构断言通过；未增大任何基线。
- [桌面项目列表](../../web/test/evidence/project-picker/desktop-light-projects.png)、[平板来源列表](../../web/test/evidence/project-picker/tablet-light-sources.png)、[手机 GitHub 来源](../../web/test/evidence/project-picker/phone-light-github.png) 与对应深色图片已保存。
- [手机正常速度交互 GIF](../../web/test/evidence/project-picker/phone-light-navigation.gif) 为 390×844 浅色演示，含四来源进入、输入、返回及选择成功；107 帧，约 13.4 秒。
- 局限：无真机验证；无头浏览器未验证真实鼠标悬停和触屏常驻动作。GIF 选择后的结果为独立组件预览，并非生产会话设置。

## 来源组件审查补充：异步选择与重连

- 克隆完成后的项目读取带观察代次；用户停止跟踪或选择其他项目后，旧响应不能再触发选择。停止跟踪只清除浏览器跟踪记录，不取消服务端任务、不删除文件。
- 每个操作只保留一个查询链；成功查询清除旧错误，停止跟踪与销毁会中止查询并清理计时器。能力恢复后可继续读取保存的操作。
- 413 等明确拒绝不会保留未入队请求；缺少克隆能力时仍可停止跟踪旧请求，继续使用本地来源。借入控制器的组件不销毁调用方资源。
- 来源搜索增加可见中文标签关键词；13 条浏览器交互/控制器回归、类型检查、lint 和完整架构门禁通过。
- [手机恢复交互 GIF](../../web/test/evidence/project-picker/phone-light-recovery.gif) 展示登记失败后的停止跟踪、选择其他项目及返回；使用 390×844 浅色隔离预览。

- CI 复核发现旧弹窗用例在等待 HTTP 被调用后就清理存储，真实创建 helper 后续写入偏好会污染下一用例；失败截图显示当前启动配置被前一用例的旧标识替代。夹具现在等待真实 helper 完成再清理，不增加超时或跳过断言。

## 项目来源第 6 层：生产会话设置接入

- 范围：US-B01–US-B03、US-B06、US-B10–US-B12；故事总数保持 259。
- 全局新建会话先选项目，四来源成功后进入实际会话设置；普通目录不显示 Git 控件，Git 工作位置只列当前项目，明确仓库与工作树上下文保留范围。
- 切换项目保留运行方式、名称与首条消息，重置项目专属字段；分屏未改目录时保留完整路径和工作区，显式选择项目后不沿用旧关联。
- 打开 linked worktree 子目录时按路径选中对应工作树，避免归一到仓库后误选主工作区；真实代码浏览器跨工作树回归通过。
- 缺少 Git 时仍可登记本地项目并启动 Shell；禁用 workspace 时保留直接目录及缺失目录确认创建，不请求禁用的项目接口。
- 电视遥控在 Command 搜索框按 Enter 选择项目，保持原有终端捕获与重连；两个电视视口回归通过，未进行电视真机验证。
- 单元测试 90 文件、1161 条通过；浏览器测试 117 文件、901 条通过。生产构建、测试类型、格式、lint、check:arch、文档、持久化、go vet 与 Go lint 通过。
- 完整端到端首次执行 628 条：564 通过、58 按设备跳过、6 失败；修复工作树选择、异步启动选择等待及核心模式按钮定位后，对应 6 条全部复测通过。
- 额外插件组合 36 条通过，覆盖 legacy 全量、纯核心、code、launch、workspace、workspace+launch、workspace+files、chat 合法依赖与 mcp；每种均从界面创建真实 PTY。
- Storybook 新增普通项目、Git 项目、项目不可用、启动配置不可用四个会话设置场景；连同九个来源场景，13 条布局回归通过，每条覆盖 12 个视口/主题/双倍字号组合。
- 双倍字号的 320px 屏幕下，底部操作允许换行；项目按钮复用可伸展的控件外观，长路径保留完整提示。
- 真实应用在 1280×900、768×800、390×844、375×667、320×568、667×375 明暗主题下检查 48 个全局入口/普通项目/Git 项目组合，弹窗位于视口内，无页面横向溢出。
- 截图使用相同隔离演示项目及入口，修改前使用 PR 基线构建，所有尺寸均保存明暗主题前后图。
- 桌面：[修改前](../../web/test/evidence/project-session/desktop-light-before.png)、[修改后](../../web/test/evidence/project-session/desktop-light-after.png)。
- 平板：[Git 设置](../../web/test/evidence/project-session/tablet-dark-git-setup.png)；手机：[普通项目设置](../../web/test/evidence/project-session/phone-light-plain-setup.png)。
- [手机真实应用 GIF](../../web/test/evidence/project-session/phone-light-navigation.gif) 展示项目列表、本地目录登记、填写首条消息、换项目、查看工作位置与关闭；390×844 浅色、正常速度、隔离演示数据。
- 局限：无手机或平板真机验证；无头浏览器未验证真实鼠标悬停与触屏常驻动作；GitHub 使用受控来源测试，未访问个人账号或私有仓库。

## 会话设置审查补充：返回项目与仓库刷新

- 保持会话设置表单挂载，返回同一项目不重建工作树字段；更换到其他项目仍重置项目专属字段。首次从明确目录上下文返回项目列表时等待挂载，再聚焦搜索。
- 真实后端复核进一步发现同 ID 仓库的新快照也会触发初始化；工作树字段改为依赖稳定的仓库 ID，状态刷新不清空分支、起始提交或手动路径。
- 新分支与已有分支两种模式均覆盖返回同项目及宿主快照更新；四个相关浏览器文件 89 条通过，测试类型检查通过。
- 桌面与手机真实后端的返回同项目及宿主刷新回归均通过；完整 make check 再次通过。
- Command 搜索的上下键及 Enter 保持原生列表导航；端到端显式等待选中项后再确认，两个电视尺寸连续三轮共 6 条通过，不放宽超时或跳过断言。
- [真实应用工作树草稿 GIF](../../web/test/evidence/project-session/desktop-light-worktree-draft.gif) 展示填写分支与目录、返回项目、选回同一项目后保留草稿；1280×900 浅色隔离数据，正常速度。
- Storybook 最后一次生产构建及 13 条布局复测通过；额外 4 倍 CPU 性能回归 18 通过、2 项按设备跳过。

## 远端组合复核补充

- 最新实现的远端单元测试 1161 条、浏览器测试 904 条通过，Storybook 与性能门禁通过。
- 电视夹具显式加入已登记后失效的目录；断言当前实际选中的可用项目，不假定列表第一行可选。两个电视尺寸连续三轮共 6 条通过。
- 原始移动聊天滚动用例在首轮远端重试时通过；独立连续三轮复测通过，未更改聊天滚动实现或放宽断言。

## 完整远端回归收尾

- 提交 36fdad68 的 Test 运行 43648754819 全部通过：格式、构建、类型、架构、文档、浏览器、Storybook、性能及四个端到端分片。
- 完整端到端 594 通过、58 按设备跳过，无失败和 flaky；单元 1161、浏览器 904 条通过。
- 额外真实应用核对 1280×720 明暗主题，修改前全局入口与修改后项目列表、普通项目、Git 项目共 8 个组合，弹窗位于视口内且无横向溢出。
- 同仓库状态刷新保留新分支、已有分支与手动路径的说明同步到 US-J04–US-J05；故事总数仍为 259。

## 上下文项目身份复核

- 项目身份按服务端仓库 ID 与规范路径比较；上下文、repository 前缀记录和后续关联 Git 的 directory 记录可识别为同一项目，真正换项目仍重置项目字段。
- 浏览器夹具使用真实 repository 前缀，覆盖全局与分屏上下文的新分支/已有分支返回；相关两个浏览器文件 59 条、项目边界单元 16 条通过。
- 全局与分屏上下文在桌面、手机四条真实后端回归通过；完整 make check 再次通过。
- [分屏上下文草稿 GIF](../../web/test/evidence/project-session/desktop-light-context-worktree-draft.gif) 为 1280×900 浅色真实应用，展示返回同项目保留分支与路径；无私人输出。
- 方案交付链接改为 PR #627 直链，实际 HTTP 200 验证通过。

## 实施任务完成核对

- 提交 95c8acdf 的 Test 运行 43651050961 全部通过；单元 1162、浏览器 906、完整端到端 596 通过，58 项按设备条件跳过。
- 首轮浏览器套件中未修改的终端触摸用例丢失第二段手势；该文件本地连续三轮共 24 条通过，远端失败任务重跑后全部通过，未更改终端实现、断言或超时。
- Pullfrog 对 95c8acdf 批准，pullfrog 与 pullfrog-approval 均通过；此前 Codex 和 Pullfrog 的三个审查线程全部处理并关闭。
- 21 个嵌入 PR 的图片与 GIF 链接全部实际 HTTP 200，方案交付直链可访问；本次规划任务已完成，故事总数保持 259。

## 使用说明一致性收尾

- CLI 接入、混合会话、工作树与使用入口改为项目优先顺序，并保留纯核心直接目录和明确上下文直达的说明。
- 项目 API 文档移除“尚未接入”的旧说明；现行会话规格和规格索引指向已实施的项目扩展，说明项目能力可用时的契约优先级。

## 项目步骤切换的焦点回归

- 文档收尾提交的远端电视回归发现，选定项目后焦点仍可能留在隐藏的搜索框；项目步骤现在保留草稿并显式将焦点移入可见设置，更新 US-B01，故事总数不变。
- 普通项目无 launch、Git 工作位置的浏览器焦点断言通过；相关三个浏览器文件 72 条通过，类型检查与完整 make check 通过。
- 两个电视尺寸各连续五轮共 10 条通过，使用真实遥控器按键并断言运行控件获得焦点，未注入测试焦点、放宽断言或跳过用例。
- 会话创建与分屏上下文 12 条、插件组合及纯核心 38 条真实后端回归通过；48 个明暗主题与尺寸组合重新检查，无页面横向溢出或弹窗越界。
- 三个交互 GIF 已按最终实现重新录制并逐帧抽查，继续使用隔离演示数据和正常速度；无真机及真实悬停验证。

## 快捷键教程迁入设置页（US-D15、US-H08）

- 设置页新增“快捷键”分区和锚点导航，保留原有 7 组键位、适用范围与平台说明；终端顶栏和菜单的旧入口移除。
- 桌面与手机的快捷键、会话切换定向 E2E 共 14 项通过，无重试，覆盖锚点刷新、输入隔离、草稿保留和返回终端。
- 生产与测试类型检查、lint、前端架构守卫（46 项测试）、源码格式检查、生产构建及文档守卫通过。
- 真实隔离应用检查桌面、平板、手机及 375px、320px、低高度横屏的明暗主题，均无页面横向溢出；键盘 Enter 导航后焦点进入教程。
- PR 证据包含桌面 1280×900、平板 768×800、手机 390×844 的同状态明暗主题前后截图、教程最终截图及正常速度导航 GIF。
- CI 补充：核心模式不再固定整份导航清单，改为逐项断言禁用插件的导航与分区均缺席；设置导航和终端触摸两个浏览器文件共 14 项通过，测试类型检查通过，未修改触摸实现或放宽断言。
- US-L02：远端六 worker 的 Storybook 两轮运行均在既有长对话布局中超时；本地单 worker 定向两项通过。为降低资源竞争，CI 的 Storybook 改为两个 worker；本地使用 CI 配置完整运行 119 项通过，无重试，保留所有断言与原有超时，不宣称提速。
- 未进行真机、真实鼠标悬停或触屏交互验证；故事 ID 和总数不变，未重新验收其他能力。

## Antigravity CLI 终端标题与混合会话标题延续（US-Q03、US-Q04、US-Q12）

- 范围（故事 ID 与总数 259 不变）：
  - `agy` TUI 注入的会话级 `--settings` `title` 钩子同步将 `conversation_title` 作为 `OSC 2` 窗口标题写入控制终端（为空时清空旧标题，不写标准输出）；
  - 终端未上报自身 `OSC 0/2` 标题时，`session/list` 探测与切换前核对采用非空会话标题；候选探测型 agent（如 `kimi`）切换至聊天侧时保留实时 `OSC 0/2` 窗口标题；
  - 同一运行内按标题序号仅清除上一会话遗留标题（保留同名 `/fork` 新标题），未命名会话切换或清空标题时重置可恢复记录标题。
- 本地验证：
  - `internal/agent/agy` 对真实 `/bin/sh` 执行注入钩子，覆盖空值、JSON/HTML 转义、CJK/Emoji 与紧凑/带空格 JSON；
  - `internal/session`、`internal/acp`、`internal/hybrid`、`internal/archtest`、`internal/docguard` 及全仓 Go race 测试通过；
  - 新增混合会话标题回归连续 5 轮 `-race` 通过：`TestAgyTUITitleContinuityAndRename`、`TestStaleTerminalTitleClearedOnAgentSessionChange`、`TestCandidateAgentPreservesLiveOSCTitleOnSwitch` 与 `TestSessionListTitleFallbackWithoutOSC`；
  - `go vet`、`golangci-lint`、生产与测试类型检查、Oxlint、前端架构守卫（46 项）、格式与持久化守卫通过。
- 局限：回归使用受控 `fakeagent` 与隔离临时目录，未在自动化测试中连接真实 Google 账号驱动在线 `agy` 对话；未重新验收其他故事能力。

## Codex 文件导入与快照开发验证

- 范围：US-Q19；新增原生历史开发入口，不宣称路线 #640 已完成。
- 固定 Codex 0.77.0 源码、合成 JSONL、解析器与来源映射进入同一 conversation 存储。
- 已运行定向 race：codexrollout、conversation、api、pluginapi、pluginwire。
- 已运行前端构建与 Svelte/TypeScript 检查、46 项前端架构测试、原生契约 Vitest。
- 已运行架构、文档守卫和 Go lint；工具安装前的首次 lint 因缺少固定版本而失败，安装后通过。
- wire snapshots 已重录核对；没有模型调用、真实图片链路或 token 节省实测。
- 补充验证了结构化工具输出、图片校验引用、resume/物化 fork、来源读取期间变化、并发追加、保留引用与未覆盖范围；后续直读迁移边界见[原生来源说明](../codex-native.md#投影图片与当前迁移边界)。
- 审查回归覆盖 9 MiB 工具历史的投影与 PNG、快照冻结缺口、按 position 关联的来源分页、发现取消/部分结果、诊断上限和无 Activity 原生记录的数据库唯一约束。

## 网页前端中英文国际化（US-A07）

- 范围：新增 US-A07，故事总数由 259 增至 260；采用 `@inlang/paraglide-js` 完成网页前端 4231 个词条的中英双语本地化，默认跟随系统语言（`zh*` 解析为简体中文，非中文显式标签解析为英语，空列表回退简体中文），显式偏好仅存于本机 `webmux.locale`，不同步至服务端设置。
- 入口与即时切换：设置页“外观与阅读”和未登录连接门禁复用 `SegmentedControl` 语言选择器，切换后不刷新页面、不断开实时连接即同步界面文案、前端 Zod 校验错误、预设配置标签、Diff Worker 文案与 `<html lang>`，并保留输入草稿；窄屏设置页头返回按钮自动收合为等高图标按钮。
- 静态与规格门禁：`make check`（前端构建、源码/测试/Storybook 类型检查、格式、Oxlint、`check:arch` 46 项架构断言且 0 克隆 0 未使用导出、`docs-check`、`persistence-check`、`go vet`、`golangci-lint` v2.15.0 0 告警）与 `openspec validate --specs`（99/99）通过。
- 测试门禁：Go race 全量包通过；单元测试 91 文件 1168 项通过（含词条键集、复数变体与占位符一致性及源码硬编码中文词法扫描）；浏览器测试 118 文件 908 项通过；Storybook 131 项通过；端到端 602 项通过、56 项按设备跳过；4 倍 CPU 性能回归 18 项通过、2 项按设备跳过。
- 视觉与证据：隔离演示后端检查 1280×900、1280×720、768×800、390×844、375×667、320×568、667×375 明暗主题英语视图，无页面横向溢出且弹窗位于视口内；PR 附桌面 1280×900、平板 768×800、手机 390×844 明暗前后对比图与正常速度语言切换 GIF。
- 局限：仅在 Linux 无头 Chromium 下验证，`hover` 与 `any-pointer: coarse` 均为 false，真实鼠标悬停和触屏常驻状态未验证；未在手机、平板、macOS 或 Windows 真机验证；未重新验收其他既有故事。

## 终端与聊天底部居中“回到最新”悬浮按钮（US-D07、US-H08、US-Q05）

- 范围（故事 ID 与总数 260 不变）：
  - 移除终端顶栏右上角“滚动到最新位置”按钮与冗余回调；抽取共享 `JumpToLatestButton`（`outline` + `raised`），在桌面、平板与手机统一为向上滚动离开底端时于视口靠下居中悬浮显示，点击直达底端且不抢夺输入焦点；
  - 接入 Paraglide `m.s4_jump_to_latest()` 中英双语词条（中文“回到最新” / 英文“Jump to bottom”），并清理已移除旧按钮的冗余词条；
  - 修正聊天文档级滚动在高度低于 480px 的横屏窗口中停靠区 `overflow-y: auto` 裁切悬浮按钮的问题。
- 本地验证：
  - 前端单元测试（含 `test/i18n.test.ts` 中英词条对等与源码硬编码中文扫描）通过；相关浏览器回归（`TerminalViewport` 含中英文即时切换、`ChatView` 及终端控制/混合/观察/回放/触摸）通过；Storybook 完整套件 131 项通过；
  - 定向端到端回归（`compact-chrome`、`lifecycle`、`i18n` 与混合聊天文档滚动）与全仓 Go `-race`、`make check`、`openspec validate --specs` 通过；
  - 隔离演示应用在 1280×900、1280×720、768×800、390×844、375×667、320×568、667×375 明暗主题共 28 个终端/聊天组合下核对无横向溢出、按钮居中未裁切且支持键盘聚焦触发；采集三组尺寸明暗前后对比截图与 1280×800 正常速度交互 GIF（46 帧，约 10.4 秒）。
- 局限：无手机或平板真机验证；无头 Chromium 未验证真实鼠标悬停与 `any-pointer: coarse` 触屏硬件行为；未重新验收其他故事能力。

## 原生 PNG 字库的发行取舍（2026-11-16，US-Q19）

- 决定默认保留完整、未修改的 CJK 字库，支持单文件离线中文 PNG；运行时关闭 chat 不会剪裁静态链接体积。
- 隔离 `62c43b2aa` 基线与接入版，复用同一前端 ZIP、Go 1.27.4 和纯 Go 发布参数，实测 Linux amd64 增加 16.67 MiB、macOS arm64 交叉版增加 16.78 MiB。
- 20 MiB 的本次接入增量预算及精确字节、哈希、方法记录在[体积说明](../binary-size.md#原生历史-png-的字体预算)；未声称真机运行或 RSS 改善。
- OFL、Adobe 版权、核心及依赖许可均随二进制内嵌资源交付，新增真实静态资源读取回归。
- 使用仓库既有版本输出替身制作 deb/rpm，在隔离 Debian/Fedora Docker 中验证新增许可文件的路径、内容、模式、安装、升级和卸载；不把替身包检查当作真实服务器行为证明。

## 主机状态新增交换空间指标（US-A04）

- 范围（故事总数 260 不变）：
  - `GET /api/health` 新增可选 `swap`（total/used），Linux 解析 `/proc/meminfo` 的 `SwapTotal`/`SwapFree`，macOS 读 `vm.swapusage`，其他平台与未配置交换空间的主机不上报；
  - 侧栏主机状态弹层新增“交换空间”行（`s1_swap`/`s1_swap_value` 中英词条）；
  - 协议参考、`docs/architecture.md` 的 `internal/hoststat` 行与包注释同步说明 `swap`。
- 验证：`internal/hoststat` 新增解析用例、`TestWireSnapshots` 重录、`go vet`、gofmt 通过；darwin/windows 交叉编译通过；真实服务端到端返回 `swap` 字段且弹层渲染“已用 444.0 KB / 共 6.0 GB”；`pnpm --dir web check`、`lint`、`check:arch`、`make format-check` 通过。
- PR 证据：桌面 1280×900、平板 768×800、手机 390×844 明暗主题六组前后对比截图（before 为移除新增行复现基线），手机截图为打开的导航抽屉内弹层。
- 局限：macOS `vm.swapusage` 仅交叉编译未实机运行；无头浏览器未验证真实悬停与触屏常驻状态；未重新验收其他既有故事。

## 明暗主题改为当前浏览器本地偏好（US-C09）

- 范围（故事 ID 与总数 260 不变）：
  - 将明暗主题（`system` / `light` / `dark`）从服务端同步设置（`/api/settings`、`/ws/settings`）迁回浏览器本机 `deviceScopedKeys`（`webmux.theme`，与 `index.html` 首屏内联防闪烁脚本对齐）；切换或清理远端宿主凭据时保留本机主题偏好；
  - `internal/settings` 保留旧 `settings.json` 中遗留 `theme` 键的兼容读取并在下次写入时自动移除；`PATCH /api/settings` 提交 `theme` 返回 `400`；同步更新 Go wire 快照与前端严格契约；
  - 设置页“外观与阅读”将主题选择器移出服务端同步 `<fieldset>` 并与语言、终端字号等本机偏好统一归组，服务端设置返回 `503` 不可用时仍可即时切换主题；同一浏览器的多标签页与独立代码窗口通过 `storage` 事件实时同步。
- 本地验证：
  - 静态与规格门禁：`make check`（前端构建、源码/测试/Storybook 类型检查、格式、Oxlint、`check:arch` 46 项架构断言且 0 克隆 0 未使用导出、`docs-check`、`persistence-check`、`go vet`、`golangci-lint` v2.15.0 0 告警）与 `openspec validate --specs`（99/99）通过；
  - 后端与组件门禁：全仓 Go `-race` 测试通过；浏览器测试 118 文件 912 项通过；Storybook 完整套件 131 项通过；
  - 单元测试：90 文件 1166 项通过；与本改动无关的 `test/base64url.test.ts` 1 项因本机 Node `v24.20.0` 尚未提供 `.nvmrc` Node `26.13.0` 原生 `Uint8Array.prototype.toBase64` 在本地失败，由远端 Node 26 CI 验证；
  - 定向端到端回归：`theme`、`settings-sync`、`code-popout`、`page-stability`、`terminal`、`frontend-quality`、`i18n` 共 68 项通过、12 项按设备条件跳过、0 失败；
  - 视觉与证据：隔离演示应用在 1280×900、1280×720、768×800、390×844、375×667、320×568、667×375 明暗主题共 28 个前后组合下核对无页面横向溢出；
  - 公开取证：采集三组尺寸明暗前后对比截图、服务端 `503` 不可用降级前后对比图及 1280×900 正常速度交互 GIF（8.3 秒），全部 15 个公开直链均经 HTTP 200 校验。
- 局限：仅在 Linux 无头 Chromium 下验证，未在手机、平板、macOS 或 Windows 真机验证真实鼠标悬停与触屏硬件行为；未重新验收其他既有故事。

## 终端图片滚出屏幕后拖回仍完整显示（2026-11-16，US-D12）

- 范围（故事 ID 与总数 263 不变）：
  - `attachTerminalGraphics` 修正光标与活动屏边界：
    - `moveCursor` 将非滚屏的 `CSI Ps B`（`CUD`）改为底边距自动滚屏的 `ESC D`（`IND`），使底行多行 `C=0` 图片完整分配历史缓冲行而不与后续输出重叠；
    - `cursor` 将待折行（pending wrap，`cursorX === cols`）钳制在右边界内；
    - `viewport` 改为返回活动屏幕 `{ top: baseY, bottom: baseY + rows }`，不再误用历史滚动视口 `viewportY`，并在 `onRender` 视口或缓冲区切换时同帧刷新图形位置；
  - `removeKittyPlacements`：将 `a=d` 屏幕级删除（`d=a`/`c`/`p`/`q`/`x`/`y`/`z`）限定在当前活动缓冲区的活动屏幕内，避免查看历史时被底部 TUI 刷新或 `CSI 2 J` 误删回滚区图片及图像数据，保留 `d=i`/`n`/`r` 全局按标识删除语义；
  - `graphicsAnchor` 修正行锚点生命周期与回流跟踪：
    - 为 xterm `CircularList.get` 增加 `[0, lines.length)` 边界检查；
    - 按放置总行数跟踪顶部裁剪后的存活行，直到整幅图片全部离开缓冲才释放，兼容备用屏超高 `C=0` 放置与跨屏幕顶部的 `CSI 3 J` 清理；
    - 在终端变宽回流解包续行（`onDelete`）时将锚点合并至前一行，而非误判为 VT 行删除；
    - 在 `RIS` 缓冲区替换与尺寸重排后同步清理越界放置；
  - `.xterm-graphics-layer`：设置 `z-index: 3`，确保在 WebGL 与动态降级的 DOM 渲染层及选区层（`1`）之上稳定绘制，同时低于辅助输入（`5`）、装饰（`6`–`8`）与滚动条（`11`）。
- 本地验证：
  - 静态与文档门禁：`make check`（前端构建、源码/测试/Storybook `svelte-check --tsgo --fail-on-warnings`、格式、`docs-check`、`persistence-check`、Oxlint、`check:arch` 46 项断言且 0 克隆 0 未使用导出、`go vet`、`golangci-lint` v2.15.0 0 告警）通过；
  - 单元测试：`pnpm --dir web test:unit` 92 文件 1,216 项通过（含新增 `kitty-graphics.test.ts` 屏幕级删除与历史保留断言；无关的 `test/base64url.test.ts` 1 项因本机 Node `v24.20.0` 缺少 `.nvmrc` Node `26.13.0` 的 `Uint8Array.prototype.toBase64` 在本地失败，由远端 Node 26 CI 验证）；
  - 浏览器回归：`pnpm --dir web test:browser` 122 文件 947 项全量通过，含 `TerminalGraphics.test.ts` 18 项与新增 `TerminalGraphicsScrollback.test.ts` 5 项（覆盖滚动条拖动、底行 `C=0` 多行分配、待折行钳制、活动屏清理、跨顶边部分截断及宽度回流/渲染器切换）；
  - 端到端回归：Playwright `test/e2e/terminal.spec.ts` 18 项通过、4 项按设备项目条件跳过、0 失败；
  - 视觉与证据：隔离演示服务在 `1280×900`、`1280×720`、`768×800`、`390×844`、`375×667`、`320×568`、`667×375` 明暗主题下核对无页面横向溢出，采集桌面、平板、手机明暗六组前后对比截图与 `1280×900` 正常速度交互 GIF。
- 局限：仅在 Linux 无头 Chromium 下验证，未在手机、平板、macOS 或 Windows 真机验证真实鼠标悬停与 `any-pointer: coarse` 触屏硬件行为；未重新验收其他既有故事。

## 原生直读切换与真实运行边界（2026-11-16，US-Q19、US-Q20）

- 范围：九个 reader、metadata-only import、冻结 scope、统一机械投影和 schema 14 正文镜像退役；故事 ID 与总数不变。局部运行证据不等于所有 Agent 在线兼容，也不代表主线 CI 已完成。
- 集成回归已通过：九个 reader 的真实脱敏夹具 race 测试，以及 historyimport/historyprojection 切换 race 测试；夹具通过不是九种 Agent 在线模型调用成功的证据。
- `import` 恢复为历史身份与冻结引用登记，不启动 Agent、不持续复制正文；引用与 retained 导出、分支/Generation/hash 以及当前任务独立保存的契约见[原生历史](../codex-native.md)。
- schema 14 先私有迁移旧可导出正文与引用为不可变 exports，再 DROP 旧 Activity/native_source/history_snapshot 链路；v10/v11 owner 修复及原 native ID 的 unknown 门禁继续保留。
- omp 18.20.2 使用隔离 cwd/session-dir 与纯合成文本，真实模型 normal、同会话 resume、native fork 后 continue、扩充合成历史后的远端 compaction 及压缩后 resume 均观察到成功。
- fork RPC 未收到确认应答；成功证据限于新 native 文件与随后模型 continue，不能写成 RPC ack 成功。首次 compaction 因会话过小拒绝，扩充合成数据后才成功。
- 真实 omp 的原分支、fork 用户祖先截断和 compaction 保留范围已由 reader 与实际 historyprojection 验证；父/fork 冻结游标互换返回 ErrChanged，私有加密状态未出现在导出正文。
- Claude Code 2.1.245 在隔离 bare 环境未登录，未进行 resume/fork/compaction；OpenCode 1.18.23 免费额度拒绝，仅读取其真实失败请求数据库并验证合成用户消息，不宣称模型调用成功。
- Codex 0.165.0 沿用此前 token_revoked 失败事实，没有重复模型请求；Kimi、agy、Devin、QoderCN 等 CLI 不在 PATH。Qoder 的预期实测对象是 QoderCN，两者均未做在线运行验证。
- 交接回归：conversation、handoff、threadread race 通过；真实 stdio MCP 读取省略原文、长项续读、来源隔离和 provider new/load/resume 通过；保留快照在追加、遗忘及删除来源后仍独立读取完整正文，附件按目标自己的路径回放。
- 前端验证：源码与测试类型检查、Oxlint、46 项架构断言、374 项相关契约与消费者单元测试、2 项历史读取浏览器测试通过；隔离 Chromium desktop 的草稿关闭/刷新/重启及 A→B→A 两项端到端通过，查看了 1440×900 交接弹窗截图。
- 本轮 UI 未验证手机、平板、深色或真机；主线全量 CI 尚待集成结果。本节真实 CLI 运行证据仅含原生文本，图片传输实验的独立边界见[真实 ACP 图片实测](../native-image-transport.md)。
- 原始 session、compact 输出可能含 encrypted/private provider state、签名及凭据标识，未复制入库；仅允许使用脱敏合成内容与安全结果摘要。

## 设置页“Agent 工具”本机扫描、按需更新检查与 CLI `webmux agents`（2026-11-16，US-B13、US-S18）

- 范围（新增 US-B13 并扩展 US-S18，故事总数由 263 增至 264）：
  - 在 `internal/agentprofile` 补充分发元数据（安装/升级命令、文档链接、公开上游包源），由共享包 `internal/agentcatalog` 汇总 9 款混合模式 agent、外部 ACP 连接器与 4 款纯终端预设；
  - 新增 `internal/agentcheck` 服务：单次交互式启动 Shell（`bash -ic` / `zsh -ic`，否则回退进程 `PATH`）批量定位路径并带出登录环境 `PATH`，独立会话/进程组有界并发采集 `--version` 并做内存 TTL 缓存；
  - `GET /api/agent-tools` 保持零外发网络请求；仅在显式触发 `POST /api/agent-tools/check` 或 `webmux agents --check` 时查询只读 HTTPS 包源（npm 主源及 `registry.npmmirror.com` 回退、PyPI JSON、GitHub Releases），区分 `source_unavailable` 与 `belowVerified`；
  - 随 `launch` 内置功能装配：禁用时路由返回 `404`、设置页隐藏 `#agent-tools` 分区与导航锚点、`webmux agents` 报告 `feature_disabled`；
  - 设置页在 `#launch-configs` 正下方新增 `#agent-tools` 常驻卡片（默认展开已安装项与连接器子卡片、底部折叠收纳未安装工具、提供一键复制安装/升级命令）；新增 `webmux agents [--check] [--refresh] [--all] [--json]`。
- 本地验证：
  - 静态与规格门禁：
    - 前端构建、源码/测试/Storybook `svelte-check --tsgo --fail-on-warnings`、格式检查、Oxlint 与 `check:arch`（46 项架构断言，0 克隆、0 未使用导出）通过；
    - `docs-check`、`persistence-check`、`go vet`（含 `darwin` 交叉 vet）、`golangci-lint` v2.15.0（0 告警）与 `openspec validate --specs`（100/100）通过；
  - 后端与组件门禁：全仓 Go `-race` 测试（含 `protocol/relay`）全量通过；浏览器测试 123 文件 948 项通过；Storybook 完整套件 137 项（含新增 4 个场景 × 12 组视口/主题/双倍字号布局断言）通过；
  - 单元测试：93 文件 1,261 项通过，含 `wire-contract`、`wire-arrays` 与 `i18n` 中英词条对等及源码硬编码中文扫描：
    - 无关的 `test/base64url.test.ts` 1 项因本机 Node `v24.20.0` 缺少 `.nvmrc` Node `26.13.0` 原生 `Uint8Array.prototype.toBase64` 在本地失败，由远端 Node 26 CI 验证；
  - 定向端到端回归：`agent-tools`、`launch-configs`、`i18n`、`settings-sync` 共 26 项全部通过（0 失败），覆盖 `.bashrc` 动态路径重扫、零静默外连、按需更新检查、命令复制及核心模式 404 门控；
  - 视觉与证据：隔离演示服务在 `1280×900`、`1280×720`、`768×800`、`390×844`、`375×667`、`320×568`、`667×375` 明暗主题与中英文共 28 个组合下核对无页面横向溢出；采集桌面、平板、手机明暗六组前后对比截图与 `1280×900` 正常速度交互 GIF（6.0 秒），13 个公开直链均经 HTTP 200 校验。
- 局限：仅在 Linux 无头 Chromium 下验证，未在手机、平板、macOS 或 Windows 真机验证真实鼠标悬停与触屏硬件行为；自动化测试中上游包源使用本地受控测试服务模拟，未依赖实时外网仓库稳定性；未重新验收其他既有故事。

## 终端 OSC 8 外部链接应用内本地化确认（2026-11-16，US-A07、US-D01）

- 范围：将 xterm 默认英文浏览器 `window.confirm` 链接跳转提示替换为跟随用户语言偏好的应用内 `ConfirmDialog`，同步更新 US-A07、US-D01 与 `docs/usage/terminal.md`，故事 ID 与总数不变。
- 核心改动：
  - 新增 `web/src/domains/terminal/terminal-link.ts`，严格放行 `http:` / `https:` 协议并将 IDN 主机名规范化为 Punycode，确认后以 `window.open(href, "_blank", "noopener,noreferrer")` 在新标签页打开；
  - 在 `terminal-renderer.ts` 注册 `linkHandler` 将点击事件转发至会话终端 chrome，并在 `SessionTerminalDialogs.svelte` 渲染本地化 `ConfirmDialog`（支持 `zh-CN` 与 `en` 实时无刷新切换，不阻塞 PTY 输出流，关闭后按平台规则恢复终端焦点）。
- 本地验证：
  - 静态与文档门禁：`make check`（前端构建、`svelte-check --tsgo --fail-on-warnings`、格式、`docs-check`、`persistence-check`、Oxlint、`check:arch` 46 项断言、`go vet`、`golangci-lint`）通过；
  - 单元测试：`make test-unit` 96 文件 1,265 项全量通过（含新增 `test/terminal-link.test.ts` 3 项）；
  - 浏览器与端到端回归：Vitest 浏览器 `test/browser/TerminalLinkConfirm.test.ts` 2 项通过（覆盖中英实时切换、PTY 流不中断、取消与确认打开、`javascript:` 拦截及 320px 超长 URL 折行）；Playwright `test/e2e/i18n.spec.ts` 与 `test/e2e/terminal.spec.ts` 共 24 项通过、4 项按设备项目条件跳过、0 失败；
  - 视觉与证据：隔离演示服务核对 `1280×900`、`1280×720`、`768×800`、`390×844`、`375×667`、`320×568`、`667×375` 明暗主题与中英文均无横向溢出且弹窗完整落在视口内，采集桌面、平板、手机明暗前后对比截图与 `1280×900` 正常速度交互 GIF。
- 局限：仅在 Linux 无头 Chromium 下验证；无头浏览器 `hover` 与 `any-pointer: coarse` 均为 `false`，真实鼠标悬停态与触屏硬件行为未在手机、平板、macOS 或 Windows 真机验证。
