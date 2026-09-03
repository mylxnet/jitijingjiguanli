// 集体台账 API 类型定义
// 接口契约以本文档为准，前后端同步更新

export interface Category {
  id: number
  name: string
  level: 1 | 2
  parentId: number | null
  status: 'active' | 'inactive'
  balanceType: 'residual' | 'spending'
  openingBalanceCents: number
  includeInReconciliation: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
  balanceCents?: number
  txnCount?: number
  children?: Category[]
}

export interface Transaction {
  id: number
  txnDate: string
  direction: 'income' | 'expense'
  amountCents: number
  categoryId: number
  note: string | null
  status: 'normal' | 'voided'
  createdAt: string
  updatedAt: string
}

export interface TransactionListResponse {
  items: Transaction[]
  total: number
  summary: {
    incomeTotal: number
    expenseTotal: number
    balance: number
  }
}

export interface ApiResponse<T> {
  data: T
}

export interface ApiError {
  error: {
    code: string
    message: string
    details?: Record<string, unknown>
  }
}

// 金额格式化工具
export function formatYuan(cents: number): string {
  return (cents / 100).toLocaleString('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2,
  })
}

export function parseFen(yuan: string): number {
  // 输入 "123.45" 返回 12345
  const cleaned = yuan.replace(/[^0-9.]/g, '')
  const num = parseFloat(cleaned)
  if (isNaN(num)) return 0
  return Math.round(num * 100)
}

export function formatFen(cents: number): string {
  return (cents / 100).toFixed(2)
}

export function formatDate(dateStr: string): string {
  // YYYY-MM-DD -> MM-DD
  return dateStr.slice(5)
}

export function todayStr(): string {
  const d = new Date()
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function currentMonthStr(): string {
  const d = new Date()
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  return `${y}-${m}`
}

export function getMonthRange(month: string): { from: string; to: string } {
  // month format: YYYY-MM
  const [y, m] = month.split('-').map(Number)
  const firstDay = `${month}-01`
  const lastDay = new Date(y, m, 0).getDate()
  const to = `${month}-${String(lastDay).padStart(2, '0')}`
  return { from: firstDay, to }
}