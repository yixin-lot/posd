// 云端实现：默认同源 /pos/v1（SaaS BFF 与收银台同域部署）。
// 跨域联调时在页面注入 window.__POS_API_BASE__ = 'https://<tenant-host>/pos/v1'。
import { createPosApi } from './base.js'

const base = (typeof window !== 'undefined' && window.__POS_API_BASE__) || '/pos/v1'

export const api = createPosApi(base)
