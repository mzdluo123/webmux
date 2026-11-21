# 创建会话与选择启动位置

连接、认证与中转访问边界见 [连接、认证与访问边界（A）](connection-and-launch.md)。

## B. 创建会话与选择启动位置

- **US-B01 启动 Shell 或 agent**：作为使用者，我希望通过自定义启动配置或显式命令运行 Shell、Open Code、Claude Code、Codex、omp 等程序，以便快速开始终端工作或遥控 agent。
  验收要点：
  - 显式 argv、cwd 与环境变量属于核心能力；保存的配置与预设需 `launch`，仓库位置需 `workspace`，混合模式需 `chat`，附件及临时文件引用需 `files`。关闭这些功能不妨碍创建普通 PTY（US-K19）；
  - 启用 workspace.projects 时，全局新建会话先进入可搜索的项目列表，选定后显示项目与运行设置；明确仓库、工作树和分屏上下文直接进入设置，缺少项目能力时保留“位置”与“运行”的直接创建方式；
  - 首次选定项目后，键盘焦点进入可见的工作位置、运行或命令控件。往返项目列表与运行设置时各自恢复滚动位置，返回设置时优先聚焦原来的可见控件；
  - 手机上，新建会话的项目向导在选择项目和运行设置之间切换时保持抽屉高度，本地文件夹登记返回后可继续在固定标题下方滚动表单；
  - 启用 `launch` 时提供保存的启动配置与末尾的“自定义命令…”，首次启用且存储缺失时才写入常见 agent 预设，没有不可修改的内置预设；
  - 本机 Agent 工具扫描就绪后，新建会话“运行”选择器、快速开始聊天与交接新建目标自动隐藏对应主 CLI 未安装的受检 Agent 启动配置：
    - `Shell` 等非 Agent 配置、显式路径（含 `/` 或 `\`）、携带自定义 `PATH` 环境变量的配置，以及主 CLI 已安装仅缺少外部 ACP 连接器的配置保留显示；
    - 扫描未完成或读取失败时保留全部已保存配置；
  - 默认启动配置自动跳过已隐藏的未安装 Agent 配置；浏览器记住的配置对应本机未安装 Agent 时置为待选状态并要求重新选择；服务端存在已保存配置且全部因未安装而隐藏时，选择器展示前往 `/settings#launch-configs` 管理的提示并默认选中“自定义命令…”；
  - Claude Code、Codex CLI、Antigravity CLI、Open Code、Qoder、QoderCN、Kimi、Devin 与 omp 的预置配置默认自动批准工具调用：
    - Claude Code `--dangerously-skip-permissions`；
    - Codex CLI `--dangerously-bypass-approvals-and-sandbox`；
    - Antigravity CLI `--dangerously-skip-permissions`；
    - Open Code `--auto`；
    - Qoder `--dangerously-skip-permissions`；
    - QoderCN `--dangerously-skip-permissions`；
    - Kimi `--yolo`；
    - Devin `--permission-mode dangerous`；
    - omp `--auto-approve`；
  - Cursor、Copilot、Qwen、CodeBuddy 使用各自默认授权行为；
  - 十二款 CLI 的启动命令见 [接入说明](../../cli-agents.md)；
  - Claude Code、Codex CLI、Antigravity CLI、Open Code、omp、Qoder、QoderCN、Kimi 与 Devin 预置配置默认为混合模式（US-Q01）；
  - 命令选择器可用键盘操作（Enter 打开、方向键与搜索、Esc 只关闭选择器），选中行和每个选项同时显示图标、名称与命令，长命令保留完整提示；
  - 启动方式、工作位置、目录、项目和 GitHub 仓库的选择列表共用通栏搜索样式，聚焦时搜索栏底线高亮。
  - 保存的配置按 ID 启动，临时命令按完整 argv 启动，均创建真实 PTY；
  - Open Code 可通过“加载 shell 启动文件”保存为 Bash（`~/.bashrc`）或 Zsh（`~/.zshrc`）包装，新安装按服务端 `$SHELL` 预选，其他 shell 回退为不包装或手动选择，不自动安装程序；
  - 勾选了混合模式的配置可选择从终端或聊天开始（US-Q02）；
  - 首页与工作树统一使用「新建会话」；会话页经「打开面板 → 新建会话…」复用同一对话框，打开表单时关闭面板选择器；
  - 无论哪种启动方式，对话框都有可选的“用户指令”，创建后自动发送：
    - 聊天端作为第一条消息；终端端若所选混合模式 agent 已支持命令行初始提示参数（且启动命令本身未带初始提示），通过首次 TUI 启动参数直接传入（不写入 `SessionInfo.Cmd` 或持久化记录），尚未迁移的 agent、不支持该消息形式的边界情况与自定义命令仍在程序输出安静后输入并自动回车；
    - 未填会话名称的终端启动以该指令为会话名（US-B07）；
    - 位置属于已登记仓库且该仓库设置了初始化指令时，另有“使用初始化指令”（从聊天开始时默认勾选，从终端开始时默认不勾选），指令先于用户指令发送（模板规则见 US-J12）；
    - 引用无效、临时文件夹不可用等发送前即可发现的问题使创建失败并保留输入，不留下半启动的会话；
  - 更换项目保留运行配置、自定义命令、名称与用户指令，重置项目专属分支和工作树字段；用户指令中的 Enter 保持换行；
  - 底部汇总“将在 X 运行 Y”，项目与默认值齐全时可直接创建；会话名称收在“高级：会话名称”中；
  - 创建成功后按入口进入终端或加入当前布局。
