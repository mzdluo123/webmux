# Spec Delta

## MODIFIED Requirements

### Requirement: 安装版识别

只有从安装程序登记的安装目录运行的托盘 SHALL 视为安装版，可以自动更新自身。其他位置的托盘为便携版：发现新版本时 SHALL 只在状态行提示，并在菜单提供“下载托盘 <版本>”，用默认浏览器打开该版本所在的 Release 页面（正式版为标签 `v<版本>`，开发版为 Release `dev`），SHALL NOT 下载或运行安装程序。

#### Scenario: 便携版

- **WHEN** 用户运行 `Downloads\webmux-tray-windows-amd64.exe` 0.4.0，最新版本为 0.5.0
- **THEN** 状态行提示有新版本，菜单出现“下载托盘 0.5.0”，没有下载安装程序

#### Scenario: 便携版开发版通道

- **WHEN** 便携版托盘通道为 `dev`，最新版本为 0.10.3-dev.4
- **THEN** 菜单出现“下载托盘 0.10.3-dev.4”，点击后打开 Release `dev` 的页面
