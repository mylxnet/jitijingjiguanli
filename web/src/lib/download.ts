// 文件下载工具：导出走后端 /api/export（xlsx 带免加工格式，见后端 export 包）。

export type ExportContent = 'transactions' | 'summary' | 'balance_sheet'
export type ExportFormat = 'csv' | 'xlsx'

/**
 * 触发浏览器下载后端导出的文件。
 * @param params 筛选参数（from/to/categoryId/keyword/minAmount/maxAmount，单位分）
 * @param content 导出内容
 * @param format 导出格式
 */
export async function downloadExport(
  params: Record<string, string | number | boolean | undefined>,
  content: ExportContent,
  format: ExportFormat,
): Promise<void> {
  const qs = new URLSearchParams()
  qs.set('content', content)
  qs.set('format', format)
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') qs.set(k, String(v))
  }

  const res = await fetch(`/api/export?${qs.toString()}`, { credentials: 'include' })
  if (res.status === 401) {
    window.location.hash = '#/login' // 会话过期，回登录页
    throw new Error('登录已过期，请重新登录')
  }
  if (!res.ok) {
    const err = await res.json().catch(() => null)
    throw new Error(err?.error?.message || `导出失败 (${res.status})`)
  }

  const blob = await res.blob()
  // 优先取服务端 Content-Disposition 的文件名（RFC 5987 filename*）
  let filename = `集体台账-${content}-${format}`
  const cd = res.headers.get('Content-Disposition') || ''
  const m = cd.match(/filename\*=UTF-8''([^;]+)/i)
  if (m) {
    try {
      filename = decodeURIComponent(m[1])
    } catch {
      /* 保持默认名 */
    }
  } else {
    filename += '.' + format
  }

  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