- **US-B02 自定义命令与参数**：作为进阶使用者，我希望填写完整命令及参数，或选用已保存的配置，以便运行自己的 CLI 工作流。
  验收要点：
  - `launch` 未选择时只提供自定义命令，不读取配置列表；浏览器遗留 `configId` 不能转换成猜测命令或另一配置，必须显式选择并填写新命令；
  - 在启动配置与“自定义命令…”之间切换时保留手动输入和已选配置，但只提交当前方式；
  - 手动切换时，命令区平滑开合；开启“减少动态效果”时直接切换。配置读取失败立即显示，已聚焦的命令框保留光标；
  - 空手动命令不能误用保存配置；
  - 正确保留引号、空参数与参数边界，不进行 shell 展开；
  - 未闭合引号或空可执行程序阻止提交；
  - `configId` 与非空 `cmd`、`env` 互斥；
  - 临时命令不落盘，保存配置不可用或仍在加载时可改选“自定义命令…”。
- **US-B03 选择直接目录**：作为使用者，我希望直接指定任意可访问目录并复用最近使用的位置，以便启动普通项目而不必先登记 Git 仓库。
  验收要点：
  - `workspace` 未选择时仍可输入或浏览直接目录、使用最近目录与默认目录；不读取仓库或工作树清单，不显示其位置选项，也不按旧仓库偏好切换项目；
  - 启用项目能力时通过项目列表或“本地文件夹”明确选择目录，普通项目不显示 Git 或额外工作目录字段；关闭项目能力时保留直接目录输入、浏览与最近目录；
  - 从首页的“新建会话”打开时范围收窄：选中仓库只列该仓库的工作树与“新建工作树…”；选中工作树（或点工作树行的“新建会话”）只列该工作树与“从当前工作树分支新建…”，起始分支默认为该工作树的分支；选中“全部仓库”、“直接会话”时先选项目，分屏保持源目录并可显式更换项目；
  - 明确上下文优先：工作树入口或分屏源目录 > 主页选中仓库的可用主工作区（否则其他可用工作树）；
  - 全局项目流程等待明确选择，不以最近目录替代项目；兼容的直接创建路径才按最近目录、服务默认目录依次预填；
  - 主页选中仓库但没有可用工作区时保留仓库并要求选择，不回退到其他项目；对话框内改选位置不改变主页选中的仓库；
  - 保留最近 10 个启动目录，并在成功创建后记住最近一次启动方案（启动配置及混合起始端，或临时命令），下次打开直接填入，对所有位置通用；
  - 记住的配置已删除、不可用或对应本机未安装的 Agent 时显示“选择启动方式”并禁止提交，等待用户显式选择有效命令；创建失败不覆盖记忆；
  - 分屏以源会话的完整启动目录覆盖最近目录，目录未改时保留其工作区归属。
