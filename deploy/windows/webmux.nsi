; WebMux Windows installer: a per-user install of the tray plus the Linux
; server of the same build, pre-seeded where the tray looks for downloads.
; The Linux server carries frpc and its license; the tray installs that
; exact file into the selected WSL distribution.
;
; Built by `make installer`; every input comes from -D:
;   VERSION  release version, x.y.z or x.y.z-dev.n
;   ARCH     amd64 | arm64 (Windows and pre-seeded Linux server)
;   TRAY     absolute path of the tray for ARCH (webmux-tray-windows-<ARCH>.exe,
;            webmux-<VERSION>-tray-windows-<ARCH>.exe for a development version)
;   SERVER   absolute path of the Linux server for ARCH (webmux-linux-<ARCH>,
;            webmux-<VERSION>-linux-<ARCH> for a development version)
;   ICON     absolute path of the installer icon
;   OUTFILE  absolute output path
;
; Exit codes: 0 done, 2 wrong Windows architecture or a failure while no
; tray had been closed (none ran, or it had not been closed yet), 3 a
; running tray could not be closed (nothing was written), 4 a failure after
; the installed tray was closed (that tray was started again).
;
; Silent mode (/S) over an existing install keeps its directory and the
; login item as they are; internal/tray updates itself this way. Silent
; uninstall (/S) keeps tray settings and logs; `/S /PURGE` also removes
; %APPDATA%\WebMux and %LOCALAPPDATA%\WebMux.
;
; The installer itself runs as the user (`RequestExecutionLevel user`).
; Adding a firewall rule (`webmux-tray firewall allow`) prompts for
; administrator confirmation once — either when the optional install checkbox
; is kept checked, or later when enabling LAN access from the tray menu.
; Silent installs never prompt and leave any existing rule untouched; an
; in-place silent update keeps it because the rule follows the tray's path.
; Uninstall checks both the current tray path and any earlier recorded path.

!ifndef VERSION | ARCH | TRAY | SERVER | ICON | OUTFILE
  !error "VERSION, ARCH, TRAY, SERVER, ICON and OUTFILE must be defined with -D"
!endif

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user
; A file that cannot be written fails the install instead of being skipped,
; so .onInstFailed can restart the tray.
AllowSkipFiles off

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"
!include "FileFunc.nsh"
!include "nsDialogs.nsh"

!insertmacro un.GetParameters
!insertmacro un.GetOptions

!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\WebMux"
; Same value name and quoting as internal/tray/traysys/autostart.go, so the
; tray's "登录时自动启动" check box recognises the entry.
!define RUN_KEY "Software\Microsoft\Windows\CurrentVersion\Run"
!define STARTUP_APPROVED_KEY "Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run"
!define RUN_VALUE "WebMuxTray"
!define NOTIFY_ICON_KEY "Control Panel\NotifyIconSettings"
!define MUICACHE_KEY "Software\Classes\Local Settings\Software\Microsoft\Windows\Shell\MuiCache"
!define PCA_STORE_KEY "Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Compatibility Assistant\Store"
!define COMPAT_LAYERS_KEY "Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers"
; FOLDERID_LocalAppData GUID used by Windows 11 NotifyIconSettings ExecutablePath.
!define LOCALAPPDATA_GUID "{F1B32785-6FBA-4FCF-9D55-7B8E7F157091}"
!define LOCAL_DIR "$LOCALAPPDATA\WebMux"
; Same name as Installer.Install in internal/tray/install.go.
!define CACHE_DIR "${LOCAL_DIR}\downloads"
!define CACHE_FILE "webmux-${VERSION}-linux-${ARCH}"
!define UPDATES_DIR "${LOCAL_DIR}\updates"
!define KEEPALIVE_FILE "${LOCAL_DIR}\keepalive.json"
!define ROAMING_DIR "$APPDATA\WebMux"

; The version resource takes four numbers: the part of VERSION before "-".
!searchparse "-${VERSION}-" "-" VERSION_NUMBER "-"

!if "${ARCH}" == "amd64"
  !define OTHER_ARCH "arm64"
!else if "${ARCH}" == "arm64"
  !define OTHER_ARCH "amd64"
!else
  !error "ARCH must be amd64 or arm64"
!endif
; Same file names as the Makefile's release artifacts.
!if "${VERSION_NUMBER}" == "${VERSION}"
  !define OTHER_SETUP "webmux-setup-windows-${OTHER_ARCH}.exe"
