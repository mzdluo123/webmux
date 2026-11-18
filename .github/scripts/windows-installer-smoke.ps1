# Smoke test of the Windows installer on a real Windows user session:
# silent install, `webmux-tray quit` (including `--stop-keepalive`),
# `webmux-tray firewall` (allow/remove only when elevated), upgrade over a
# running tray, refusal beside a foreign tray, wrong-architecture refusal,
# default uninstall cleanup and `/PURGE` personal-data cleanup.
#
# It installs into the current user's profile and removes its test files on
# completion; run it on a CI runner or a throwaway account.
param(
  [Parameter(Mandatory)] [string] $Setup,          # installer for this machine
  [Parameter(Mandatory)] [string] $SetupOtherArch, # installer for the other arch
  [Parameter(Mandatory)] [string] $ServerSha256,   # SHA-256 of the bundled server
  [Parameter(Mandatory)] [string] $Version,
  [Parameter(Mandatory)] [string] $Arch
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\WebMux'
$tray = Join-Path $installDir 'webmux-tray.exe'
$uninstaller = Join-Path $installDir 'uninstall.exe'
$shortcut = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\WebMux.lnk'
$uninstKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\WebMux'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$startupApprovedKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run'
$notifyIconRoot = 'HKCU:\Control Panel\NotifyIconSettings'
$muiCacheKey = 'HKCU:\Software\Classes\Local Settings\Software\Microsoft\Windows\Shell\MuiCache'
$localDir = Join-Path $env:LOCALAPPDATA 'WebMux'
$roamingDir = Join-Path $env:APPDATA 'WebMux'
$downloadsDir = Join-Path $localDir 'downloads'
$updatesDir = Join-Path $localDir 'updates'
$keepaliveFile = Join-Path $localDir 'keepalive.json'
$preseed = Join-Path $downloadsDir "webmux-$Version-linux-$Arch"
$work = Join-Path ([IO.Path]::GetTempPath()) ('webmux-smoke-' + [guid]::NewGuid())
New-Item -ItemType Directory $work | Out-Null

function Check([bool] $ok, [string] $what) {
  if (-not $ok) { throw "FAILED: $what" }
  Write-Host "ok - $what"
}

# Waits for the process only: Windows PowerShell's Start-Process -Wait also
# waits for descendants, such as the tray the installer starts. Arguments
# are joined unquoted by Start-Process, so quote any with spaces.
function Run([string] $exe, [string[]] $argv, [string] $cwd = '') {
  $quoted = @($argv | ForEach-Object { if ($_ -match '\s') { "`"$_`"" } else { $_ } })
  $params = @{ FilePath = $exe; ArgumentList = $quoted; PassThru = $true; WindowStyle = 'Hidden' }
  if ($cwd) { $params.WorkingDirectory = $cwd }
  $p = Start-Process @params
  $null = $p.Handle # keeps the exit code readable after exit
  $p.WaitForExit()
  return $p.ExitCode
}

# Always an array (the leading comma stops PowerShell from unrolling it).
function TrayProcesses([string] $path) {
  , @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
      try { $_.Path -and ($_.Path -ieq $path) } catch { $false }
    })
}

function WaitFor([scriptblock] $cond, [int] $seconds = 15) {
  $deadline = (Get-Date).AddSeconds($seconds)
  while ((Get-Date) -lt $deadline) {
    if (& $cond) { return $true }
    Start-Sleep -Milliseconds 200
  }
  return [bool](& $cond)
}

function RegValue([string] $key, [string] $name) {
  try { Get-ItemPropertyValue -LiteralPath $key -Name $name -ErrorAction Stop } catch { $null }
}

function RunValue {
  RegValue $runKey 'WebMuxTray'
}

# A stand-in for an older tray: it holds the instance mutex but has no quit
# event, so it never answers `quit`. With another mutex name it also plays a
# detached background keepalive process.
$sid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
Add-Type -OutputType WindowsApplication -OutputAssembly (Join-Path $work 'fake.exe') -TypeDefinition @'
using System.Threading;
static class FakeTray {
  static void Main(string[] args) {
    var m = new Mutex(false, args[0]);
    Thread.Sleep(Timeout.Infinite);
    System.GC.KeepAlive(m);
  }
}
'@
# Matches traysys.Identify: creation FILETIME converted to Unix nanoseconds.
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class WinProc {
  [DllImport("kernel32.dll", SetLastError = true)]
  static extern IntPtr OpenProcess(uint access, bool inherit, uint pid);
  [DllImport("kernel32.dll", SetLastError = true)]
  static extern bool CloseHandle(IntPtr h);
  [DllImport("kernel32.dll", SetLastError = true)]
  static extern bool GetProcessTimes(IntPtr h, out long creation, out long exit, out long kernel, out long user);
  public static string Identity(int pid) {
    IntPtr h = OpenProcess(0x1000, false, (uint)pid);
    if (h == IntPtr.Zero) throw new InvalidOperationException("OpenProcess failed");
    try {
      long c, e, k, u;
      if (!GetProcessTimes(h, out c, out e, out k, out u)) throw new InvalidOperationException("GetProcessTimes failed");
      return ((c - 116444736000000000L) * 100L).ToString();
    } finally {
      CloseHandle(h);
    }
  }
}
'@
$fakeDir = Join-Path $work 'other'
New-Item -ItemType Directory $fakeDir | Out-Null
$fake = Join-Path $fakeDir 'webmux-tray.exe'
Copy-Item (Join-Path $work 'fake.exe') $fake
function MutexExists {
  $m = $null
  if ([Threading.Mutex]::TryOpenExisting("Local\WebMuxTray-$sid", [ref] $m)) { $m.Dispose(); return $true }
  return $false
}
function StartFake {
  $p = Start-Process -FilePath $fake -ArgumentList "Local\WebMuxTray-$sid" -PassThru
  if (-not (WaitFor { MutexExists } 5)) { throw 'fake tray did not create the instance mutex' }
  return $p
}
function StartFakeKeepalive {
  $p = Start-Process -FilePath $fake -ArgumentList "Local\WebMuxKeepalive-$sid" -PassThru
  New-Item -ItemType Directory -Force $localDir | Out-Null
  $id = [WinProc]::Identity($p.Id)
  [IO.File]::WriteAllText($keepaliveFile, "{`"pid`":$($p.Id),`"identity`":`"$id`"}")
  return $p
}

try {
  # Wrong architecture: refused before anything is written.
  Check ((Run $SetupOtherArch @('/S')) -eq 2) 'other-architecture installer exits 2'
  Check (-not (Test-Path $installDir) -and -not (Test-Path $uninstKey)) 'other-architecture installer writes nothing'

  # A foreign tray that cannot be closed: refused before anything is written.
  $foreign = StartFake
  Check ((Run $Setup @('/S')) -eq 3) 'installer exits 3 beside a foreign unresponsive tray'
  Check (-not (Test-Path $installDir) -and -not (Test-Path $uninstKey)) 'refused installer writes nothing'
  Check (-not $foreign.HasExited) 'foreign tray left running'
  Stop-Process -Id $foreign.Id

  # Fresh silent install.
  Check ((Run $Setup @('/S')) -eq 0) 'silent install exits 0'
  Check ((Test-Path $tray) -and (Test-Path $uninstaller)) 'tray and uninstaller installed'
  Check (Test-Path $shortcut) 'Start menu shortcut created'
  $u = Get-ItemProperty $uninstKey
  Check ($u.DisplayName -eq 'WebMux' -and $u.DisplayVersion -eq $Version) 'uninstall entry name and version'
  Check ($u.InstallLocation -ieq $installDir -and $u.QuietUninstallString -like '*uninstall.exe" /S') 'uninstall entry paths'
  Check ((RunValue) -ieq "`"$tray`"") 'login item points to the installed tray'
  Check ((Get-FileHash $preseed -Algorithm SHA256).Hash -ieq $ServerSha256) 'bundled server pre-seeded in the tray download cache'
  Check (WaitFor { (TrayProcesses $tray).Count -eq 1 }) 'installer started the tray'

  # quit: the installed tray exits; with no tray, quit returns at once.
  Check ((Run $tray @('quit')) -eq 0) 'quit exits 0 when the tray exits'
  Check ((TrayProcesses $tray).Count -eq 0) 'tray process gone after quit'
  $t = Measure-Command { $code = Run $tray @('quit') }
  Check ($code -eq 3 -and $t.TotalSeconds -lt 5) 'quit without a tray exits 3 at once'
  Check ((Run $tray @('quit', '--bogus')) -eq 2) 'quit rejects unknown arguments'

  # quit --stop-keepalive ends the recorded keepalive and removes keepalive.json.
  $ka = StartFakeKeepalive
  Check ((Run $tray @('quit', '--stop-keepalive')) -eq 3) 'quit --stop-keepalive without a tray exits 3'
  Check (WaitFor { $ka.HasExited } 5) 'keepalive process ended by quit --stop-keepalive'
  Check (-not (Test-Path $keepaliveFile)) 'keepalive record removed by quit --stop-keepalive'

  # quit and an unresponsive tray: only the --kill-path one is ended.
  $old = StartFake
  Check ((Run $tray @('quit', '--kill-path', $tray)) -eq 1) 'quit exits 1 when an unresponsive tray runs elsewhere'
  Check (-not $old.HasExited) 'unresponsive tray at another path survives'
  Check ((Run $tray @('quit', '--kill-path', $fake)) -eq 0) 'quit --kill-path ends the unresponsive tray at that path'
  Check (WaitFor { $old.HasExited } 5) 'unresponsive tray terminated'

  # firewall: a silent install adds no rule. Elevated (CI runners), allow
  # adds the rule for exactly this tray and remove deletes it again. The
  # caller wraps FirewallRules in @() since an empty result unrolls to $null.
  function FirewallRules {
    Get-NetFirewallApplicationFilter -Program $tray -ErrorAction SilentlyContinue |
      Get-NetFirewallRule -ErrorAction SilentlyContinue |
      Where-Object { $_.DisplayName -like 'WebMux tray forwarding *' }
  }
  Check (@(FirewallRules).Count -eq 0) 'silent install adds no firewall rule'
  Check ((Run $tray @('firewall')) -eq 2) 'firewall rejects missing arguments'
  Check ((Run $tray @('firewall', 'allow', 'C:\Other\program.exe')) -eq 2) 'firewall allow only applies to this tray'
  Check ((Run $tray @('firewall', 'remove', 'webmux-tray.exe')) -eq 2) 'firewall remove rejects a relative path'
  Check ((Run $tray @('firewall', 'remove')) -eq 0) 'firewall remove without a rule exits 0'
  $identity = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
  $isAdmin = $identity.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
  if ($isAdmin) {
    Check ((Run $tray @('firewall', 'allow')) -eq 0) 'firewall allow exits 0'
    $rule = @(FirewallRules)
    Check ($rule.Count -eq 1 -and "$($rule[0].Direction)" -eq 'Inbound' -and "$($rule[0].Action)" -eq 'Allow' -and "$($rule[0].Profile)" -eq 'Domain, Private' -and "$($rule[0].Enabled)" -eq 'True') 'rule allows inbound to this tray on private and domain networks only'
    Check ((Get-ItemProperty $uninstKey).FirewallRule -ieq $tray) 'firewall allow records FirewallRule on an installed copy'
    Check ((Run $tray @('firewall', 'allow')) -eq 0 -and @(FirewallRules).Count -eq 1) 'firewall allow is idempotent'

    Set-NetFirewallRule -Name $rule[0].Name -Enabled False
    Check ((Run $tray @('firewall', 'allow')) -eq 0) 'firewall allow repairs a disabled rule'
    $rule = @(FirewallRules)
    Check ($rule.Count -eq 1 -and "$($rule[0].Enabled)" -eq 'True' -and "$($rule[0].Action)" -eq 'Allow') 'disabled rule repaired without duplicates'

    Set-NetFirewallRule -Name $rule[0].Name -Action Block
    Check ((Run $tray @('firewall', 'allow')) -eq 0) 'firewall allow repairs a rule changed to Block'
    $rule = @(FirewallRules)
    Check ($rule.Count -eq 1 -and "$($rule[0].Action)" -eq 'Allow' -and "$($rule[0].Profile)" -eq 'Domain, Private') 'block-tampered rule replaced cleanly'

    $blockRule = New-NetFirewallRule -DisplayName 'WebMux Smoke Block' -Direction Inbound -Action Block -Protocol TCP -Profile Private -Program $tray
    try {
      Check ((Run $tray @('firewall', 'allow', '--clear-blocks')) -eq 0) 'firewall allow --clear-blocks exits 0'
      $remainingBlock = @(Get-NetFirewallRule -Name $blockRule.Name -ErrorAction SilentlyContinue)
      Check ($remainingBlock.Count -eq 0 -and @(FirewallRules).Count -eq 1) 'conflicting local inbound block rule removed'
    } finally {
      Remove-NetFirewallRule -Name $blockRule.Name -ErrorAction SilentlyContinue
    }

    Check ((Run $tray @('firewall', 'remove')) -eq 0 -and @(FirewallRules).Count -eq 0) 'firewall remove deletes the rule'
    Check ($null -eq (Get-ItemProperty $uninstKey -Name FirewallRule -ErrorAction SilentlyContinue)) 'firewall remove clears FirewallRule record'
  } else {
    Write-Host 'skip - firewall allow and remove need an elevated session'
  }

  # Upgrade over a running tray.
  Start-Process -FilePath $tray | Out-Null
  Check (WaitFor { (TrayProcesses $tray).Count -eq 1 }) 'tray running before upgrade'
  $before = (TrayProcesses $tray)[0].Id
  Check ((Run $Setup @('/S')) -eq 0) 'upgrade over a running tray exits 0'
  Check (WaitFor { $p = TrayProcesses $tray; $p.Count -eq 1 -and $p[0].Id -ne $before }) 'old tray replaced by a new one'

  # Default uninstall cleans caches, keepalive and an unrecorded firewall rule,
  # while keeping personal settings, logs and a login item pointing elsewhere.
  $settings = Join-Path $roamingDir 'tray.json'
  New-Item -ItemType Directory -Force $roamingDir | Out-Null
  if (-not (Test-Path $settings)) { New-Item -ItemType File -Force $settings | Out-Null }
  New-Item -ItemType Directory -Force $downloadsDir | Out-Null
  New-Item -ItemType File -Force (Join-Path $downloadsDir 'webmux-0.0.0-linux-arm64') | Out-Null
  New-Item -ItemType Directory -Force $updatesDir | Out-Null
  New-Item -ItemType File -Force (Join-Path $updatesDir 'webmux-setup-windows-amd64.exe') | Out-Null
  $kaUninstall = StartFakeKeepalive
  if ($isAdmin) {
    Check ((Run $tray @('firewall', 'allow')) -eq 0 -and @(FirewallRules).Count -eq 1) 'runtime firewall rule added before uninstall'
    Remove-ItemProperty $uninstKey FirewallRule
  }
  Set-ItemProperty $runKey WebMuxTray "`"$fake`""
  New-Item -Path $startupApprovedKey -Force | Out-Null
  Set-ItemProperty $startupApprovedKey WebMuxTray ([byte[]](2,0,0,0,0,0,0,0,0,0,0,0))
  $notifySubkey = Join-Path $notifyIconRoot 'WebMuxSmokeTest'
  New-Item -Path $notifySubkey -Force | Out-Null
  Set-ItemProperty $notifySubkey ExecutablePath '{F1B32785-6FBA-4FCF-9D55-7B8E7F157091}\Programs\WebMux\webmux-tray.exe'
  New-Item -Path $muiCacheKey -Force | Out-Null
  Set-ItemProperty $muiCacheKey "$tray.FriendlyAppName" 'WebMux'
  $tempJournal = Join-Path $env:TEMP 'webmux-journal-smoke.log'
  New-Item -ItemType File -Force $tempJournal | Out-Null
  Check ((Run $uninstaller @('/S')) -eq 0) 'silent uninstall exits 0'
  Check (WaitFor { -not (Test-Path $uninstKey) }) 'uninstall entry removed'
  Check ((TrayProcesses $tray).Count -eq 0) 'tray closed by the uninstaller'
  Check (-not (Test-Path $installDir) -and -not (Test-Path $shortcut)) 'install directory and shortcut removed'
  Check (-not (Test-Path $downloadsDir) -and -not (Test-Path $updatesDir)) 'downloads and updates caches removed recursively'
  Check (-not (Test-Path $tempJournal)) 'exported journal temp files removed on uninstall'
  Check (-not (Test-Path $notifySubkey)) 'NotifyIconSettings entry for installed tray removed'
  Check ($null -eq (RegValue $muiCacheKey "$tray.FriendlyAppName")) 'MuiCache entry for installed tray removed'
  Check ((WaitFor { $kaUninstall.HasExited } 5) -and -not (Test-Path $keepaliveFile)) 'keepalive process and record removed on uninstall'
  if ($isAdmin) {
    Check (@(FirewallRules).Count -eq 0) 'unrecorded firewall rule removed on uninstall'
  }
  Check ((Test-Path $settings) -and (Test-Path (Join-Path $localDir 'tray.log'))) 'tray settings and log kept by default'
  Check ((RunValue) -ieq "`"$fake`"" -and $null -ne (RegValue $startupApprovedKey 'WebMuxTray')) 'login item and StartupApproved pointing elsewhere kept'
  Remove-ItemProperty $runKey WebMuxTray

  # When %LOCALAPPDATA%\WebMux holds only caches (no logs left) and the
  # uninstaller starts with CWD = $installDir (like Explorer / Geek Uninstaller),
  # default uninstall removes $installDir, empty %LOCALAPPDATA%\WebMux and
  # StartupApproved\Run\WebMuxTray.
  Check ((Run $Setup @('/S')) -eq 0) 'reinstall before empty-dir check exits 0'
  Check (WaitFor { (TrayProcesses $tray).Count -eq 1 }) 'tray started after reinstall'
  Check ((Run $tray @('quit')) -eq 0) 'tray stopped before removing logs'
  Remove-Item (Join-Path $localDir 'tray.log*') -Force
  Check (-not (Test-Path (Join-Path $localDir 'tray.log')) -and (Test-Path $downloadsDir)) 'only caches remain in %LOCALAPPDATA%\WebMux'
  Check ((Run $uninstaller @('/S') $installDir) -eq 0) 'silent uninstall from $installDir CWD exits 0'
  Check (WaitFor { -not (Test-Path $uninstKey) }) 'uninstall entry removed after empty-dir check'
  Check (-not (Test-Path $installDir)) 'install directory removed even when uninstaller CWD started in $installDir'
  Check (-not (Test-Path $localDir)) 'empty %LOCALAPPDATA%\WebMux directory removed'
  Check ($null -eq (RunValue) -and $null -eq (RegValue $startupApprovedKey 'WebMuxTray')) 'Run and StartupApproved entries removed for installed tray'
  Check (Test-Path $settings) 'settings still kept without /PURGE'

  # Silent uninstall with /PURGE removes %APPDATA%\WebMux and %LOCALAPPDATA%\WebMux
  # completely (including atomic temp files, rotated logs and extra files).
  Check ((Run $Setup @('/S')) -eq 0) 'reinstall before /PURGE check exits 0'
  Check (WaitFor { (TrayProcesses $tray).Count -eq 1 }) 'tray started before /PURGE check'
  New-Item -ItemType File -Force (Join-Path $roamingDir '.tray-12345.json') | Out-Null
  New-Item -ItemType File -Force (Join-Path $roamingDir 'extra.bak') | Out-Null
  New-Item -ItemType File -Force (Join-Path $localDir 'tray.log.1') | Out-Null
  New-Item -ItemType File -Force (Join-Path $localDir 'extra.tmp') | Out-Null
  Check ((Test-Path $settings) -and (Test-Path (Join-Path $localDir 'tray.log'))) 'settings and log present before /PURGE'
  Check ((Run $uninstaller @('/S', '/PURGE') $installDir) -eq 0) 'silent /PURGE uninstall exits 0'
  Check (WaitFor { -not (Test-Path $uninstKey) }) 'uninstall entry removed after /PURGE'
  Check (-not (Test-Path $roamingDir) -and -not (Test-Path $localDir)) 'personal settings and local logs removed by /PURGE'
} finally {
  Get-Process -ErrorAction SilentlyContinue | Where-Object {
    try { $_.Path -and $_.Path.StartsWith($work, [StringComparison]::OrdinalIgnoreCase) } catch { $false }
  } | Stop-Process -Force -ErrorAction SilentlyContinue
  Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}
Write-Host 'windows installer smoke: all checks passed'
