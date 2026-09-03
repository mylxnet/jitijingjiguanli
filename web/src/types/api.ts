// 集体台账 API 类型定义
// 接口契约以本文档为准，前后端同步更新

export interface Category {
  id: number
  name: string
  level: 1 | 2
  parentId: number | null
  status: 'active' | 'inactive'
  balanceType: 'residual' | 'spending'
  kind: 'normal' | 'asset'
  openingBalanceCents: number
  includeInReconciliation: boolean
  preset?: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
  balanceCents?: number
  txnCount?: number
  children?: Category[]
}

export interface FundMove {
  id: number
  orgId: number
  moveDate: string
  kind: 'invest' | 'recover'
  assetCategoryId: number
  amountCents: number
  note: string | null
  status: 'normal' | 'voided'
  createdAt: string
  updatedAt: string
}

export interface FundMoveListResponse {
  items: FundMove[]
  total: number
}

export type PartyKind = 'household' | 'unit'
export type RecvKind = 'rent' | 'dividend' | 'other'

export interface Party {
  id: number
  orgId: number
  name: string
  kind: PartyKind
  note: string | null
  createdAt: string
  updatedAt: string
  outstandingCents: number
}

export interface Receivable {
  id: number
  orgId: number
  partyId: number
  partyName: string
  recvKind: RecvKind
  title: string
  amountCents: number
  incomeCategoryId: number | null
  status: 'open' | 'closed'
  note: string | null
  createdAt: string
  updatedAt: string
  paidCents: number
  outstandingCents: number
}

export interface ReceivableListResponse {
  items: Receivable[]
  total: number
}

export interface Receipt {
  id: number
  orgId: number
  receivableId: number
  amountCents: number
  receiptDate: string
  method: 'cash' | 'offset'
  txnId: number | null
  note: string | null
  status: 'normal' | 'voided'
  createdAt: string
  updatedAt: string
}

export interface ReceivableDetail {
  receivable: Receivable
  receipts: Receipt[]
}

// 展示标签
export const partyKindLabel: Record<PartyKind, string> = { household: '农户', unit: '单位' }
export const recvKindLabel: Record<RecvKind, string> = { rent: '流转费', dividend: '投资收益', other: '其他' }

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