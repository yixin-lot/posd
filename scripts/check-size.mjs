#!/usr/bin/env node
/**
 *
 * 用法：
 *   node scripts/check-size.mjs              # 检查 dist/posd.zip（默认）
 *   node scripts/check-size.mjs --file <p>   # 检查指定产物
 *   node scripts/check-size.mjs --limit 30   # 自定义阈值（MB，默认 30）
 *
 * 阈值：30 MB（ADR-005，安装包 ≤ 30 MB 是产品指标）。
 * 超了先看是不是又引了大依赖，别先调阈值。
 *
 * 组成说明（"谁占的"）：
 *   - 安装包 = dist/posd.zip（发布物，只含 posd.exe，解压即用）
 *   - 二进制本体 = dist/posd.exe（zip 内唯一内容，故 zip 大小 ≈ exe 压缩后）
 *   - 前端资源 = web/dist 原始体积（打进 exe 的 embed 部分，参考值）
 * 报告输出三者，便于判断体积来自哪一端。
 */
import { existsSync, statSync, readdirSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const distDir = join(root, 'dist')
const defaultFile = join(distDir, 'posd.zip')
const defaultLimitMB = 30

function parseArgs(argv) {
  const out = { file: defaultFile, limitMB: defaultLimitMB }
  for (let i = 2; i < argv.length; i++) {
    const a = argv[i]
    if (a === '--file') out.file = resolve(root, argv[++i])
    else if (a === '--limit') out.limitMB = Number(argv[++i])
    else {
      console.error(`未知参数: ${a}`)
      process.exit(2)
    }
  }
  return out
}

function mb(bytes) {
  return (bytes / (1024 * 1024)).toFixed(2)
}

const { file, limitMB } = parseArgs(process.argv)

if (!existsSync(file)) {
  console.error(`[check-size] 找不到产物: ${file}`)
  console.error(`  请先运行: node scripts/package.mjs`)
  process.exit(1)
}

const zipSize = statSync(file).size

// 二进制本体：优先 dist/posd.exe；没有则从 zip 内成员推断
let exeSize = null
const exePath = join(distDir, 'posd.exe')
if (existsSync(exePath)) {
  exeSize = statSync(exePath).size
}

// 前端资源体积：只算真正 embed 进二进制的 internal/webui/dist。
// （web/dist 是构建产物源，内容与其相同；两个都加会翻倍。）
let webDistSize = 0
const embedDir = join(root, 'internal', 'webui', 'dist')
if (existsSync(embedDir)) {
  const walk = (p) => {
    for (const e of readdirSync(p, { withFileTypes: true })) {
      const fp = join(p, e.name)
      if (e.isDirectory()) walk(fp)
      else webDistSize += statSync(fp).size
    }
  }
  walk(embedDir)
}


// 报告
const line = '─'.repeat(46)
console.log(`[check-size] ${file}`)
console.log(line)
console.log(`  安装包 zip     ${mb(zipSize).padStart(9)} MB  (阈值 ${limitMB} MB)`)
if (exeSize !== null) {
  const ratio = zipSize > 0 ? (100 - (zipSize / exeSize) * 100).toFixed(1) : '0.0'
  console.log(`  二进制 exe     ${mb(exeSize).padStart(9)} MB  (zip 压缩省 ${ratio}%)`)
}
console.log(`  前端 embed     ${mb(webDistSize).padStart(9)} MB  (internal/webui/dist，已打进 exe)`)
console.log(line)

const ok = zipSize <= limitMB * 1024 * 1024
console.log(ok ? `结果：PASS ✅（${mb(zipSize)} MB ≤ ${limitMB} MB）` : `结果：FAIL ❌（${mb(zipSize)} MB > ${limitMB} MB）`)
if (!ok) {
  console.error('提示：先看是否引入了大依赖，别先调阈值。')
  process.exit(1)
}
