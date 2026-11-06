# 在目标机器上安装（zip 交付形态）：建快捷方式 + 可选开机自启
# 用法：pwsh -File install.ps1 [-Autostart] [-Port 8080]
param(
  [switch]$Autostart,
  [int]$Port = 8080
)

$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $here "posd.exe"
if (-not (Test-Path $exe)) { throw "找不到 posd.exe，请把本脚本放在解压后的目录里运行" }

$desktop = [Environment]::GetFolderPath("Desktop")
$lnk = Join-Path $desktop "沪上真茶铺POS.lnk"
$ws = New-Object -ComObject WScript.Shell
$s = $ws.CreateShortcut($lnk)
$s.TargetPath = $exe
$s.Arguments = "--addr 0.0.0.0:$Port"
$s.WorkingDirectory = $here
$s.Save()
Write-Host "已创建桌面快捷方式：$lnk" -ForegroundColor Green

if ($Autostart) {
  # 机器级自启（需要管理员）；若没有权限会自动退回到当前用户
  $run = "HKLM:\Software\Microsoft\Windows\CurrentVersion\Run"
  if (-not (Test-Path $run) -or -not (Test-Path $run -ErrorAction SilentlyContinue)) { $run = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" }
  Set-ItemProperty -Path $run -Name "PosD" -Value "\"$exe\" --addr 0.0.0.0:$Port"
  Write-Host "已设置开机自启（HKCU Run）" -ForegroundColor Green
}

# 局域网访问需要放行端口（首次会弹管理员确认）
try {
  New-NetFirewallRule -DisplayName "沪上真茶铺POS $Port" -Direction Inbound -Protocol TCP -LocalPort $Port -Action Allow -ErrorAction Stop | Out-Null
  Write-Host "已放行防火墙端口 $Port" -ForegroundColor Green
} catch {
  Write-Warning "防火墙规则未添加（可能需要管理员权限）：$($_.Exception.Message)"
}

Write-Host ""
Write-Host "下一步：双击桌面图标启动，浏览器打开 http://本机IP:$Port" -ForegroundColor Cyan
Write-Host "默认账号 admin / 1234，登录后请立刻修改 PIN。" -ForegroundColor Cyan
