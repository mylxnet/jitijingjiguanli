// HTTP 客户端封装
// 统一处理 Cookie 认证、错误响应、401 跳转
//
// 统一使用同源相对路径 /api：
// - 开发模式经 vite proxy 转发到后端（web/vite.config.ts 已配 /api → localhost:8080），
//   避免跨端口直连触发 CORS；
// - 生产模式由 Go 二进制同源托管静态资源与 API。

const BASE_URL = '/api'

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
    body: options.body
      ? (options.body instanceof FormData ? options.body : JSON.stringify(options.body))
      : undefined,
  })

  if (res.status === 401) {
    if (onUnauthorized) {
      onUnauthorized()
    }
    throw new Error('请先登录')
  }

  if (!res.ok) {
    const errData = await res.json().catch(() => null)
    const msg = errData?.error?.message || errData?.data?.message || `请求失败 (${res.status})`
    // 构建增强错误对象，保留响应详情供上层判断
    const err: any = new Error(msg)
    err.status = res.status
    err.response = errData?.data || errData?.error || errData
    throw err
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

// 下载文件：返回二进制 Blob 与建议文件名（从 Content-Disposition 解析，缺省用 path 末尾段）
export async function downloadFile(path: string): Promise<{ blob: Blob; filename: string }> {
  const res = await fetch(`${BASE_URL}${path}`, {
    method: 'GET',
    credentials: 'include',
  })
  if (res.status === 401) {
    if (onUnauthorized) onUnauthorized()
    throw new Error('请先登录')
  }
  if (!res.ok) {
    const errData = await res.json().catch(() => null)
    const msg = errData?.error?.message || errData?.data?.message || `请求失败 (${res.status})`
    const err: any = new Error(msg)
    err.status = res.status
    throw err
  }
  // 解析 Content-Disposition: attachment; filename="xxx.db"
  let filename = path.slice(path.lastIndexOf('/') + 1)
  const cd = res.headers.get('Content-Disposition')
  if (cd) {
    const m = cd.match(/filename\*?=(?:"([^"]+)"|([^;]+))/i)
    if (m) filename = (m[1] || m[2] || '').trim() || filename
  }
  return { blob: await res.blob(), filename }
}