- **US-B04 浏览目录**：作为使用者，我希望逐层浏览和补全服务器目录，以便在手机上也能准确选中启动位置。
  验收要点：
  - 支持路径筛选、主目录、上一级、显示隐藏目录；
  - 支持方向键、Enter/Tab 进入和 Alt+↑ 返回；
  - 显示读取错误并能选择当前目录。
- **US-B05 创建缺失目录**：作为使用者，我希望在目录不存在时确认创建再启动，以便直接为新任务准备工作位置。
  验收要点：
  - 直接目录启动失败且目录不存在时才提示创建；
  - 确认后创建目录并重试；
  - 不因此登记仓库。
- **US-B06 在已有或新工作树启动**：作为使用者，我希望从已有 worktree 启动，或先创建 worktree 再启动 agent，以便在指定的代码副本中工作。
  验收要点：
  - 项目流程的“工作位置”只列当前项目的已有工作树（显示分支与路径）与“新建工作树…”；不包含其他项目或任意目录；直接创建兼容入口保留“位置”选择器；
  - 不可用仓库、不可用或 bare 工作树显示但禁用；
  - 新工作树在位置行下方展开：“创建新分支 / 使用已有分支”、起始分支与新分支名称，起始提交与存放目录收在“高级：起始提交与存放目录”，位置行摘要显示“新分支 ← 起始分支”；改选其他位置再选回同一仓库的“新建工作树…”时保留已填写内容；
  - 位置选择器在目录、已有工作树和新建工作树之间保持同一行高；手动选出新字段后，表单按需滚动到字段；
  - 新工作树成功后可启动会话或仅保留工作树；
  - agent 启动失败时保留新工作树与分支，位置自动变为该工作树，重试不重复创建；改选其他位置后仍可在列表中选回它；
  - 侧栏仓库菜单的“新建工作树…”默认只创建工作树，勾选“创建后启动会话”才显示命令并启动；未勾选时改选已有工作树或目录会禁用“创建工作树”并提示，不会无操作地关闭；
  - 启动配置不可用不阻止单独创建工作树。
- **US-B07 命名会话**：作为使用者，我希望给新会话填写名称，以便在多个相似终端中识别任务。
  验收要点：
  - 创建名称优先于首条提示标题与程序标题（终端 OSC 0/2、聊天端 ACP `session_info_update`）；
  - 未命名且从终端视图启动、填写了用户指令时，以该指令为名（折叠空白，最多 80 个字符，超出加“…”；不含初始化指令，去掉 `{{dir}}`、`{{webmux}}` 引用，普通 Shell 命令同样适用），之后程序标题不再覆盖；名称输入框的占位提示“留空则用首条提示，没有则跟随终端标题”；
  - 没有用户指令（含只有初始化指令）时使用程序标题，再回退到目录名；程序标题同样折叠空白、最多 80 个字符，超出加“…”；聊天端从首条消息取临时标题，见 US-Q02；
  - 首页、终端、聊天页头与浏览器标签使用会话标题；终端或 agent 标题中的 Nerd Font 私有区图标在首页、侧栏、会话行和页头等界面沿用内置图标的字形显示。
