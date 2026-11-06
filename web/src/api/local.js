// 单机版实现：调 /pos/v1/*（契约已冻结，单机版 server 即 /pos/v1；
// token 逻辑沿用 lib/http.js。
import { createPosApi } from './base.js'

export const api = createPosApi('/pos/v1')
