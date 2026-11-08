# 生成 SBOM（软件物料清单）：Go 模块 + 前端依赖
# 用法：pwsh -File scripts/pack/sbom.ps1 -Out release\SBOM.md
param([string]$Out = "release/SBOM.md")

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$env:PATH = "$env:GOROOT\bin;$env:PATH"
$env:GOPROXY = "off"; $env:GOSUMDB = "off"

$sb = New-Object System.Text.StringBuilder
[void]$sb.AppendLine("# SBOM · posd")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("生成时间：" + (Get-Date -Format "yyyy-MM-dd HH:mm:ss"))
[void]$sb.AppendLine("版本：$env:POS_VERSION")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("## Go 模块")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("| 模块 | 版本 |")
[void]$sb.AppendLine("|---|---|")
# 用二进制里的构建信息：离线可用，且就是"实际打进包里的"模块（go list -m all 需要联网解析图）
$exe = "dist/posd.exe"
if (Test-Path $exe) {
  & go version -m $exe | Where-Object { $_ -match "^\s*dep\s" } | ForEach-Object {
    $parts = ($_ -replace "^\s*dep\s+", "") -split "\s+"
    if ($parts.Count -ge 2) { [void]$sb.AppendLine("| " + $parts[0] + " | " + $parts[1] + " |") }
  }
} else {
  [void]$sb.AppendLine("| （未找到 dist/posd.exe，先构建再生成 SBOM） | |")
}
[void]$sb.AppendLine("")
[void]$sb.AppendLine("## 前端依赖（生产）")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("| 包 | 版本 |")
[void]$sb.AppendLine("|---|---|")
Push-Location web
$json = npm ls --omit=dev --depth=0 --json 2>$null | ConvertFrom-Json
foreach ($p in $json.dependencies.PSObject.Properties) {
  [void]$sb.AppendLine("| " + $p.Name + " | " + $p.Value.version + " |")
}
Pop-Location
[void]$sb.AppendLine("")
[void]$sb.AppendLine("## 许可证说明")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("本项目以 Apache-2.0 发布。第三方组件许可证见各依赖仓库；")
[void]$sb.AppendLine("CI 中的 ``scripts/ci/license-check.mjs`` 会在引入 GPL/AGPL 依赖时失败。")

$dir = Split-Path -Parent $Out
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
$sb.ToString() | Out-File -FilePath $Out -Encoding utf8
Write-Host "SBOM 已生成：$Out" -ForegroundColor Green
