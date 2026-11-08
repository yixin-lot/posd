import { translateServerMessage } from '../i18n.js'
// 统一的请求封装：自动带 token、把 401 抛成可识别的错误。
const TOKEN_KEY = 'pos.token'

export class HttpError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export async function request(path, options) {
  const opts = Object.assign({}, options || {})
  const headers = Object.assign({}, opts.headers || {})
  // 默认按 JSON 处理请求体；调用方显式指定 Content-Type（如 text/csv）时尊重之。
  // FormData 由浏览器自己带 boundary，不能手写 Content-Type
  const isForm = typeof FormData !== 'undefined' && opts.body instanceof FormData
  if (opts.body && !isForm && !headers['Content-Type'] && !headers['content-type']) {
    headers['Content-Type'] = 'application/json'
  }
  const token = getToken()
  if (token) headers['Authorization'] = 'Bearer ' + token

  const res = await fetch(path, Object.assign({}, opts, { headers }))
  if (res.status === 204) return null
  const text = await res.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch (e) {
    data = null
  }
  if (!res.ok) {
    // 已登录状态下被踢回 401：通知外层回到登录页（登录接口本身的 401 不算）
    if (res.status === 401 && token) {
      clearToken()
      window.dispatchEvent(new CustomEvent('pos:auth-expired'))
    }
    throw new HttpError(res.status, translateServerMessage((data && data.message) || '请求失败 ' + res.status))
  }
  return data
}
