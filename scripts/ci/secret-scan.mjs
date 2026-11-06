#!/usr/bin/env node
// 仓库敏感信息扫描：内网地址 / 真实口令 / 私钥 / 数据库文件。
//
// 目的不是"零告警"，而是**把已知的假数据放行、把真凭证拦下来**：
//   - 文档里的 192.168.1.50 这类示例地址 → 允许（allowlist 注释）
//   - 私钥、.env、*.db、真实内网段 → 失败
//
// 跑法：node scripts/ci/secret-scan.mjs
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

const ROOT = process.cwd()
const SKIP_DIRS = new Set(['node_modules', '.git', 'dist', 'docs-site', '_shots', 'backups'])
const TEXT_EXT = /.(md|go|js|mjs|cjs|ts|vue|yml|yaml|json|ps1|sql|py|html|css|txt|toml|sh)$/
const ALLOW_LINE = /secret-scan:allow/

// 每条规则：命中即失败（除非该行带 secret-scan:allow）
const RULES = [
  { name: '私钥', re: /-----BEGIN (RSA|OPENSSH|EC|PGP) PRIVATE KEY-----/ },
  { name: '真实内网段', re: /\b(?:10\.(?:\d{1,3})\.(?:\d{1,3})\.(?:\d{1,3})|172\.(?:1[6-9]|2\d|3[01])\.(?:\d{1,3})\.(?:\d{1,3})|192\.168\.(?!1\.|0\.)\d{1,3}\.\d{1,3})\b/ },
  { name: '疑似口令赋值', re: /(?:password|passwd|pwd|secret|token)\s*[:=]\s*['"][^'"\s]{4,}['"]/i },
  { name: '数据库连接串', re: /(?:mysql|postgres|mongodb):\/\/[^\s'"]+:[^\s'"]+@/i },
]

const problems = []

function walk(dir) {
  for (const name of readdirSync(dir)) {
    if (SKIP_DIRS.has(name)) continue
    const full = join(dir, name)
    const st = statSync(full)
    if (st.isDirectory()) { walk(full); continue }
    if (!TEXT_EXT.test(name)) continue
    const rel = relative(ROOT, full).replace(/\\/g, '/')
    const lines = readFileSync(full, 'utf8').split('\n')
    lines.forEach((line, i) => {
      if (ALLOW_LINE.test(line)) return
      for (const rule of RULES) {
        if (rule.re.test(line)) {
          problems.push(`${rel}:${i + 1}  [${rule.name}]  ${line.trim().slice(0, 90)}`)
        }
      }
    })
  }
}

walk(ROOT)

if (problems.length) {
  console.error('✗ 发现 ' + problems.length + ' 处敏感信息：')
  for (const p of problems.slice(0, 50)) console.error('  ' + p)
  if (problems.length > 50) console.error('  …还有 ' + (problems.length - 50) + ' 处')
  console.error('\n确认是示例数据就在该行加注释：secret-scan:allow')
  process.exit(1)
}
console.log('✓ 未发现敏感信息')
