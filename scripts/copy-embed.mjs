#!/usr/bin/env node
/* 把前端构建产物拷进 Go 的 embed 目录。
   用法：node scripts/copy-embed.mjs [--check]
   ——CI 里用 --check 只校验目录存在且非空，不写文件。 */
import { cpSync, existsSync, readdirSync, rmSync, mkdirSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const src = join(root, 'web', 'dist')
const dst = join(root, 'internal', 'webui', 'dist')
const checkOnly = process.argv.includes('--check')

if (!existsSync(src) || readdirSync(src).length === 0) {
  console.error('web/dist 不存在或为空：先跑 npm --prefix web run build')
  process.exit(1)
}
if (checkOnly) {
  console.log('embed 目录检查通过（' + readdirSync(src).length + ' 个条目待内嵌）')
  process.exit(0)
}
rmSync(dst, { recursive: true, force: true })
mkdirSync(dst, { recursive: true })
cpSync(src, dst, { recursive: true })
console.log('已拷贝前端产物到 internal/webui/dist（' + readdirSync(dst).length + ' 个条目）')
