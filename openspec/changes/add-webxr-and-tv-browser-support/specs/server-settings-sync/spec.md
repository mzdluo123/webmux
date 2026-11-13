## ADDED Requirements

### Requirement: XR 与 TV 显示偏好仅保存在设备本地

TV 模式、头显浏览器模式开关与 VR 面板布局 SHALL 属于仅当前设备的显示偏好，SHALL NOT 进入服务端设置文档、设置 API、设置 WebSocket 或跨客户端同步。
系统 SHALL 使用现有设备本地存储与运行时校验保存这些偏好，存储不可用或内容损坏 SHALL NOT 阻止会话与终端使用。

这些偏好 SHALL NOT 包含 token、推送订阅、草稿正文、终端输出、代码内容或其他会话敏感数据。另一浏览器 SHALL 使用自己的 TV/XR 显示选择，不继承源设备布局。

#### Scenario: 设置快照不包含显示布局

- **WHEN** 用户开启 TV 模式并保存 VR 面板布局后读取 `GET /api/settings` 或设置同步帧
- **THEN** 响应、文件和帧不包含 TV/头显浏览器模式、面板位置、大小或会话排列字段

#### Scenario: 两台设备独立显示

- **WHEN** 用户在头显中保存 VR 布局，随后在电视浏览器打开同一 WebMux 实例
- **THEN** 电视不应用头显的 VR 布局，其 TV 模式仍由本设备自己的选择决定

#### Scenario: 本地布局损坏

- **WHEN** 当前浏览器中的 TV/XR 显示偏好损坏或版本不兼容
- **THEN** 系统忽略无效偏好并使用可用的默认显示模式，终端连接、草稿和服务端会话不受影响