- **US-B08 管理启动配置**：作为使用者，我希望把常用的命令、参数和环境变量保存为服务端启动配置，以便在任何设备上一键选用而不必重复输入。
  验收要点：
  - 本故事的存储、预设与设置入口仅在选择 `launch` 时存在；未选择时不打开、锁定、初始化或补写配置文件。混合开关需 `chat`，不因禁用聊天而清除已有混合字段；
  - 已选择的聊天或通知编辑器加载失败时，修改名称等其他字段仍保留已读取的混合模式与提醒策略，不把缺少编辑器当成用户清空选项；
  - CLI 在只选择 `launch` 时可新增并实际运行普通配置，也可修改名称、环境和预览；省略未启用功能拥有的写入字段，保留已存混合与提醒选项。显式聊天开关不因省略而伪装成功；
  - 设置导航合并为单一「启动配置」入口（`#launch-configs`，旧锚点 `#agent-tools` 自动重定向至 `#launch-configs`）；卡片以不带边框的行列表呈现（行高不低于 44px）：
    - 非 Agent 配置与已安装的 Agent 配置展示在主列表中，展示图标、名称与一行配置摘要（终端或混合会话、环境变量数、预览行数与提醒策略；完整命令在编辑对话框中显示），已安装 Agent 行内融合展示当前版本号、解析路径、更新状态、复制升级命令及外部 ACP 连接器子卡片；
    - 整行主体或编辑按钮可打开编辑对话框，行尾“⋯”菜单提供编辑与删除；
    - 本机未安装的受检 Agent 启动配置与无对应配置的未安装工具收纳在底部可折叠的「未安装的工具（N）」区域，保留编辑、删除与复制安装命令入口；新建或补回未安装的 Agent 预设后自动展开该区域；
  - 设置页可新建、编辑、删除自定义启动配置，并选择自动匹配、通用终端或 LobeHub 品牌图标；配置最多 100 条且文件总大小不超过 4 MiB，超出时保存被拒绝并提示精简（400），其余配置与功能不受影响；
  - 图标随配置持久保存并在设置列表、新建会话选择器中显示，缺少图标的旧配置按可识别的直接命令或标准 Bash/Zsh 包装自动匹配，不改写原配置；
  - “加载 shell 启动文件”复选框将普通命令安全包裹为所选 Bash 或 Zsh 的交互式 argv，保留空参数、空格和字面量 shell 字符；
  - 切换“加载 shell 启动文件”时，下方字段保持原位；关掉后 Tab 跳过 shell 选择，并保留命令与环境变量；
  - 重新编辑还原普通命令并保持勾选与所选 shell，保存不重复包裹，取消勾选恢复直接执行；
  - 新建时按服务端只读 `defaultShell` 预选，空值回退 Bash；
  - 包装参数计入 argv 上限；
  - 可为每条配置选择跟随全局默认或单独设置输出预览行数与跳过末尾行数；
  - 切换预览方式或修正行数时，预览区与后续提醒设置保持位置；跟随全局默认时，键盘直接跳过自定义字段；
  - 自定义预览值无效时，切回「跟随全局默认」即可保存配置。
  - 编辑启动命令时，混合支持区只显示当前命令的结论，过长的连接器安装信息在限高滚动区内展示；
  - 可为每条配置选择跟随全局默认或单独设置“完成与介入提醒”：
    - 支持设置提醒范围（`all`/`urgent`/`off`）、四类信号源开关（`OSC 7501`、Warp Agent 事件、`OSC 9·777` 通知、BEL 响铃）、最短回合耗时（`0–300` 秒）与完成确认延迟（`0–60` 秒）；
    - 方向键连续切换提醒范围；选“关闭”时平滑收起信号源与时间阈值，选“仅需介入”时时间阈值原位禁用，范围错误按需即时播报；
    - 编辑器显式说明信号源优先级（`OSC 7501 > Agent 事件 > 桌面通知 > 终端响铃`），配置列表摘要显示“仅需介入”“提醒已关闭”或“自定义提醒”；
  - 新安装首次启动时自动写入 Shell、十二款 CLI agent 与 omp 共 14 条带图标的启动配置（完整列表见 [接入说明](../../cli-agents.md)），其中 omp 附带 `PI_FORCE_IMAGE_PROTOCOL=kitty` 以启用 US-D12 图片显示；
    - Shell 为通用终端，agent 使用本地打包的 LobeHub 图标（Antigravity CLI 为 `antigravity`，同时保留旧 `geminicli` 兼容渲染），omp 使用 Pi；
    - agent 预设按服务端 `$SHELL` 经“加载 shell 启动文件”包裹（Bash 或 Zsh，未设置时 Bash，其他 shell 直接执行）；
    - 各命令的授权参数见接入说明，Claude Code、Codex CLI、Antigravity CLI、Open Code、omp、Qoder、QoderCN、Kimi 与 Devin 为混合模式，可编辑或删除；
  - 文件已存在时启动服务（包括用户编辑或删空后）不再自动补写或覆盖，但可在设置页「新建启动配置」旁的「添加系统预设」菜单中补回单个或全部缺失的系统预设（已有同名配置时菜单项置灰并标记“已存在”，不覆盖用户已修改的配置）；
  - 配置跨重启与设备共享，环境变量值默认遮挡；
  - 添加环境变量后聚焦新变量名；删除后聚焦相邻变量名，删空后回到添加按钮。错误显示在对应行下方，添加按钮固定在列表上方；
  - 按配置 ID 启动，修改或删除配置不影响运行中会话的命令与环境，会话信息保留配置 ID 供首页解析预览；
  - 存储不可用时显示原因，临时命令仍可用。编辑器可勾选混合模式，可用性由服务端判定并显示原因，列表显示“混合”标记（US-Q01）。