!else
  !define OTHER_SETUP "webmux-${VERSION}-setup-windows-${OTHER_ARCH}.exe"
!endif

Name "WebMux"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\WebMux"
InstallDirRegKey HKCU "${UNINST_KEY}" "InstallLocation"
BrandingText "WebMux ${VERSION}"

VIProductVersion "${VERSION_NUMBER}.0"
VIFileVersion "${VERSION_NUMBER}.0"
VIAddVersionKey /LANG=2052 "ProductName" "WebMux"
VIAddVersionKey /LANG=2052 "ProductVersion" "${VERSION}"
VIAddVersionKey /LANG=2052 "FileVersion" "${VERSION}"
VIAddVersionKey /LANG=2052 "FileDescription" "WebMux 安装程序（${ARCH}）"
VIAddVersionKey /LANG=2052 "LegalCopyright" "WebMux contributors"

; ${BST_CHECKED} writes the login item. Silent installs keep the default;
; a silent update leaves the item as it is.
Var Autostart
Var AutostartBox
; ${BST_CHECKED} keeps a firewall rule for the installed tray: the first
; interactive install defaults to it, a later one to the recorded choice.
; Silent installs never touch the rule.
Var Firewall
Var FirewallBox
; The tray path whose rule an earlier install added (FirewallRule), or "".
Var FirewallOld
; 1 once a running installed tray has been closed.
Var TrayClosed
; 1 while webmux-tray.exe.old holds the installed tray, until the install
; succeeds.
Var TrayBackup
; ${BST_CHECKED} removes %APPDATA%\WebMux and %LOCALAPPDATA%\WebMux logs
; during uninstall. Defaults to unchecked; `/PURGE` enables it silently.
Var Purge
Var PurgeBox
Var UnfinishText

!define MUI_ICON "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_DIRECTORY
Page custom optionsPage optionsLeave
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_TEXT "WebMux 托盘已安装并启动。$\r$\n$\r$\n首次使用时，在托盘菜单中选择“安装 WSL 中的 webmux”；安装包已内置 ${ARCH} 版服务端，WSL 为同一架构时无需联网。"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
UninstPage custom un.optionsPage un.optionsLeave
!insertmacro MUI_UNPAGE_INSTFILES
!define MUI_FINISHPAGE_TEXT "$UnfinishText"
!insertmacro MUI_UNPAGE_FINISH
!insertmacro MUI_LANGUAGE "SimpChinese"

; CloseTray asks the tray started from $INSTDIR (and any tray of this user)
; to quit as with "仅退出托盘". `quit` kills an unresponsive tray only when
; it runs from $INSTDIR; a tray elsewhere is the user's to close. EXTRA_ARGS
; lets uninstall also pass `--stop-keepalive`. It returns with $0 = 0 when a
; running tray was closed and 3 when none ran (a tray that predates exit
; code 3 also reports none as 0).
!macro CloseTray UN EXE EXTRA_ARGS
  Function ${UN}closeTray
    retry:
      ClearErrors
      ExecWait '"${EXE}" quit --kill-path "$INSTDIR\webmux-tray.exe"${EXTRA_ARGS}' $0
      ${If} ${Errors}
        StrCpy $0 3 ; no tray executable to ask: nothing of ours can run
      ${ElseIf} "${EXTRA_ARGS}" != ""
      ${AndIf} $0 == 2
        ; Fallback when $INSTDIR\webmux-tray.exe was replaced by an older build.
        ClearErrors
        ExecWait '"${EXE}" quit --kill-path "$INSTDIR\webmux-tray.exe"' $0
        ${If} ${Errors}
          StrCpy $0 3
        ${EndIf}
      ${EndIf}
      ${If} $0 == 0
      ${OrIf} $0 == 3
        Return
      ${EndIf}
      IfSilent fail
      MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "另一个 WebMux 托盘仍在运行，无法自动关闭（可能是旧版本或其他位置的托盘）。$\r$\n$\r$\n请在通知区域右键 WebMux 图标，选择“退出 → 仅退出托盘”，然后点击“重试”。" IDRETRY retry
    fail:
      SetErrorLevel 3
      Quit
  FunctionEnd
!macroend
!insertmacro CloseTray "" "$PLUGINSDIR\webmux-tray.exe" ""
!insertmacro CloseTray "un." "$INSTDIR\webmux-tray.exe" " --stop-keepalive"

