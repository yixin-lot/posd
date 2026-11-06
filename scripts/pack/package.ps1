# 打包脚本：构建 → 组装 release 目录 → zip →（可选）编译安装包
# 用法：pwsh -File scripts/pack/package.ps1 -Version 0.1.0
param(
  [string]$Version = "0.1.0",
  [string]$Locale = "zh-CN"   # zh-CN = 国内版；en-US = 国际版（界面/小票/货币默认英文+美元）
)
$suffix = if ($Locale -eq "en-US") { "-intl" } else { "" }
$env:VITE_DEFAULT_LOCALE = $Locale

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root

Write-Host "== 1/4 构建前端 ==" -ForegroundColor Cyan
Push-Location web
npm run build
Pop-Location

Write-Host "== 2/4 内嵌前端并构建单文件 ==" -ForegroundColor Cyan
# 应用图标：产出 packaging\pos.ico 与 cmd\posd\rsrc_windows_amd64.syso
# （syso 与 main.go 同目录，go build 会自动链入 exe 资源；安装包再复用同一个 ico）
if (Get-Command python -ErrorAction SilentlyContinue) {
  python scripts\pack\make-app-icon.py
  if ($LASTEXITCODE -ne 0) { throw "生成应用图标失败（exit=$LASTEXITCODE）" }
} else {
  if (-not (Test-Path cmd\posd\rsrc_windows_amd64.syso)) {
    Write-Warning "未找到 python 且缺少 rsrc_windows_amd64.syso：本次构建的 exe 将没有应用图标"
  }
}
if (Test-Path internal\webui\dist) { Remove-Item internal\webui\dist -Recurse -Force }
Copy-Item web\dist internal\webui\dist -Recurse
$env:PATH = "$env:GOROOT\bin;$env:PATH"
$env:GOPROXY = "off"
$env:GOSUMDB = "off"
go build -trimpath -ldflags "-s -w -H=windowsgui -X main.version=$Version" -o dist\posd.exe ./cmd/posd

Write-Host "== 3/4 组装发布目录 ==" -ForegroundColor Cyan
$out = "release\pos-$Version$suffix"
if (Test-Path $out) { Remove-Item $out -Recurse -Force }
New-Item -ItemType Directory -Force -Path "$out\docs" | Out-Null
Copy-Item dist\posd.exe $out
Copy-Item docs\user-guide.md, docs\deployment.md $out\docs
Copy-Item scripts\pack\install.ps1 $out
# Apache-2.0 要求随分发提供许可证副本；未公开前用模板，公开前由 enable-oss 提升到根目录
Copy-Item docs\oss-templates\LICENSE "$out\LICENSE.txt"
Copy-Item docs\oss-templates\NOTICE "$out\NOTICE.txt"
Copy-Item docs\oss-templates\TRADEMARK.md "$out\TRADEMARK.txt"
@"
沪上·真茶铺 POS $Version
=========================
1. 双击 posd.exe 启动（默认 0.0.0.0:8080，同网段手机/平板可直接访问）
2. 浏览器打开 http://<本机IP>:8080，默认账号 admin / 1234（首次登录后请改 PIN）
3. 数据在 posd.exe 同目录的 pos.db，请定期备份（系统已自动备份，保留 7 份）
4. 卸载：结束 posd.exe 进程后删除整个目录即可
"@ | Out-File -FilePath "$out\使用说明.txt" -Encoding utf8

Write-Host "== 3.5/4 生成 SBOM ==" -ForegroundColor Cyan
$env:POS_VERSION = $Version
& pwsh -NoProfile -File scripts/pack/sbom.ps1 -Out "$out\SBOM.md"

$zip = "release\pos-$Version$suffix.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path $out\* -DestinationPath $zip
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash
Write-Host "zip: $zip" -ForegroundColor Green
Write-Host "sha256: $hash" -ForegroundColor Green

Write-Host "== 4/4 编译安装包（需要 Inno Setup 6）==" -ForegroundColor Cyan
# Inno Setup 的常见安装位置（6/7、Program Files / 自定义盘符）
$iscc = @(
  $env:ISCC,
  "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
  "C:\Program Files\Inno Setup 6\ISCC.exe",
  "D:\Programs\Inno Setup 7\ISCC.exe",
  "C:\Program Files\Inno Setup 7\ISCC.exe",
  "C:\Program Files (x86)\Inno Setup 7\ISCC.exe"
) | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
if (-not $iscc) { $iscc = (Get-Command ISCC.exe -ErrorAction SilentlyContinue).Source }
if ($iscc) {
  & $iscc "packaging\pos.iss" "/DMyAppVersion=$Version" "/DMyLocale=$Locale" "/DMySuffix=$suffix"
  if ($LASTEXITCODE -ne 0) { throw "ISCC 编译失败（exit=$LASTEXITCODE），请看上面的报错行" }
  Write-Host "安装包已生成：release\pos-$Version$suffix-setup.exe" -ForegroundColor Green
  # 发布资产的校验和（用户可核对下载完整性）
  $sums = foreach ($f in @("release\pos-$Version$suffix-setup.exe", $zip)) {
    "$((Get-FileHash $f -Algorithm SHA256).Hash.ToLower())  $(Split-Path $f -Leaf)"
  }
  $sums | Out-File -FilePath "release\SHA256SUMS.txt" -Encoding ascii
  Write-Host "校验和：release\SHA256SUMS.txt" -ForegroundColor Green
} else {
  Write-Warning "未找到 ISCC.exe：已生成 zip，安装包需装 Inno Setup 6 后重跑本脚本。"
}
