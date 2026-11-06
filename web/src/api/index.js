// 适配层入口：按运行模式选实现，UI 只从这里取实例。
// 模式优先级：window.__POS_MODE__（运行时注入）> 构建期 __POS_MODE__（vite define，来自 POS_MODE 环境变量）> 默认 standalone。
import { api as localApi } from './local.js'
import { api as cloudApi } from './cloud.js'

function detectMode() {
  if (typeof window !== 'undefined') {
    if (window.__POS_MODE__ === 'cloud' || window.__POS_MODE__ === 'standalone') {
      return window.__POS_MODE__
    }
  }
  if (typeof __POS_MODE__ !== 'undefined' && __POS_MODE__ === 'cloud') {
    return 'cloud'
  }
  return 'standalone'
}

export function createApi(mode) {
  const m = mode || detectMode()
  return m === 'cloud' ? cloudApi : localApi
}

// 单例：应用启动时按模式定好，组件直接 import。
export const api = createApi()
