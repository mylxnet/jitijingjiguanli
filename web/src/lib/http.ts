// HTTP 客户端封装
// 统一处理 Cookie 认证、错误响应、401 跳转

const BASE_URL = import.meta.env.DEV
  ? 'http://localhost:8080/api'
  : '/api'

interface RequestOptions {
  method?: string
  body?: unknown
  params?: Record<string, string | number | boolean | undefined>
}

let onUnauthorized: (() => void) | null = null

export function setOnUnauthorized(fn: () => void) {
  onUnauthorized = fn
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  let url = `${BASE_URL}${path}`

  // 构建查询参数
  if (options.params) {
    const searchParams = new URLSearchParams()
    for (const [key, value] of Object.entries(options.params)) {
      if (value !== undefined && value !== '') {
        searchParams.append(key, String(value))
      }
    }
    const qs = searchParams.toString()
    if (qs) url += '?' + qs
  }

  const headers: Record<string, string> = {}
  if (options.body && !(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }

  const res = await fetch(url, {
    method: options.method || 'GET',
    headers,
    credentials: 'include', // 携带 Cookie
    body: options.body ? JSON.stringify(options.body) : undefined,
  })

  if (res.status === 401) {
    if (onUnauthorized) {
      onUnauthorized()
    }
    throw new Error('请先登录')
  }

  if (!res.ok) {
    const errData = await res.json().catch(() => null)
    const msg = errData?.error?.message || `请求失败 (${res.status})`
    throw new Error(msg)
  }

  return res.json()
}

// 便捷方法
export const api = {
  get: <T>(path: string, params?: Record<string, string | number | boolean | undefined>) =>
    request<T>(path, { params }),

  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body }),

  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body }),

  del: <T>(path: string) =>
    request<T>(path, { method: 'DELETE' }),
}