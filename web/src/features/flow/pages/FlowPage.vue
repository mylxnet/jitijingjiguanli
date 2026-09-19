<template>
  <div class="fp-page">
    <!-- ========== 土地流转费收入 ========== -->
    <template v-if="activeTab === 'rent'">
      <div class="page-header">
        <div>
          <h2 class="page-title">土地流转费收入</h2>
          <p class="page-sub">{{ rentYear }} 年度流转费收入情况</p>
        </div>
        <van-button size="small" @click="exportCSV('rent')">导出</van-button>
      </div>

      <div class="fp-year-bar">
        <label class="fp-year-label">选择年度：</label>
        <select v-model="rentYear" class="fp-year-select" @change="onRentYearChange">
          <option v-for="y in rentYearOptions" :key="y" :value="y">{{ y }} 年</option>
        </select>
      </div>

      <div class="fp-stats">
        <div class="fp-stat">
          <div class="fp-stat-label">年度应收</div>
          <div class="fp-stat-value">{{ fmt(rentStats.total) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">已收</div>
          <div class="fp-stat-value" style="color:var(--success)">{{ fmt(rentStats.paid) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">未收</div>
          <div class="fp-stat-value" style="color:var(--danger)">{{ fmt(rentStats.unpaid) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">坏账</div>
          <div class="fp-stat-value" style="color:var(--warn)">{{ fmt(rentStats.writeoff) }}</div>
        </div>
        <div class="fp-stat fp-stat--expense">
          <div class="fp-stat-label">转付农户支出</div>
          <div class="fp-stat-value">{{ fmt(rentFarmerExpenseTotal) }}</div>
        </div>
      </div>

      <!-- 明细表格：表头可点击展开/收起，默认隐藏 -->
      <div class="fp-section">
        <div class="fp-table-wrap">
          <table class="fp-table">
            <thead><tr class="fp-th-clickable" @click="rentShowDetail = !rentShowDetail">
              <th>
                <span class="fp-collapse-icon">{{ rentShowDetail ? '▲' : '▼' }}</span>
                单位名称
              </th>
              <th class="num">应收金额</th><th class="num">已收金额</th><th class="num">未收金额</th><th>状态</th>
            </tr></thead>
            <tbody v-show="rentShowDetail">
              <tr v-for="item in rentItems" :key="item.partyId">
                <td>{{ item.partyName }}</td>
                <td class="num">{{ fmt(item.amountCents) }}</td>
                <td class="num">{{ fmt(cashPaidOf(item)) }}</td>
                <td class="num">{{ fmt(item.outstandingCents) }}</td>
                <td>
                  <span class="fp-status" :class="rowStatusClass(item)">{{ rowStatusLabel(item) }}</span>
                </td>
              </tr>
              <tr v-if="rentItems.length === 0"><td colspan="5" class="empty-cell">暂无数据</td></tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- 转付农户支出记录 -->
      <div class="fp-section">
        <div class="fp-section-title">{{ rentYear }} 年度转付农户支出</div>
        <div class="fp-table-wrap">
          <table class="fp-table">
            <thead><tr>
              <th>日期</th><th class="num">金额</th><th>备注</th>
            </tr></thead>
            <tbody>
              <tr v-for="t in rentFarmerExpenses" :key="t.id">
                <td>{{ t.txnDate }}</td>
                <td class="num">{{ fmt(t.amountCents) }}</td>
                <td>{{ t.note || '-' }}</td>
              </tr>
              <tr v-if="rentFarmerExpenses.length === 0"><td colspan="3" class="empty-cell">暂无支出记录</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <!-- ========== 流转管理费 ========== -->
    <template v-if="activeTab === 'service'">
      <div class="page-header">
        <div>
          <h2 class="page-title">流转管理费</h2>
          <p class="page-sub">{{ svcYear }} 年度管理费收支情况</p>
        </div>
        <van-button size="small" @click="exportCSV('service')">导出</van-button>
      </div>

      <div class="fp-year-bar">
        <label class="fp-year-label">选择年度：</label>
        <select v-model="svcYear" class="fp-year-select" @change="onSvcYearChange">
          <option v-for="y in svcYearOptions" :key="y" :value="y">{{ y }} 年</option>
        </select>
      </div>

      <div class="fp-stats">
        <div class="fp-stat">
          <div class="fp-stat-label">年度应收</div>
          <div class="fp-stat-value">{{ fmt(svcStats.total) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">已收</div>
          <div class="fp-stat-value" style="color:var(--success)">{{ fmt(svcStats.paid) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">坏账</div>
          <div class="fp-stat-value" style="color:var(--warn)">{{ fmt(svcStats.writeoff) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">已支出</div>
          <div class="fp-stat-value" style="color:var(--warn)">{{ fmt(svcExpenseTotal) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">可支出</div>
          <div class="fp-stat-value" :style="{color: svcAvailable >= 0 ? 'var(--info)' : 'var(--danger)'}">{{ fmt(svcAvailable) }}</div>
        </div>
      </div>

      <!-- 明细表格：表头可点击展开/收起，默认隐藏 -->
      <div class="fp-section">
        <div class="fp-table-wrap">
          <table class="fp-table">
            <thead><tr class="fp-th-clickable" @click="svcShowDetail = !svcShowDetail">
              <th>
                <span class="fp-collapse-icon">{{ svcShowDetail ? '▲' : '▼' }}</span>
                单位名称
              </th>
              <th class="num">应收金额</th><th class="num">已收金额</th><th class="num">未收金额</th><th>状态</th>
            </tr></thead>
            <tbody v-show="svcShowDetail">
              <tr v-for="item in svcItems" :key="item.partyId">
                <td>{{ item.partyName }}</td>
                <td class="num">{{ fmt(item.amountCents) }}</td>
                <td class="num">{{ fmt(cashPaidOf(item)) }}</td>
                <td class="num">{{ fmt(item.outstandingCents) }}</td>
                <td>
                  <span class="fp-status" :class="rowStatusClass(item)">{{ rowStatusLabel(item) }}</span>
                </td>
              </tr>
              <tr v-if="svcItems.length === 0"><td colspan="5" class="empty-cell">暂无数据</td></tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- 支出记录 -->
      <div class="fp-section">
        <div class="fp-section-title">{{ svcYear }} 年度管理费支出</div>
        <div class="fp-table-wrap">
          <table class="fp-table">
            <thead><tr>
              <th>日期</th><th class="num">金额</th><th>备注</th>
            </tr></thead>
            <tbody>
              <tr v-for="t in svcExpenses" :key="t.id">
                <td>{{ t.txnDate }}</td>
                <td class="num">{{ fmt(t.amountCents) }}</td>
                <td>{{ t.note || '-' }}</td>
              </tr>
              <tr v-if="svcExpenses.length === 0"><td colspan="3" class="empty-cell">暂无支出记录</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../../lib/http'

interface Receivable {
  id: number; partyId: number; partyName: string; recvYear: number;
  recvKind: string; amountCents: number; paidCents: number; writeoffCents?: number;
  outstandingCents: number; status: 'open' | 'partial' | 'paid';
}
interface Transaction {
  id: number; categoryId: number; direction: string;
  amountCents: number; txnDate: string; note: string; status: string;
}

const props = defineProps<{ activeTab: string }>()

const parties = ref<any[]>([])
const allReceivables = ref<Receivable[]>([])
const allTransactions = ref<Transaction[]>([])
const allCategories = ref<any[]>([])

const curYear = new Date().getFullYear()
const rentYear = ref(curYear)
const svcYear = ref(curYear)

// 年份选项由真实数据驱动（应收单年度 + 支出流水年度 + 当年），
// 历年欠款补录后，对应历史年度即可在下拉中选到（不再写死从 START_YEAR 开始）。
function uniqueDesc(arr: number[]): number[] {
  return [...new Set(arr)].sort((a, b) => b - a)
}
function collectYears(recvKind: string, l1Name: string, l2Name: string): number[] {
  const set = new Set<number>()
  allReceivables.value.forEach(r => {
    if (r.recvKind === recvKind && (r.recvYear || 0) > 0) set.add(r.recvYear)
  })
  const cid = findCatId(l1Name, l2Name)
  if (cid > 0) {
    allTransactions.value.forEach(t => {
      if (t.categoryId === cid && t.direction === 'expense' && t.txnDate) {
        const y = Number(t.txnDate.slice(0, 4))
        if (y > 0) set.add(y)
      }
    })
  }
  return uniqueDesc([...set])
}

// 实际有数据的年份（应收/支出），用于进入页面时自动聚焦到最近有欠款或流水的年度
const rentDataYears = computed(() => collectYears('rent', '分配与支出', '土地流转费-转付农户'))
const svcDataYears = computed(() => collectYears('service', '分配与支出', '管理费支出'))
// 下拉选项：当年必在，其次为数据年份
const rentYearOptions = computed(() => uniqueDesc([curYear, ...rentDataYears.value]))
const svcYearOptions = computed(() => uniqueDesc([curYear, ...svcDataYears.value]))

let yearsAutoAdjusted = false
function adjustYearsIfEmpty() {
  if (yearsAutoAdjusted) return
  yearsAutoAdjusted = true
  if (rentDataYears.value.length && !rentDataYears.value.includes(rentYear.value)) {
    rentYear.value = rentDataYears.value[0]
  }
  if (svcDataYears.value.length && !svcDataYears.value.includes(svcYear.value)) {
    svcYear.value = svcDataYears.value[0]
  }
}

const rentShowDetail = ref(false)
const svcShowDetail = ref(false)

const fmt = (c: number) => '¥' + (c / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })

const statusLabel = (s: string) => ({ open: '未收', partial: '部分收', paid: '已收清' }[s] || s)
// 坏账单行：状态显示为「坏账」
const rowStatusLabel = (r: Receivable) => (r.writeoffCents || 0) > 0 ? '坏账' : statusLabel(r.status)
const rowStatusClass = (r: Receivable) => (r.writeoffCents || 0) > 0 ? 'writeoff' : r.status

// ====== 土地流转费收入 ======

// 已收 = 全部核销 − 坏账（坏账非真实收款）
const cashPaidOf = (r: Receivable) => Math.max(0, (r.paidCents || 0) - (r.writeoffCents || 0))
// 正常页面可见：非坏账单，或坏账单但「实际已收 > 0」；全额坏账完全剔除
const visRecv = (r: Receivable) => (r.writeoffCents || 0) === 0 || cashPaidOf(r) > 0

// 显示集：明细与统计共用同一集合
const rentAll = computed(() =>
  allReceivables.value.filter(r => r.recvKind === 'rent' && r.recvYear === rentYear.value && visRecv(r))
)

const rentItems = computed(() => {
  const map = new Map<number, Receivable>()
  for (const r of rentAll.value) {
    map.set(r.partyId, r)
  }
  return Array.from(map.values())
})

const rentStats = computed(() => {
  const items = rentAll.value
  const total = items.reduce((s, r) => s + r.amountCents, 0)
  const paid = items.reduce((s, r) => s + cashPaidOf(r), 0)
  const unpaid = items.reduce((s, r) => s + r.outstandingCents, 0)
  const writeoff = items.reduce((s, r) => s + (r.writeoffCents || 0), 0)
  return { total, paid, unpaid, writeoff }
})

// 按名称动态解析科目 ID（科目 ID 随组织注册而变化，禁止硬编码，见 532 修复同款约定）
function findCatId(l1Name: string, l2Name: string): number {
  for (const l1 of allCategories.value) {
    if (l1.name !== l1Name) continue
    const l2 = (l1.children || []).find((c: any) => c.name === l2Name)
    if (l2) return l2.id
  }
  return -1
}

const rentFarmerExpenses = computed(() => {
  const farmerCatId = findCatId('分配与支出', '土地流转费-转付农户')
  if (farmerCatId < 0) return []
  return allTransactions.value.filter(t => t.categoryId === farmerCatId && t.direction === 'expense' && t.txnDate?.startsWith(String(rentYear.value)))
})

const rentFarmerExpenseTotal = computed(() =>
  rentFarmerExpenses.value.reduce((s, t) => s + t.amountCents, 0)
)

function onRentYearChange() {
  rentShowDetail.value = false
}

// ====== 流转管理费 ======

// 显示集：明细与统计共用同一集合
const svcAll = computed(() =>
  allReceivables.value.filter(r => r.recvKind === 'service' && r.recvYear === svcYear.value && visRecv(r))
)

const svcItems = computed(() => {
  const map = new Map<number, Receivable>()
  for (const r of svcAll.value) {
    map.set(r.partyId, r)
  }
  return Array.from(map.values())
})

const svcStats = computed(() => {
  const items = svcAll.value
  const total = items.reduce((s, r) => s + r.amountCents, 0)
  const paid = items.reduce((s, r) => s + cashPaidOf(r), 0)
  const unpaid = items.reduce((s, r) => s + r.outstandingCents, 0)
  const writeoff = items.reduce((s, r) => s + (r.writeoffCents || 0), 0)
  return { total, paid, unpaid, writeoff }
})

const svcExpenses = computed(() => {
  const expCatId = findCatId('分配与支出', '管理费支出')
  if (expCatId < 0) return []
  return allTransactions.value.filter(t => t.categoryId === expCatId && t.direction === 'expense' && t.txnDate?.startsWith(String(svcYear.value)))
})

const svcExpenseTotal = computed(() =>
  svcExpenses.value.reduce((s, t) => s + t.amountCents, 0)
)

const svcAvailable = computed(() => {
  // 流转管理费科目（L1，按名称动态解析）的余额 = 累计收入 - 累计支出
  const mgmtFeeCat = allCategories.value.find((c: any) => c.name === '流转管理费' && c.level === 1)
  if (!mgmtFeeCat) return 0
  const childrenBalance = (mgmtFeeCat.children || []).reduce((s: number, c: any) => s + (c.balanceCents || 0), 0)
  return childrenBalance - svcExpenseTotal.value
})

function onSvcYearChange() {
  svcShowDetail.value = false
}

// 导出
function downloadCSV(filename: string, headers: string[], rows: string[][]) {
  const csv = [headers.join(','), ...rows.map(r => r.map(c => '"' + (c || '').replace(/"/g, '""') + '"').join(','))].join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = filename; a.click()
  URL.revokeObjectURL(url)
}

function exportCSV(tab: string) {
  if (tab === 'rent') {
    downloadCSV('土地流转费收入.csv',
      ['单位名称', '应收金额', '已收金额', '未收金额', '状态'],
      rentItems.value.map(i => [i.partyName, fmt(i.amountCents), fmt(i.paidCents), fmt(i.outstandingCents), statusLabel(i.status)]))
  } else if (tab === 'service') {
    downloadCSV('流转管理费.csv',
      ['日期', '金额', '备注'],
      svcExpenses.value.map(t => [t.txnDate, fmt(t.amountCents), t.note || '']))
  }
}

async function load() {
  try {
    const [recvR, txnR, catR] = await Promise.all([
      api.get<{ data: { items: Receivable[] } } | { items: Receivable[] }>('/receivables'),
      api.get<{ data: { items: Transaction[] } } | { items: Transaction[] }>('/transactions'),
      api.get<any[]>('/categories'),
    ])
    const recvItems = (recvR as any)?.data?.items || (recvR as any)?.items || []
    const txnItems = (txnR as any)?.data?.items || (txnR as any)?.items || []
    allReceivables.value = Array.isArray(recvItems) ? recvItems : []
    allTransactions.value = Array.isArray(txnItems) ? txnItems : []
    allCategories.value = Array.isArray(catR) ? catR : (catR as any)?.data || []
  } catch (e) {
    console.error('load error', e)
  }
  adjustYearsIfEmpty()
}

onMounted(load)
</script>

<style scoped>
.page-header { margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; }
.page-title { font-size: 18px; font-weight: 600; margin: 0; }
.page-sub   { font-size: 12px; color: var(--ink-muted); margin: 4px 0 0; }

.fp-year-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; }
.fp-year-label { font-size: 13px; color: var(--ink-soft); white-space: nowrap; }
.fp-year-select {
  padding: 6px 10px; border: 1px solid var(--line); border-radius: 6px; font-size: 14px;
  background: #fff; outline: none; min-width: 120px;
}
.fp-year-select:focus { border-color: var(--info); }

.fp-stats {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 10px; margin-bottom: 14px;
}
.fp-stat {
  background: #fff; border: 1px solid var(--line-soft); border-radius: 8px; padding: 10px 12px;
}
.fp-stat-label { font-size: 12px; color: var(--ink-muted); }
.fp-stat-value { font-size: 16px; font-weight: 600; color: var(--ink-900, var(--ink)); margin-top: 2px; font-variant-numeric: tabular-nums; }
.fp-stats .fp-stat { border-color: transparent; }
.fp-stats .fp-stat:nth-child(1) { background: var(--warn-bg); }
.fp-stats .fp-stat:nth-child(2) { background: var(--success-bg); }
.fp-stats .fp-stat:nth-child(3) { background: var(--danger-bg); }
.fp-stats .fp-stat:nth-child(4) { background: var(--info-bg); }
/* 转付农户支出=支出口径：浅砖红底 + AA 深色文字（配对 token） */
.fp-stats .fp-stat--expense { background: var(--danger-bg); }
.fp-stats .fp-stat--expense .fp-stat-value { color: var(--danger-deep); }

.fp-section { margin-bottom: 20px; }
.fp-section-title {
  font-size: 14px; font-weight: 600; color: var(--ink);
  margin-bottom: 8px; padding: 0 2px;
  display: flex; align-items: center; gap: 8px;
}
.fp-section-title.clickable { cursor: pointer; user-select: none; }
.fp-section-title.clickable:hover { color: var(--info); }
.fp-collapse-icon { font-size: 10px; color: var(--ink-muted); }

.fp-table-wrap { overflow-x: auto; border: 1px solid var(--line-soft); border-radius: 8px; background: #fff; }
.fp-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.fp-table th {
  position: sticky; top: 0; z-index: 1;
  background: var(--paper-deep); padding: 10px 12px; text-align: left; font-weight: 600; color: var(--ink-soft);
  border-bottom: 1px solid var(--line); white-space: nowrap;
}
.fp-table td { padding: 10px 12px; border-bottom: 1px solid var(--line-soft); color: var(--ink); }
.fp-table tr:last-child td { border-bottom: none; }
.fp-table td.num { text-align: right; font-variant-numeric: tabular-nums; }
.fp-table th.num { text-align: right; }
.fp-table .empty-cell { text-align: center; color: var(--ink-muted); padding: 24px; }
.fp-th-clickable { cursor: pointer; user-select: none; }
.fp-th-clickable:hover { background: var(--paper-deep); }

.fp-status {
  display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px;
}
.fp-status.open { background: var(--danger-bg); color: var(--danger-deep); }
.fp-status.partial { background: var(--warn-bg); color: var(--warn); }
.fp-status.paid { background: var(--success-bg); color: var(--success); }
.fp-status.writeoff { background: var(--warn-bg); color: var(--warn); }

@media (max-width: 600px) {
  .fp-stats { grid-template-columns: repeat(2, 1fr); }
  .fp-table th, .fp-table td { padding: 8px 8px; font-size: 12px; }
}
</style>