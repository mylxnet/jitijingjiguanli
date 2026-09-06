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
          <option v-for="y in yearOptions" :key="y" :value="y">{{ y }} 年</option>
        </select>
      </div>

      <div class="fp-stats">
        <div class="fp-stat">
          <div class="fp-stat-label">年度应收</div>
          <div class="fp-stat-value">{{ fmt(rentStats.total) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">已收</div>
          <div class="fp-stat-value" style="color:#07c160">{{ fmt(rentStats.paid) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">未收</div>
          <div class="fp-stat-value" style="color:#ee0a24">{{ fmt(rentStats.unpaid) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">转付农户支出</div>
          <div class="fp-stat-value" style="color:#ff6034">{{ fmt(rentFarmerExpenseTotal) }}</div>
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
                <td class="num">{{ fmt(item.paidCents) }}</td>
                <td class="num">{{ fmt(item.outstandingCents) }}</td>
                <td>
                  <span class="fp-status" :class="item.status">{{ statusLabel(item.status) }}</span>
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
          <option v-for="y in yearOptions" :key="y" :value="y">{{ y }} 年</option>
        </select>
      </div>

      <div class="fp-stats">
        <div class="fp-stat">
          <div class="fp-stat-label">年度应收</div>
          <div class="fp-stat-value">{{ fmt(svcStats.total) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">已收</div>
          <div class="fp-stat-value" style="color:#07c160">{{ fmt(svcStats.paid) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">未收</div>
          <div class="fp-stat-value" style="color:#ee0a24">{{ fmt(svcStats.unpaid) }}</div>
        </div>
        <div class="fp-stat">
          <div class="fp-stat-label">剩余</div>
          <div class="fp-stat-value" :style="{color: svcRemaining >= 0 ? '#1989fa' : '#ee0a24'}">{{ fmt(svcRemaining) }}</div>
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
                <td class="num">{{ fmt(item.paidCents) }}</td>
                <td class="num">{{ fmt(item.outstandingCents) }}</td>
                <td>
                  <span class="fp-status" :class="item.status">{{ statusLabel(item.status) }}</span>
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
  kind: string; amountCents: number; paidCents: number;
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

const START_YEAR = 2026
const yearOptions = computed(() => {
  const cur = new Date().getFullYear()
  const years: number[] = []
  for (let y = cur; y >= START_YEAR; y--) years.push(y)
  return years
})

const rentYear = ref(new Date().getFullYear())
const svcYear = ref(new Date().getFullYear())

const rentShowDetail = ref(false)
const svcShowDetail = ref(false)

const fmt = (c: number) => '¥' + (c / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })

const statusLabel = (s: string) => ({ open: '未收', partial: '部分收', paid: '已收清' }[s] || s)

// ====== 土地流转费收入 ======

const rentReceivables = computed(() =>
  allReceivables.value.filter(r => r.kind === 'rent' && r.recvYear === rentYear.value)
)

const rentItems = computed(() => {
  const map = new Map<number, Receivable>()
  for (const r of rentReceivables.value) {
    map.set(r.partyId, r)
  }
  return Array.from(map.values())
})

const rentStats = computed(() => {
  const items = rentReceivables.value
  const total = items.reduce((s, r) => s + r.amountCents, 0)
  const paid = items.reduce((s, r) => s + r.paidCents, 0)
  const unpaid = items.reduce((s, r) => s + r.outstandingCents, 0)
  return { total, paid, unpaid }
})

const rentFarmerExpenses = computed(() =>
  allTransactions.value.filter(t => t.categoryId === 81 && t.direction === 'expense' && t.txnDate?.startsWith(String(rentYear.value)))
)

const rentFarmerExpenseTotal = computed(() =>
  rentFarmerExpenses.value.reduce((s, t) => s + t.amountCents, 0)
)

function onRentYearChange() {
  rentShowDetail.value = false
}

// ====== 流转管理费 ======

const svcReceivables = computed(() =>
  allReceivables.value.filter(r => r.kind === 'service' && r.recvYear === svcYear.value)
)

const svcItems = computed(() => {
  const map = new Map<number, Receivable>()
  for (const r of svcReceivables.value) {
    map.set(r.partyId, r)
  }
  return Array.from(map.values())
})

const svcStats = computed(() => {
  const items = svcReceivables.value
  const total = items.reduce((s, r) => s + r.amountCents, 0)
  const paid = items.reduce((s, r) => s + r.paidCents, 0)
  const unpaid = items.reduce((s, r) => s + r.outstandingCents, 0)
  return { total, paid, unpaid }
})

const svcExpenses = computed(() =>
  allTransactions.value.filter(t => t.categoryId === 85 && t.direction === 'expense' && t.txnDate?.startsWith(String(svcYear.value)))
)

const svcRemaining = computed(() => {
  return svcStats.value.paid - svcExpenses.value.reduce((s, t) => s + t.amountCents, 0)
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
    const [recvR, txnR] = await Promise.all([
      api.get<{ data: { items: Receivable[] } } | { items: Receivable[] }>('/receivables'),
      api.get<{ data: { items: Transaction[] } } | { items: Transaction[] }>('/transactions'),
    ])
    const recvItems = (recvR as any)?.data?.items || (recvR as any)?.items || []
    const txnItems = (txnR as any)?.data?.items || (txnR as any)?.items || []
    allReceivables.value = Array.isArray(recvItems) ? recvItems : []
    allTransactions.value = Array.isArray(txnItems) ? txnItems : []
  } catch (e) {
    console.error('load error', e)
  }
}

onMounted(load)
</script>

<style scoped>
.page-header { margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; }
.page-title { font-size: 18px; font-weight: 600; margin: 0; }
.page-sub   { font-size: 12px; color: #969799; margin: 4px 0 0; }

.fp-year-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; }
.fp-year-label { font-size: 13px; color: #646566; white-space: nowrap; }
.fp-year-select {
  padding: 6px 10px; border: 1px solid #dcdee0; border-radius: 6px; font-size: 14px;
  background: #fff; outline: none; min-width: 120px;
}
.fp-year-select:focus { border-color: #1989fa; }

.fp-stats {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-bottom: 14px;
}
.fp-stat {
  background: #fff; border: 1px solid var(--line-soft, #eaeaea); border-radius: 8px; padding: 10px 12px;
}
.fp-stat-label { font-size: 12px; color: #969799; }
.fp-stat-value { font-size: 16px; font-weight: 600; color: var(--ink-900, #1f2329); margin-top: 2px; font-variant-numeric: tabular-nums; }

.fp-section { margin-bottom: 20px; }
.fp-section-title {
  font-size: 14px; font-weight: 600; color: #1f2329;
  margin-bottom: 8px; padding: 0 2px;
  display: flex; align-items: center; gap: 8px;
}
.fp-section-title.clickable { cursor: pointer; user-select: none; }
.fp-section-title.clickable:hover { color: #1989fa; }
.fp-collapse-icon { font-size: 10px; color: #969799; }

.fp-table-wrap { overflow-x: auto; border: 1px solid var(--line-soft, #eaeaea); border-radius: 8px; background: #fff; }
.fp-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.fp-table th {
  position: sticky; top: 0; z-index: 1;
  background: #f7f8fa; padding: 10px 12px; text-align: left; font-weight: 600; color: #646566;
  border-bottom: 1px solid #ebedf0; white-space: nowrap;
}
.fp-table td { padding: 10px 12px; border-bottom: 1px solid #f0f1f2; color: #1f2329; }
.fp-table tr:last-child td { border-bottom: none; }
.fp-table td.num { text-align: right; font-variant-numeric: tabular-nums; }
.fp-table th.num { text-align: right; }
.fp-table .empty-cell { text-align: center; color: #969799; padding: 24px; }
.fp-th-clickable { cursor: pointer; user-select: none; }
.fp-th-clickable:hover { background: #edf0f4; }

.fp-status {
  display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px;
}
.fp-status.open { background: #fff1f0; color: #ee0a24; }
.fp-status.partial { background: #fff7e6; color: #fa8c16; }
.fp-status.paid { background: #e8f8e8; color: #07c160; }

@media (max-width: 600px) {
  .fp-stats { grid-template-columns: repeat(2, 1fr); }
  .fp-table th, .fp-table td { padding: 8px 8px; font-size: 12px; }
}
</style>