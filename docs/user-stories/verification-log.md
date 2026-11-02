# 用户故事核对记录（续）

本页从 [WebMux User Stories](../user-stories.md) 拆出，按时间续记 2026-10-29 以后的局部核对；更早的记录仍在索引页开头。

本次局部核对（2026-10-29）：代码质量扫描修复对应 US-K09（`allowed_origins` 条目须为浏览器 Origin 的精确小写形式、`listen` 端口无效时启动即失败），删除会话的非“不存在”失败不再报 404；其余为内部并发、测试与无障碍修正，不改变故事行为。当前 168 个故事 ID 唯一；未重新核对其余故事能力。

本次补充核对（2026-10-29）：软键栏 Shift 修饰键对应 US-H01，点按为一次性、长按约 0.5 秒锁定并在再次点按后解除，支持 Shift+Tab 与带 Shift 的方向、Home/End、翻页序列；粘贴内容不应用修饰键。已运行前端类型检查、lint、架构与文档守卫、定向 Vitest 及 softkeys Playwright 用例；未重新核对其余故事。

本次局部核对（2026-10-31）：终端实时输出不再为攒批固定等待后续字节，对应 US-D01。
逐字回显延迟测试在修复前复现约 51 ms 等待，修复后三轮约 0.03 ms（本机 PTY 到 WebSocket）。
连续输出回归覆盖 14 万字节的完整性、顺序、64 KiB 实时帧上限和退出顺序。
未重新核验其余故事能力。

本次局部核对（2026-10-31）：#215 独立代码窗口中修改视图的行号 Shift 范围选择与重新选择失效，对应更新 US-I13。
diff 库的行号按压只在主页面 `document` 上等待 pointerup，现由 `CodeDiffReader` 在独立窗口中转发；`code-popout.spec.ts` 新增用例修复前失败、修复后通过。
未重新核验其余故事能力。

本次局部核对（2026-10-31）：#216 首页未保存显示方式时恢复默认输出预览，显式选择的紧凑列表仍在当前浏览器记忆，对应更新 US-C02。
`repository-scope.spec.ts` 改为断言默认预览、切换紧凑后刷新仍保持紧凑；未重新核验其余故事能力。

本次局部核对（2026-10-31）：圆角统一为设计系统刻度并由 `arch:test` 检查，对应更新 US-L06；聊天卡片、仓库侧栏、紧凑列表面板和空状态由 12px 改为内容容器 8px，复选框、滑块、滚动条与键帽不再使用直角或固定像素。
新规则在改动前的源码上列出 18 处违规，改动后 `check:arch` 通过；未重新核验其余故事能力。

本次局部核对（2026-10-31）：US-D01 的高吞吐输出在 CPU 紧张时不再误断开。持续输出产生大量小块读取（约 340 字节），写协程被调度延后时 256 条队列先满，以 4408 断开（macOS 与 Ubuntu CI 均失败）。
终端输出队列改按积压字节限制（8 MiB，最多 1024 条）；`TestTerminalBurstPreservesOutputBeforeExit` 在 `-race -cpu 1`/`-cpu 2` 下由 23/100、9/200 失败变为 0/300，与全量 Go race 并行时 0/150；新增字节预算单元测试。测试客户端读取上限提高到 1 MiB（输出帧最大 64 KiB）。未重新核验其余故事能力。

本次局部核对（2026-11-01）：#220 代码工作区资源管理器的右键文件操作（上传、下载、目录 ZIP、新建文件夹、重命名、复制、剪切、粘贴、删除）对应新增 US-I19，并更新 US-I18（临时文件夹改为可写）与 US-A05（代码接口含文件操作）；覆盖索引同步。未重新核验其余故事能力。

本次局部核对（2026-11-01）：#220 补充移动端文件操作（长按、底部操作面板、触控尺寸、剪贴板提示条、重命名预选文件名），对应更新 US-I19；Playwright 桌面与 Pixel 7 用例覆盖长按与操作面板。未重新核验其余故事能力。

本次局部核对（2026-11-01）：独立代码窗口改为在窗口内用同源 iframe 运行独立的代码工作区，并新增代码工作区拖放上传，对应更新 US-I13、US-I19。未重新核验其余故事能力。

- 行为：弹出、收回与直接关闭窗口保留完整阅读状态；窗口内右键菜单、对话框与底部面板不再打开在主窗口。
- 依据：`CodePanel`、各阅读组件与导航的浏览器测试，以及 `code-popout.spec.ts` 的桌面 E2E（画中画、普通弹窗、拖放上传）。

本次局部核对（2026-11-01）：#208 的 Codex CLI 混合模式与第三方 ACP 连接器安装检查对应新增 US-Q16，并更新 US-Q01、US-Q03、US-Q08，覆盖索引同步。

- 依据：`internal/agentprofile`、`internal/hybrid`（含 `codex_test.go` 的往返、连接器缺失与安装后重检、启动文件检查、外部占用、插话退回队首、启动静默期）与 `internal/session` 的 Go race 测试，线上快照与 wire 契约用例，浏览器组件测试，以及桌面/Pixel 7 的 Codex E2E 用例。
- 真实工具实测：Linux 上 Codex CLI 0.166.0 与 codex-acp 2.5.0 的写锁、`session/load` 配置重置、`thread_active_writer`、插话 `idleBehavior` 与权限选项（结论记录在 OpenSpec 变更的 design.md）；未在 macOS 上用真实 Codex 验收。
- 一致性：175 个故事 ID 唯一；未重新核验其余故事能力。

