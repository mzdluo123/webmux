# Design

## Context

`SessionTerminal` 的根节点是纵向 flex：`TerminalHeader`、`TerminalNotices`（多条 `shrink-0` 的条带）、混合会话连接器提示、`TerminalWorkspace`。条带占用高度，`TerminalWorkspace` 的 `flex-1` 随之伸缩；终端宿主的 `ResizeObserver` 在防抖后 fit 并发送 0x01 resize 帧。

## Goals / Non-Goals

**Goals:**

- 提示显示、关闭、更换内容时终端宿主的宽高不变。

**Non-Goals:**

- 不重做提示的视觉样式、文案或触发条件。
- 不改变 `TerminalNotices` 的属性与行为。

## Decisions

### 1. 零高度锚点容器 + 绝对定位内层

在 `TerminalNotices` 与连接器提示外包一层 `relative z-20 h-0 shrink-0` 的容器，内层用 `absolute inset-x-0 top-0 flex flex-col bg-background` 承载全部条带。

零高度容器不贡献布局高度，`TerminalWorkspace` 的尺寸与有无提示无关；内层没有提示时高度为 0，不遮挡任何内容。`bg-background` 让半透明的错误条带（`bg-destructive/10`）不透出终端文字。

备选：保留文档流并给终端预留固定高度。拒绝原因：浪费空间且无法适应不定数量的提示。

备选：每条提示各自 `fixed` 定位。拒绝原因：多条提示需要自行堆叠，且与顶栏高度耦合。

## Risks / Trade-offs

- 悬浮提示遮住终端最上面几行 → 提示均可关闭或随状态消失；退出、会话不存在等较高的提示出现时终端内容本就不再变化。
- 内层位于 `TerminalWorkspace` 之前的 DOM 位置，`z-20` 保证盖过终端与分隔条；弹窗与菜单的层级更高，不受影响。