Function .onInit
  StrCpy $Autostart ${BST_CHECKED}
  StrCpy $Firewall ${BST_CHECKED}
  ReadRegStr $FirewallOld HKCU "${UNINST_KEY}" "FirewallRule"
  ${If} ${Silent}
    StrCpy $Firewall ${BST_UNCHECKED}
  ${Else}
    ReadRegStr $0 HKCU "${UNINST_KEY}" "InstallLocation"
    ${If} $0 != ""
    ${AndIf} $FirewallOld == ""
      StrCpy $Firewall ${BST_UNCHECKED}
    ${EndIf}
  ${EndIf}
  StrCpy $TrayClosed 0
  StrCpy $TrayBackup 0
  !if "${ARCH}" == "amd64"
    ${IfNot} ${IsNativeAMD64}
  !else
    ${IfNot} ${IsNativeARM64}
  !endif
    IfSilent +2
    MessageBox MB_OK|MB_ICONSTOP "此安装包适用于 ${ARCH} 版 Windows，当前设备请改用 ${OTHER_SETUP}。"
    SetErrorLevel 2
    Quit
  ${EndIf}
  ; A silent update over an existing install (the tray's self-update)
  ; stays in its directory, whatever /D says, and neither re-adds a login
  ; item the user removed nor changes one that exists.
  ${If} ${Silent}
    ReadRegStr $0 HKCU "${UNINST_KEY}" "InstallLocation"
    ${If} $0 != ""
      StrCpy $INSTDIR $0
      StrCpy $Autostart ${BST_UNCHECKED}
    ${EndIf}
  ${EndIf}
FunctionEnd

; A failed write may have left webmux-tray.exe incomplete, so the tray
; comes back from its backup. Any failure after the running tray was
; closed starts it again, so the user keeps a tray; the exit code tells the
; caller the update failed.
Function .onInstFailed
  ${If} $TrayBackup == 1
    Delete "$INSTDIR\webmux-tray.exe"
    Rename "$INSTDIR\webmux-tray.exe.old" "$INSTDIR\webmux-tray.exe"
  ${EndIf}
  ${If} $TrayClosed == 1
    ${If} ${FileExists} "$INSTDIR\webmux-tray.exe"
      SetOutPath "$PROFILE"
      Exec '"$INSTDIR\webmux-tray.exe"'
    ${EndIf}
    SetErrorLevel 4
  ${EndIf}
FunctionEnd

; "安装选项": whether the tray starts when the user signs in to Windows,
; and whether other devices may reach the tray's forwarding.
Function optionsPage
  !insertmacro MUI_HEADER_TEXT "安装选项" "选择开机启动与防火墙设置。"
  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  ${NSD_CreateCheckbox} 0 0 100% 12u "开机后自动启动 WebMux（登录 Windows 时启动托盘）"
  Pop $AutostartBox
  ${NSD_SetState} $AutostartBox $Autostart
  ${NSD_CreateLabel} 12u 14u -12u 24u "托盘启动后会按托盘设置在 WSL 中启动 WebMux。以后可在托盘菜单的“登录时自动启动”中更改。"
  Pop $0
  ${NSD_CreateCheckbox} 0 44u 100% 12u "允许其他设备通过托盘转发访问 WebMux（添加 Windows 防火墙规则）"
  Pop $FirewallBox
  ${NSD_SetState} $FirewallBox $Firewall
  ${NSD_CreateLabel} 12u 58u -12u 36u "只放行专用网络与域网络上对托盘的入站连接，不放行公用网络。安装时会弹出一次管理员确认；取消勾选后仍可在托盘开启“允许局域网访问”时再放行。"
  Pop $0
  nsDialogs::Show
FunctionEnd

Function optionsLeave
  ${NSD_GetState} $AutostartBox $Autostart
  ${NSD_GetState} $FirewallBox $Firewall
FunctionEnd

Function un.onInit
  StrCpy $Purge ${BST_UNCHECKED}
  ${un.GetParameters} $0
  ClearErrors
  ${un.GetOptions} $0 "/PURGE" $1
  ${IfNot} ${Errors}
    StrCpy $Purge ${BST_CHECKED}
  ${EndIf}
FunctionEnd

; "卸载选项": whether to remove personal tray settings and local logs.
Function un.optionsPage
  !insertmacro MUI_HEADER_TEXT "卸载选项" "选择是否一并清除个人配置与本地日志。"
  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  ${NSD_CreateCheckbox} 0 0 100% 12u "同时删除个人配置与本地日志（%APPDATA%\WebMux、%LOCALAPPDATA%\WebMux）"
  Pop $PurgeBox
  ${NSD_SetState} $PurgeBox $Purge
  ${NSD_CreateLabel} 12u 14u -12u 48u "不勾选时保留托盘设置（tray.json）与托盘日志（tray.log），便于以后重新安装时沿用配置。$\r$\n$\r$\n无论是否勾选，卸载都不会停止或删除 WSL 发行版内的 webmux 及其配置。"
  Pop $0
  nsDialogs::Show
FunctionEnd

Function un.optionsLeave
  ${NSD_GetState} $PurgeBox $Purge
FunctionEnd

; Removes Windows Shell, tray-icon history and compatibility cache entries
; referencing this installation's executables without touching other copies.
Function un.cleanShellTraces
  DeleteRegValue HKCU "${MUICACHE_KEY}" "$INSTDIR\webmux-tray.exe.FriendlyAppName"
  DeleteRegValue HKCU "${MUICACHE_KEY}" "$INSTDIR\webmux-tray.exe.ApplicationCompany"
  DeleteRegValue HKCU "${MUICACHE_KEY}" "$INSTDIR\uninstall.exe.FriendlyAppName"
  DeleteRegValue HKCU "${MUICACHE_KEY}" "$INSTDIR\uninstall.exe.ApplicationCompany"
  DeleteRegValue HKCU "${PCA_STORE_KEY}" "$INSTDIR\webmux-tray.exe"
  DeleteRegValue HKCU "${PCA_STORE_KEY}" "$INSTDIR\uninstall.exe"
  DeleteRegValue HKCU "${COMPAT_LAYERS_KEY}" "$INSTDIR\webmux-tray.exe"
  DeleteRegValue HKCU "${COMPAT_LAYERS_KEY}" "$INSTDIR\uninstall.exe"

  StrCpy $3 "$INSTDIR\webmux-tray.exe"
  StrLen $4 "$LOCALAPPDATA\"
  StrCpy $5 "$INSTDIR\webmux-tray.exe" $4
  ${If} $5 == "$LOCALAPPDATA\"
    StrCpy $3 "$INSTDIR\webmux-tray.exe" "" $4
    StrCpy $3 "${LOCALAPPDATA_GUID}\$3"
  ${EndIf}
  StrCpy $0 0
  ${Do}
    EnumRegKey $1 HKCU "${NOTIFY_ICON_KEY}" $0
    ${If} $1 == ""
      ${ExitDo}
    ${EndIf}
    ReadRegStr $2 HKCU "${NOTIFY_ICON_KEY}\$1" "ExecutablePath"
    ${If} $2 == "$INSTDIR\webmux-tray.exe"
    ${OrIf} $2 == $3
      DeleteRegKey HKCU "${NOTIFY_ICON_KEY}\$1"
    ${Else}
      IntOp $0 $0 + 1
    ${EndIf}
  ${Loop}
FunctionEnd

Section "-WebMux tray"
  ; Close the running tray with the new executable: an installed older
  ; tray may not know `quit` yet.
  InitPluginsDir
  File "/oname=$PLUGINSDIR\webmux-tray.exe" "${TRAY}"
  Call closeTray
  ; From here on a failure restores the backup and, when a tray was closed,
  ; restarts it (.onInstFailed): File aborts by itself (AllowSkipFiles
  ; off), other writes are checked through the error flag below.
  ${If} $0 == 0
    StrCpy $TrayClosed 1
  ${EndIf}
  ClearErrors

  SetOutPath "$INSTDIR"
  ; File overwrites in place, so the installed tray moves aside first and
  ; stays until the install succeeds. A backup an interrupted update left
  ; behind is the last complete tray when webmux-tray.exe is missing.
  ${If} ${FileExists} "$INSTDIR\webmux-tray.exe"
    Delete "$INSTDIR\webmux-tray.exe.old"
    Rename "$INSTDIR\webmux-tray.exe" "$INSTDIR\webmux-tray.exe.old"
    ${If} ${Errors}
      Abort "无法写入安装目录 $INSTDIR"
    ${EndIf}
    StrCpy $TrayBackup 1
  ${ElseIf} ${FileExists} "$INSTDIR\webmux-tray.exe.old"
    StrCpy $TrayBackup 1
  ${EndIf}
  File "/oname=webmux-tray.exe" "${TRAY}"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  ${If} ${Errors}
    Abort "无法写入安装目录 $INSTDIR"
  ${EndIf}
  ; The shortcut and removing the old server are best effort.
  CreateShortcut "$SMPROGRAMS\WebMux.lnk" "$INSTDIR\webmux-tray.exe"

  ; Replace the server an earlier version pre-seeded.
  ReadRegStr $1 HKCU "${UNINST_KEY}" "PreseededServer"
  ${If} $1 != ""
  ${AndIf} $1 != "${CACHE_DIR}\${CACHE_FILE}"
    Delete "$1"
  ${EndIf}
  ClearErrors
  CreateDirectory "${CACHE_DIR}"
  File "/oname=${CACHE_DIR}\${CACHE_FILE}" "${SERVER}"

  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "WebMux"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "WebMux"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\webmux-tray.exe"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegStr HKCU "${UNINST_KEY}" "PreseededServer" "${CACHE_DIR}\${CACHE_FILE}"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
  ${If} ${Errors}
    Abort "无法写入下载缓存或注册表"
  ${EndIf}
  StrCpy $TrayBackup 0
  Delete "$INSTDIR\webmux-tray.exe.old"
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" $0

  ${If} $Autostart == ${BST_CHECKED}
    WriteRegStr HKCU "${RUN_KEY}" "${RUN_VALUE}" '"$INSTDIR\webmux-tray.exe"'
  ${EndIf}

  ; Optional and after the install is complete: a declined or failed
  ; prompt only explains how to add the rule later. FirewallRule records
  ; the tray path, so a rule for an earlier directory is removed too.
  ${If} $Firewall == ${BST_CHECKED}
    DetailPrint "添加 Windows 防火墙规则（需要管理员确认）…"
    ClearErrors
    ExecWait '"$INSTDIR\webmux-tray.exe" firewall allow' $0
    ${If} ${Errors}
      StrCpy $0 1
    ${EndIf}
    ClearErrors
    ${If} $0 == 0
      WriteRegStr HKCU "${UNINST_KEY}" "FirewallRule" "$INSTDIR\webmux-tray.exe"
      ${If} $FirewallOld != ""
      ${AndIf} $FirewallOld != "$INSTDIR\webmux-tray.exe"
        ExecWait '"$INSTDIR\webmux-tray.exe" firewall remove "$FirewallOld"' $0
        ClearErrors
      ${EndIf}
    ${ElseIf} $0 == 3
      MessageBox MB_OK|MB_ICONINFORMATION "已跳过 Windows 防火墙规则，其他设备暂时无法通过托盘转发访问。$\r$\n$\r$\n之后可在托盘开启“允许局域网访问”时放行，或在命令行运行：$\r$\n$\"$INSTDIR\webmux-tray.exe$\" firewall allow" /SD IDOK
    ${Else}
      MessageBox MB_OK|MB_ICONEXCLAMATION "未能添加 Windows 防火墙规则（代码 $0）。托盘已安装，本机使用不受影响。$\r$\n$\r$\n之后可在托盘菜单重试，或在命令行运行：$\r$\n$\"$INSTDIR\webmux-tray.exe$\" firewall allow" /SD IDOK
    ${EndIf}
  ${ElseIfNot} ${Silent}
  ${AndIf} $FirewallOld != ""
    ; Unchecked on an interactive reinstall: withdraw the earlier rule.
    DetailPrint "删除 Windows 防火墙规则（需要管理员确认）…"
    ClearErrors
    ExecWait '"$INSTDIR\webmux-tray.exe" firewall remove "$FirewallOld"' $0
    ${If} ${Errors}
      StrCpy $0 1
    ${EndIf}
    ClearErrors
    ${If} $0 == 0
      DeleteRegValue HKCU "${UNINST_KEY}" "FirewallRule"
    ${Else}
      MessageBox MB_OK|MB_ICONINFORMATION "未删除之前添加的 Windows 防火墙规则（代码 $0）。$\r$\n$\r$\n需要时可在命令行运行：$\r$\n$\"$INSTDIR\webmux-tray.exe$\" firewall remove" /SD IDOK
    ${EndIf}
  ${EndIf}
SectionEnd

Section "-Start tray"
  ; The working directory must not be $INSTDIR, or the running tray would
  ; keep it from being removed.
  SetOutPath "$PROFILE"
  Exec '"$INSTDIR\webmux-tray.exe"'
SectionEnd

Section "Uninstall"
  ; Release any working-directory lock on $INSTDIR inherited from Explorer or
  ; third-party uninstallers so RMDir "$INSTDIR" succeeds.
  SetOutPath "$TEMP"
  Call un.closeTray

  ; Check the current and recorded paths, including rules added at runtime.
  ReadRegStr $1 HKCU "${UNINST_KEY}" "FirewallRule"
  StrCpy $2 0
  ${If} ${FileExists} "$INSTDIR\webmux-tray.exe"
    ClearErrors
    ExecWait '"$INSTDIR\webmux-tray.exe" firewall remove' $0
    ${If} ${Errors}
    ${OrIf} $0 != 0
      StrCpy $2 1
    ${EndIf}
    ClearErrors
    ${If} $1 != ""
    ${AndIf} $1 != "$INSTDIR\webmux-tray.exe"
    ${AndIf} $0 != 3
      ExecWait '"$INSTDIR\webmux-tray.exe" firewall remove "$1"' $0
      ${If} ${Errors}
      ${OrIf} $0 != 0
        StrCpy $2 1
      ${EndIf}
      ClearErrors
    ${EndIf}
  ${ElseIf} $1 != ""
    StrCpy $2 1
  ${EndIf}
  ${If} $2 != 0
    MessageBox MB_OK|MB_ICONINFORMATION "未删除 WebMux 添加的 Windows 防火墙规则。$\r$\n$\r$\n可在“高级安全 Windows Defender 防火墙”的入站规则中删除以“WebMux tray forwarding”开头的规则。" /SD IDOK
  ${EndIf}

  ReadRegStr $1 HKCU "${RUN_KEY}" "${RUN_VALUE}"
  ${If} $1 == '"$INSTDIR\webmux-tray.exe"'
  ${OrIf} $1 == "$INSTDIR\webmux-tray.exe"
    DeleteRegValue HKCU "${RUN_KEY}" "${RUN_VALUE}"
    DeleteRegValue HKCU "${STARTUP_APPROVED_KEY}" "${RUN_VALUE}"
  ${ElseIf} $1 == ""
    DeleteRegValue HKCU "${STARTUP_APPROVED_KEY}" "${RUN_VALUE}"
  ${EndIf}

  Call un.cleanShellTraces
  SetRegView 64
  Call un.cleanShellTraces
  SetRegView lastused

  ReadRegStr $1 HKCU "${UNINST_KEY}" "PreseededServer"
  ${If} $1 != ""
    Delete "$1"
  ${EndIf}
  RMDir /r "${CACHE_DIR}"
  RMDir /r "${UPDATES_DIR}"
  Delete "${KEEPALIVE_FILE}"
  Delete "$TEMP\webmux-journal-*.log"

  ${If} $Purge == ${BST_CHECKED}
    RMDir /r "${ROAMING_DIR}"
    RMDir /r "${LOCAL_DIR}"
    StrCpy $UnfinishText "WebMux 托盘已卸载，已删除本地托盘设置（%APPDATA%\WebMux）与托盘日志（%LOCALAPPDATA%\WebMux）。$\r$\n$\r$\nWSL 发行版中的 webmux 与配置未自动删除。若 WebMux 正在运行，它会继续在 WSL 中运行；如需一并停止并清理 WSL 侧文件，可在发行版终端执行：$\r$\nwebmux stop$\r$\nrm -rf ~/.local/bin/webmux ~/.config/webmux ~/.local/state/webmux"
  ${Else}
    RMDir "${ROAMING_DIR}"
    RMDir "${LOCAL_DIR}"
    StrCpy $UnfinishText "WebMux 托盘已卸载。$\r$\n$\r$\n以下内容已保留：托盘设置（%APPDATA%\WebMux）、托盘日志（%LOCALAPPDATA%\WebMux）和 WSL 中的 webmux。$\r$\n$\r$\n如果 WebMux 正在运行，它会继续在 WSL 中运行；需要停止时在 WSL 中执行 webmux stop。"
  ${EndIf}

  Delete "$SMPROGRAMS\WebMux.lnk"
  Delete "$INSTDIR\webmux-tray.exe"
  Delete "$INSTDIR\webmux-tray.exe.old"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "${UNINST_KEY}"
SectionEnd