本次局部核对（2026-11-01）：#222 把 agent 差异收拢到 `internal/agentprofile` 的适配器接口与每个 agent 一个子包，对应更新 US-Q01（不支持原因与勾选框说明列出服务端返回的受支持 agent）、US-Q03（强制切换损失说明由服务端下发）与 US-L05（agent 名称守卫）。未重新核验其余故事能力。

- 依据：`internal/agentprofile/...` 的适配器契约测试与命令解析测试、`internal/archtest`、`internal/hybrid` 与 `internal/api` 的 Go 测试、重录的线上快照，以及前端单元与浏览器组件测试。

本次局部核对（2026-11-01）：#227 重连期间恢复“正在恢复…”提示，对应更新 US-D03 与 `terminal-stream-resume` 规格。可续接终端在重连时显示不遮挡内容的提示，连接恢复后消失，已有内容保持可见。

- 通过：定向 `TerminalReplay` 与 `TerminalViewport` 浏览器用例、类型检查、测试类型检查、Oxlint、Oxfmt、前端架构守卫与文档治理。
- 未重新核验其余故事能力。

本次局部核对（2026-11-01）：`AGENTS.md` 体积预算对应新增 US-L11；`AGENTS.md` 由约 22 KB 压缩到 10 KiB 以内，频率较低的规则原文迁入 `docs/agent-rules/`。

- 依据：`go test -count=1 ./internal/archtest -run TestAgents` 与 `go test -count=1 ./internal/docguard`。
- 一致性：177 个故事 ID 唯一，两份索引已同步 US-L11；未重新核验其余故事能力。

本次局部核对（2026-11-01）：调整尺寸重绘风暴对应 US-D01；新增文字与 Kitty 图片混合的整屏重绘模拟程序与 4x CPU 性能用例。

- 依据：`go test -race ./internal/hub ./internal/api`（新增突发输出合并回归：200 块由 200 帧降为约 16 帧）、`kitty-graphics` 单元与 `TerminalGraphics` 浏览器用例。
- 本机性能用例（DOM 渲染器）：每次风暴 WebSocket 消息 113→13，最大帧间隔 1.7 s→0.18 s，时长 4.6 s→约 2.5 s；未测 WebGL；未重新核验其余故事能力。

本次局部核对（2026-11-01）：#239 恢复时闪屏对应 US-D01、US-D03（同步输出与回放期间文字与 Kitty 图片同帧切换）。

- 依据：新增 `TerminalSyncOutput` 浏览器用例（同步重绘、滚动重绘、快照回放、同步期间 resize 与焦点变化、快照停在图片上传中），修复前均失败；全部浏览器与单元用例、终端相关 E2E 通过。
- 本机重绘风暴性能用例（DOM 渲染器、4x CPU、同步输出）：可见重绘帧 37→2–3，长任务 2.3 s→0.33 s；未测 WebGL；未重新核验其余故事能力。

本次局部核对（2026-11-02）：Windows 安装程序对应新增 US-O12，并更新 US-O01（本地缓存与预置服务端离线安装）与 US-O04（安装程序登记登录启动、`webmux-tray.exe quit`）；覆盖索引同步。
核对依据为 `internal/tray` 的 Go race 测试与在 Windows 11 上实际运行的 `windows-installer-smoke.ps1`（静默安装、`quit`、运行中升级、拒绝其他位置的托盘、架构不符、卸载）；178 个故事 ID 唯一，O 组来源路径有效；未用真实 WSL 发行版验收托盘菜单离线安装，未重新核验其余故事能力。

本次局部核对（2026-11-02）：Linux deb / rpm 安装包对应新增 US-K13，覆盖索引同步。

- 依据：`internal/service` 的 Go 测试（包内单元的状态、卸载与无单元错误），以及 `test-linux-packages.sh` 在 Debian 与 Fedora 容器中的安装、检查与卸载。
- 实测：在本机 systemd 用户管理器上把同内容单元放在非 `~/.config` 的用户单元目录，`webmux service status|restart|stop|start|uninstall` 均作用于该单元，卸载后文件保留。
- 实测：在以 systemd 为 PID 1 的 Debian 与 Fedora 容器中为普通用户启用 linger 后安装 0.0.1 包。
  - 安装后服务保持未启用；`systemctl --user enable --now webmux` 启动并生成配置。
  - 升级到 0.0.2 时服务进程不变，`webmux service restart` 后运行新版本。
  - `webmux service uninstall` 禁用并停止且保留单元文件；卸载包后两项文件删除、用户配置保留。
- 限制：未验证 arm64 包的安装；未重新核验其余故事能力。

本次局部核对（2026-11-02）：自动更新新增 US-K14–US-K16、US-L12、US-O13，并更新 US-K13、US-O01、US-O11、US-O12；184 个故事 ID 唯一，两份索引已同步。

- 依据：更新、签名、API 与托盘包的 Go race 测试，前端 `pendingUpdate` 用例；容器内的安装包定时器测试；systemd 容器中定时器服务把 0.0.1 升到 0.0.2，用户服务 PID 与会话不变；minisign 0.12 验签通过。
- 限制：托盘自更新与 NSIS 静默升级只在 Linux 上测试与编译（makensis 3.11），未在 Windows 上运行；未重新核验其余故事能力。