- **US-B09 启动 agent 时不卡在“信任此文件夹”**：作为使用者，我希望 Claude Code、Codex、GitHub Copilot CLI、Antigravity CLI、Qoder、QoderCN、Kimi 和 Devin 在新目录启动时不停在信任对话框，以便无人看着的会话也能直接开始工作。
  验收要点：
  - 选择 `launch` 时默认开启 `agent_pretrust`；新建会话的命令被识别为这八款 CLI（直接执行、绝对路径、版本化二进制名或 Bash/Zsh 启动文件包装）时，先把启动目录记入该 CLI 自己的配置，或为 Kimi 注入进程级目录信任，再启动进程；
  - 未选择 `launch` 时，普通 argv 仍可运行，但不装配信任预写或 agent 专属提交延迟，`agent_pretrust = true` 不会隐式开启启动集成；
  - 写入位置优先跟随会话环境变量，其次是服务进程的同名变量，最后回退到主目录下的默认位置：
    - Claude Code：`CLAUDE_CONFIG_DIR` 或 `~/.claude.json`；
    - Codex：`CODEX_HOME` 或 `~/.codex/config.toml`；
    - GitHub Copilot CLI：`COPILOT_HOME` 或 `~/.copilot/config.json`；
    - Antigravity CLI：固定写主目录下的 `~/.gemini/antigravity-cli/settings.json`（`trustedWorkspaces`）；
    - Qoder 与 QoderCN：分别跟随 `QODER_CONFIG_DIR`（默认 `~/.qoder/settings.json`）与 `QODERCN_CONFIG_DIR`（默认 `~/.qoder-cn/settings.json`），写入 `permissions.trustDirectories`（不跟随命令行 `--config-dir`）；
    - Devin：`XDG_DATA_HOME`（默认 `~/.local/share`）下的 `devin/cli/trusted_workspaces.json`，写入 `trusted_paths`；
  - Kimi 不写持久配置，只在会话环境补充 `KIMI_CODE_TRUST_WORKSPACE=1`；恢复 TUI 与 ACP 继承同一环境，会话或服务已显式设置该变量（包括 falsy 值）时不覆盖；
  - 已有配置的其他内容与权限保持不变；已经信任时不写；Codex 保留注释且不改写用户显式设为 `untrusted` 的目录；
  - Claude Code、Copilot、Antigravity CLI、Qoder 与 QoderCN 的配置文件缺失、或无法解析时不创建、不改写；Codex 只在 `~/.codex` 已存在时创建 `config.toml`；Devin 会创建缺失的目录与文件，内容为仅含 `trusted_paths` 的最小 JSON，已有文件的其他字段保持不变；
  - 主目录、其上级目录与文件系统根永不写入或注入；目录先解析符号链接；
  - 任何失败只记录日志，会话照常启动；`agent_pretrust = false` 时不写任何文件，也不注入 Kimi trust 环境；
  - 其他 CLI 与已运行会话不受影响，说明与限制见 [CLI agent 接入](../../cli-agents.md#文件夹信任预写)。

- **US-B10 打开或创建本地项目**：作为使用者，我希望先登记项目再启动会话，以便复用服务端的普通文件夹或 Git 仓库。
  验收要点：
  - 选择项目期间调整窗口宽度或旋转设备，当前步骤、搜索词和输入焦点保留；关闭后重新打开时，按当时的宽度选择对话框或抽屉。
  - 选择 workspace 后提供项目列表、已有目录登记、父目录浏览和新建目录接口；不需要 files、launch 或 chat；
  - 普通目录无需 Git；新建默认只创建目录，显式开启初始化才创建 main 分支的空 Git 仓库，不要求作者身份或生成初始提交；
  - 相同规范路径去重，Git 子目录归一到已登记仓库；不会覆盖既有目标或修改原仓库初始化指令；
  - 创建后登记失败时保留目录并返回恢复路径，可再次登记；认证和 Origin 校验覆盖所有项目接口；
  - workspace.projects 单独报告目录能力，Git 不可用不阻断普通目录，未选择 workspace 时接口不挂载；
  - 全局新建会话以项目列表开始，可搜索名称与路径；无项目时显示来源列表，支持新建与本地文件夹；操作成功进入实际会话设置，重复提交禁用，错误保留输入；
  - 切换项目选择步骤时，主体高度平滑过渡并聚焦当前步骤的输入框。搜索和减少动态效果模式即时更新。
  - 返回来源列表保留各来源草稿；Esc 逐层返回并恢复搜索焦点，根层关闭。
  - 输入名称时即时显示完整路径；长路径在预览区内换行和滚动，名称输入和提交操作保留位置。

- **US-B11 幂等克隆远程 Git 项目**：作为使用者，我希望远程克隆在断线后继续并可恢复登记，以便创建项目时不重复下载或覆盖文件。
  验收要点：
  - workspace.clone 可用时，POST /api/projects/clone 接受 requestId、url、path，先持久记录再返回 202；同一请求与参数返回同一操作，参数不符返回 409；
  - 允许不含凭据/查询参数的 HTTPS、SSH 和 user@host:path；拒绝本地及 ext 协议、控制字符和选项输入，不执行 shell 拼接命令；
  - 两个网络克隆并发，其余排队；浏览器离开不取消任务，服务停止取消并等待整个子进程组；
  - 只在自有临时目录克隆，原子发布绝不覆盖已有目标，即使它是空目录、文件或符号链接；失败只清理本操作创建的临时目录，替换过的目录拒绝递归删除；
  - GET /api/projects/operations/{id} 可重连查询结果；克隆成功但登记失败保留目标，通过 POST /api/projects/operations/{id}/register 单独重试，不再次克隆；
  - Git URL 来源建议未填写的目标目录但保留手动修改；显示克隆进度，刷新或重新打开后恢复跟踪，登记失败可重试；显式选择其他项目会停止本浏览器观察，迟到结果不覆盖新选择；
  - 克隆状态和目标路径在独立滚动区更新；重试登记期间操作保留位置和焦点，停止跟踪后焦点回到当前步骤的输入框。
  - 公开错误不包含 Git stdout/stderr 或服务端凭据；禁用 workspace 时相关路由不挂载。

- **US-B12 通过服务端 GitHub 身份选择仓库**：作为使用者，我希望浏览和搜索服务用户有权访问的仓库，以便选择私有或协作项目并复用现有登录。
  验收要点：
  - GET /api/projects/github 返回服务端登录名、owner/name、描述、私有状态和 HTTPS 克隆地址；默认列出自己的与协作仓库，搜索支持可访问仓库；
  - GitHub 来源可搜索、翻页、选择仓库与目标目录；未安装或未登录时仍可进入配置说明，刷新后重读能力，不探测不可用接口；
  - GitHub 查询等待、失败或无访问权限时保留结果视口；输入新查询后立即撤下上一次查询的候选项。
  - 每页 20 条，页码 1–50；搜索限 200 字节且拒绝控制字符，参数独立传给 gh，不执行 shell；
  - 缺少 gh、身份不可用、权限或网络失败返回 available=false 与可读原因，仓库数组仍为 []；失败不阻断普通目录；
  - 只在显式查询时验证身份与 gh 克隆语法（需要支持 --no-upstream 的 2.88+ 版本）；workspace.github 能力仅检查所需命令与存储，不启动登录或修改认证配置；
  - source=github 的克隆使用服务端 gh 认证，来源限 github.com 的 HTTPS 仓库地址；source 参与幂等身份，普通 Git URL 克隆保持原行为；
  - 请求最长 20 秒，输出有界，取消会终止整个进程组；公开响应不包含 gh 原始输出或凭据；禁用 workspace 时接口不挂载。

- **US-B13 查看本机 Agent 工具与检查可用更新**：作为使用者与部署者，我希望在设置页或 CLI 查看本机已安装的 CLI agent 及外部 ACP 连接器、解析路径、当前版本号与可用更新，并一键复制安装或升级命令，以便无需先开终端逐个排查环境。
  验收要点：
  - 依赖 `launch` 内置功能；禁用时不构造扫描服务，`/api/agent-tools` 与 `/api/agent-tools/check` 经认证与 Origin 校验后返回 404，设置页隐藏 `#launch-configs` 分区与导航且不发请求，`webmux agents` 按 `launch.configs` 能力报功能未开启（退出码 1）；
  - 本机扫描复用服务端交互式启动 Shell（`bash -ic` 或 `zsh -ic`，其他回退进程 `PATH`）单次批量定位候选路径与启动后 `PATH`（5 秒、输出上限 64 KiB），在独立进程组并发执行有界超时（3 秒、输出上限 4 KiB）的 `--version` 采集并做内存 TTL 缓存，`?refresh=1` 或 `--refresh` 强制重扫；
  - 打开 `/settings#launch-configs`（含兼容锚点 `/settings#agent-tools`）或请求 `GET /api/agent-tools` 时零外发网络请求，仅合并未过期的内存远端版本缓存；
  - 仅在点击“检查更新”或执行 `POST /api/agent-tools/check` / `webmux agents --check` 时，才对已安装且具公开包源的工具及连接器查只读 HTTPS 上游（npm 主源失败回退 `registry.npmmirror.com`、PyPI JSON、GitHub Releases），重定向限白名单 HTTPS 域名且不带鉴权头；
  - 上游 JSON 响应上限为 1 MiB，支持含完整发行历史的 Kimi PyPI 元数据；超过上限时该项返回 `check_failed`；
  - 单个上游失败只将该条标记为 `check_failed` 并保留其余结果；无公开稳定包源的自更新二进制（如 Antigravity CLI、Cursor CLI、Devin）明确返回 `source_unavailable`（“由工具自管理更新”），不猜测版本；
  - 已安装的混合模式 agent 或连接器版本号低于档案 `verifiedVersion` 时标记 `belowVerified` 兼容性提示；外部 ACP 连接器（`claude-agent-acp`、`codex-acp`）随主工具以内嵌子卡片或 CLI 缩进子行展示独立路径、版本、更新状态与安装/升级命令；
  - 展开或收起「未安装的工具」时，列表高度连续变化，连按可中途反向。键盘焦点留在触发按钮，收起时跳过列表内的操作。系统开启减少动态效果时立即切换。
  - 设置页在「启动配置」（`#launch-configs`）卡片内将已安装工具状态直接融合进对应启动配置行（无对应启动配置的已安装工具作为独立行展示），未安装 Agent 启动配置与未安装工具默认收起在底部可展开折叠区；仅提供可复制的安装/升级命令与官方文档链接，服务端仅执行只读探测与查询；
  - 重扫或检查更新期间保留上次报告并稳定下方设置位置；
  - `webmux agents [--check] [--refresh] [--all] [--json]` 默认列出已安装工具与连接器（`--all` 包含未安装项），`--json` 输出带 `"v": 1` 的 `AgentToolsV1` 对象。

依据：

- [启动图标](../../../web/src/components/settings/LaunchConfigIcon.svelte)、[LobeHub 图标来源与许可](../../../web/src/icons/lobehub/README.md)
- [Agent 工具目录](../../../internal/agentcatalog/catalog.go)、[本机扫描与更新检查](../../../internal/agentcheck/service.go)、[Agent 工具接口](../../../internal/featureapi/agent_tools.go)
- [CLI agents 命令](../../../internal/clicmd/agents.go)、[设置页启动配置与工具融合卡片](../../../web/src/components/settings/LaunchConfigsSettings.svelte)
- [Agent 工具栏](../../../web/src/components/settings/AgentToolsToolbar.svelte)、[Agent 工具单行](../../../web/src/components/settings/AgentToolRow.svelte)
- [行归并规则](../../../web/src/domains/settings/agent-tools.ts)、[扫描控制器](../../../web/src/hooks/settings/agent-tools-inspector.svelte.ts)、[启动配置可见性规则](../../../web/src/domains/settings/launch-configs.ts)
- [新建会话](../../../web/src/components/sessions/NewSessionDialog.svelte)、[位置选择](../../../web/src/components/workspace/LocationPicker.svelte)、[命令选择](../../../web/src/components/sessions/LaunchPicker.svelte)、[默认值解析](../../../web/src/domains/sessions/new-session-defaults.ts)
- [启动配置编辑](../../../web/src/components/settings/LaunchConfigDialog.svelte)、[系统预设菜单](../../../web/src/components/settings/LaunchPresetMenu.svelte)、[启动配置存储](../../../internal/launch/store.go)、[首次启动预设](../../../internal/launch/presets.go)
- [启动配置接口](../../../internal/featureapi/launch.go)、[CLI agent 接入](../../cli-agents.md)
- [信任预写](../../../internal/agenttrust/agenttrust.go)、[会话钩子](../../../internal/session/manager.go)
- [会话标题来源](../../../internal/hybrid/actor.go)、[核心生命周期](../../../internal/session/lifecycle.go)、[聊天页头](../../../web/src/hooks/sessions/hybrid.svelte.ts)
- [目录选择](../../../web/src/components/workspace/DirectoryPicker.svelte)、[命令解析](../../../internal/session/run_command.go)、[核心创建编排](../../../internal/sessioncreate/service.go)。
- [可选创建端口](../../../internal/sessioncreate/types.go)、[工作区位置贡献](../../../internal/workspace/create_location.go)、[功能创建入口](../../../web/src/features/create.ts)。

来源补充：[统一创建回归](../../../web/test/e2e/session-creation.spec.ts)。

项目来源：[项目 API](../../protocol/http-projects.md)、[本地项目服务](../../../internal/workspace/project_local.go)、[项目接口](../../../internal/featureapi/project.go)、[项目 wire 回归](../../../internal/api/project_test.go)。

远程克隆来源：[克隆服务](../../../internal/workspace/project_clone.go)、[克隆执行](../../../internal/workspace/project_clone_worker.go)、[克隆回归](../../../internal/workspace/project_clone_test.go)。

GitHub 来源：[发现服务](../../../internal/workspace/project_github.go)、[受控进程回归](../../../internal/workspace/project_github_test.go)、[接口契约回归](../../../internal/api/project_github_test.go)。

项目来源界面（US-B10–US-B12）：[Project picker Storybook](../../../web/stories/ProjectPicker.stories.ts) 展示项目列表、四种来源、配置降级及登记失败。
生产入口通过 workspace 创建贡献接入；[项目会话组件回归](../../../web/test/browser/ProjectSessionDialog.test.ts) 覆盖项目切换、上下文与能力降级。
