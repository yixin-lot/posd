#!/usr/bin/env node
// 依赖许可证检查：Go 模块 + 前端生产依赖。
//
// 为什么要有：一个 GPL/AGPL 依赖就能逼整个项目开源，而它往往藏在
// 第三层间接依赖里。把它焊进 CI，新增依赖导致不合规时直接构建失败。
//
// 跑法：node scripts/ci/license-check.mjs
import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

const ALLOW = ['Apache-2.0', 'MIT', 'BSD-2-Clause', 'BSD-3-Clause', 'ISC', 'MPL-2.0', 'Unlicense']
const DENY = ['GPL-2.0', 'GPL-3.0', 'AGPL-3.0', 'LGPL-2.1', 'LGPL-3.0', 'SSPL', 'Commons-Clause']

const seen = new Set()
const problems = []
const missing = []

function classify(text) {
  const t = text.toLowerCase()
  if (t.includes('gnu affero general public license')) return 'AGPL-3.0'
  if (t.includes('gnu lesser general public license')) return 'LGPL'
  if (t.includes('gnu general public license')) return 'GPL'
  if (t.includes('apache license')) return 'Apache-2.0'
  if (t.includes('mozilla public license')) return 'MPL-2.0'
  if (t.includes('permission is hereby granted, free of charge')) return 'MIT'
  if (t.includes('redistribution and use in source and binary forms')) {
    if (t.includes('neither the name')) return 'BSD-3-Clause'
    return 'BSD-2-Clause'
  }
  if (t.includes('permission to use, copy, modify, and/or distribute')) return 'ISC'
  if (t.includes('this is free and unencumbered software')) return 'Unlicense'
  return ''
}

// ---- Go 模块 ----
// go list -m -json 输出的是「连续 JSON 对象」，不是数组。
// 用括号深度切分，避免字符串里出现 } 时被切坏。
function splitJSONObjects(text) {
  const out = []
  let depth = 0, start = -1, inStr = false, esc = false
  for (let i = 0; i < text.length; i++) {
    const c = text[i]
    if (inStr) {
      if (esc) esc = false
      else if (c === '\\') esc = true
      else if (c === '"') inStr = false
      continue
    }
    if (c === '"') { inStr = true; continue }
    if (c === '{') { if (depth === 0) start = i; depth++ }
    else if (c === '}') {
      depth--
      if (depth === 0 && start >= 0) { out.push(text.slice(start, i + 1)); start = -1 }
    }
  }
  return out
}

// 离线读 go.mod（go list 在 GOPROXY=off 时会因为个别模块没缓存而整体失败）。
const gomod = readFileSync(join(process.cwd(), 'go.mod'), 'utf8')
const modRe = /^\s*([\w.\-~/]+\.[\w.\-~/]+)\s+(v[^\s]+)(\s+\/\/ indirect)?\s*$/gm
const mods = []
for (const m of gomod.matchAll(modRe)) mods.push({ Path: m[1], Version: m[2] })

for (const m of mods) {
  if (!m.Path || m.Main) continue
  if (seen.has(m.Path + '@' + m.Version)) continue
  seen.add(m.Path + '@' + m.Version)
  // GOMODCACHE 下模块路径转义：大写字母 → !小写
  const cache = execFileSync('go', ['env', 'GOMODCACHE'], { encoding: 'utf8' }).trim()
  const escaped = m.Path.replace(/[A-Z]/g, (c) => '!' + c.toLowerCase())
  const dir = join(cache, escaped + '@' + m.Version)
  if (!existsSync(dir)) { missing.push(m.Path + '（未下载）'); continue }
  const candidates = ['LICENSE', 'LICENSE.txt', 'LICENSE.md', 'LICENSE-MIT', 'COPYING']
  const found = candidates.map((f) => join(dir, f)).find((p) => existsSync(p))
  if (!found) { missing.push(m.Path); continue }
  const license = classify(readFileSync(found, 'utf8'))
  if (!license) { missing.push(m.Path); continue }
  if (DENY.some((d) => license.startsWith(d))) problems.push(`${m.Path} = ${license}`)
  else if (!ALLOW.includes(license)) problems.push(`${m.Path} = ${license}（不在白名单）`)
}

// ---- 前端生产依赖 ----
const webDir = join(process.cwd(), 'web')
const pkgPath = join(webDir, 'package.json')
if (existsSync(pkgPath)) {
  const pkg = JSON.parse(readFileSync(pkgPath, 'utf8'))
  const deps = Object.keys(pkg.dependencies || {})
  for (const name of deps) {
    const p = join(webDir, 'node_modules', name, 'package.json')
    if (!existsSync(p)) continue
    const dep = JSON.parse(readFileSync(p, 'utf8'))
    const lic2 = typeof dep.license === 'string' ? dep.license : (dep.license && dep.license.type) || ''
    if (!lic2) { missing.push(name + '（无 license 字段）'); continue }
    if (DENY.some((d) => lic2.startsWith(d))) problems.push(`${name} = ${lic2}`)
    else if (!ALLOW.some((a) => lic2.startsWith(a))) problems.push(`${name} = ${lic2}（不在白名单）`)
  }
}

console.log(`检查 ${seen.size} 个 Go 模块 + 前端生产依赖`)
if (missing.length) console.log('无法识别许可证（人工确认）:', [...new Set(missing)].join(', '))
if (problems.length) {
  console.error('\n✗ 存在不合规依赖：')
  for (const p of problems) console.error('  - ' + p)
  process.exit(1)
}
console.log('✓ 许可证全部合规')
