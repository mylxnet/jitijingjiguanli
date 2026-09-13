// 集体台账 API 类型定义
// 接口契约以本文档为准，前后端同步更新

export interface Category {
  id: number
  name: string
  level: 1 | 2
  parentId: number | null
  status: 'active' | 'inactive'
  kind: 'equity' | 'asset' // v0.4：权益 / 资产
  openingBalanceCents: number // v0.5.1：科目期初
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

export type RecvKind = 'rent' | 'dividend' | 'service' | 'reinvest_dividend' | 'other'

export interface Party {
  id: number
  orgId: number
  name: string
  types: ('flow' | 'invest' | 'reinvest' | 'other')[] // 多选类型
  type: 'flow' | 'invest' | 'reinvest' | 'other' | null // 主类型（兼容旧前端，= types[0]）
  contactPhone: string
  areaMu: number // 流转面积（亩，流转企业）
  note: string | null
  createdAt: string
  updatedAt: string
  outstandingCents: number
  deletable?: boolean // 是否可删除（无欠款 + 科目余额0 + 未被引用）
  deleteBlockReason?: string // 不可删除原因（可删时为空）

  // 投资/再投资专属字段（invest / reinvest 类型用）
  investAmountCents: number // 投资本金
  returnRateBps: number // 收益率基点（500 = 5.00%）
  expectedReturnCents: number // 年收益（自动算=本金×收益率/10000，可手动改）

  // 土地流转专属字段（flow 类型用）
  landMu: number // 流转亩数
  landFeePerMuCents: number // 每亩年流转费
  expectedLandFeeCents: number // 总流转费（自动=亩数×每亩费，可改）
  mgmtFeePerMuCents: number // 每亩年管理费
  expectedMgmtFeeCents: number // 总管理费（自动=亩数×每亩管理费，可改）
}

// 再投资去向明细（ReinvestAllocation 子表）
export interface ReinvestAllocation {
  id: number
  partyId: number
  targetName: string
  amountCents: number
  notes: string | null
  createdAt: string
}

export interface Contract {
  id: number
  partyId: number
  fileName: string
  fileSize: number // bytes
  mimeType: string
  contractTitle: string
  contractDate: string | null
  expiresAt: string | null
  fileData?: string // base64 data URL，仅在下载/预览接口返回
  createdAt: string
}

export type ContractType = 'contract' | 'attachment' | 'other'

// 合同到期清单项（GET /contracts/expiring 返回）
export interface ExpiringContract {
  partyId: number
  partyName: string
  type: string
  contractId: number
  contractTitle: string
  fileName: string
  expiresAt: string
  hasExpired: boolean
  daysUntil: number
}


export interface AccrualStandard {
  id: number
  orgId: number
  partyId: number
  partyName: string
  recvKind: RecvKind
  amountCents: number
  active: boolean
}

export interface AccrueResult {
  created: number
  skipped: number
}

export interface AccruePreviewItem {
  kind: RecvKind
  title: string
  partyId: number
  partyName: string
  amountCents: number
  exists: boolean
}

export interface AccruePreview {
  year: number
  items: AccruePreviewItem[]
}

export interface Receivable {
  id: number
  orgId: number
  partyId: number
  partyName: string
  recvYear?: number // v0.4：应收归属年度（批量结转用；待后端返回）
  recvKind: RecvKind
  title: string
  amountCents: number
  incomeCategoryId: number | null
  status: 'open' | 'closed'
  note: string | null
  createdAt: string
  updatedAt: string
  paidCents: number
  writeoffCents: number
  writeoffDate?: string | null
  writeoffNote?: string | null
  writeoffReceiptId?: number | null
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
  method: 'cash' | 'offset' | 'writeoff'
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
export const recvKindLabel: Record<RecvKind, string> = { rent: '流转费', dividend: '投资收益', service: '流转管理费', reinvest_dividend: '再投资收益', other: '其他' }

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
  return '¥ ' + (cents / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
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
  return '¥ ' + (cents / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
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