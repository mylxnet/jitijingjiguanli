<template>
  <div class="summary-page">
    <div class="page-header">
      <h3>汇总</h3>
      <div class="month-selector">
        <van-button size="small" plain @click="prevYear">&lt;</van-button>
        <span class="current-month">{{ currentYear }} 年</span>
        <van-button size="small" plain @click="nextYear">&gt;</van-button>
        <van-button size="small" type="primary" plain icon="export" @click="showExportSheet = true">导出</van-button>
      </div>
    </div>

    <van-action-sheet
      v-model:show="showExportSheet"
      :actions="exportActions"
      @select="onExportSelect"
      cancel-text="取消"
      description="收支汇总为所选年份；科目余额表为当前最新余额"
    />

    <!-- 加载中 -->
    <div v-if="loading" class="loading-state">
      <van-skeleton title :row="8" />
    </div>

    <!-- 加载失败 -->
    <div v-else-if="loadError" class="error-state">
      <p>加载失败</p>
      <van-button size="small" @click="loadSummary">重试</van-button>
    </div>

    <template v-else>
      <!-- 资金构成（v0.4：银行 / 资产类 / 净资产；总资产=银行+资产，应收欠款见往来） -->
      <div class="capital-cards">
        <div class="capital-card" :class="{ warning: capital?.warning }">
          <div class="card-label">银行存款</div>
          <div class="card-value">{{ formatFen(capital?.bankBalanceCents ?? 0) }}</div>
        </div>
        <div class="capital-card asset">
          <div class="card-label">资产类</div>
          <div class="card-value">{{ formatFen(capital?.assetTotalCents ?? 0) }}</div>
        </div>
        <div class="capital-card equity">
          <div class="card-label">净资产</div>
          <div class="card-value">{{ formatFen(capital?.equityTotalCents ?? 0) }}</div>
        </div>
        <div class="capital-card total">
          <div class="card-label">总资产</div>
          <div class="card-value">{{ formatFen(totalAssets) }}</div>
        </div>
      </div>

      <div v-if="capital?.warning" class="warning-banner">
        <van-icon name="warning-o" />
        {{ capital.warning }}
      </div>

      <!-- 区间收支小计 -->
      <div class="income-expense-bar">
        <span class="income-label">收 {{ formatFen(summary.incomeTotal) }}</span>
        <span class="expense-label">支 {{ formatFen(summary.expenseTotal) }}</span>
        <span class="balance-label" :class="summary.balance >= 0 ? 'positive' : 'negative'">
          结余 {{ formatFen(summary.balance) }}
        </span>
      </div>

      <!-- 无数据 -->
      <div v-if="categories.length === 0" class="empty-state">
        <p>本月还没有流水</p>
        <van-button size="small" type="primary" @click="goHome">去记一笔</van-button>
      </div>

      <!-- 科目余额 -->
      <div v-else class="category-section">
        <div class="section-title">科目余额</div>
        <div v-for="l1 in categories" :key="l1.id" class="l1-group">
          <div class="l1-row" @click="toggleExpand(l1.id)">
            <div class="l1-info">
              <van-icon :name="expanded[l1.id] ? 'arrow-down' : 'arrow'" />
              <span class="l1-name">{{ l1.name }}</span>
              <span class="l1-chip">分组</span>
            </div>
            <div class="l1-balance">{{ formatFen(l1.currentBalanceCents) }}</div>
          </div>
          <div v-if="expanded[l1.id] && l1.children && l1.children.length > 0" class="l2-list">
            <div v-for="l2 in l1.children" :key="l2.id" class="l2-row" @click="openDrill(l2)">
              <div class="l2-info">
                <span class="l2-name">{{ l2.name }}</span>
                <span class="l2-chip" :class="l2.kind || 'equity'">{{ (l2.kind || 'equity') === 'asset' ? '资产' : '权益' }}</span>
                <span class="l2-count">{{ l2.txnCount }}笔</span>
              </div>
              <div class="l2-balance">
                {{ formatFen(l2.currentBalanceCents) }}
                <van-icon name="arrow" class="l2-arrow" />
              </div>
            </div>
          </div>
          <div v-else-if="expanded[l1.id] && (!l1.children || l1.children.length === 0)" class="l2-empty">
            暂无二级科目
          </div>
        </div>
      </div>
    </template>

    <!-- 科目当月流水弹层（点二级科目） -->
    <van-popup v-model:show="showDrill" :position="popupPos()" round closeable style="max-height: 80vh">
      <div class="drill-popup">
        <div class="drill-header">
          <span class="drill-title">{{ drillCat ? drillCat.name : '' }} · 流水</span>
        </div>
        <div class="month-selector drill-month">
          <van-button size="small" plain @click="drillPrevYear">&lt;</van-button>
          <span class="current-month">{{ drillYear }} 年</span>
          <van-button size="small" plain @click="drillNextYear">&gt;</van-button>
        </div>
        <div v-if="drillLoading" class="drill-empty">加载中…</div>
        <div v-else-if="drillItems.length === 0" class="drill-empty">该科目本年暂无流水</div>
        <div v-else class="drill-list">
          <div v-for="t in drillItems" :key="t.id" class="drill-row" @click="goListWithFilter(t)" :class="{ voided: t.status === 'voided' }">
            <span class="drill-date">{{ t.txnDate }}</span>
            <span class="drill-dir" :class="t.direction">{{ t.direction === 'income' ? '收' : '支' }}</span>
            <span class="drill-amount" :class="t.direction">{{ t.direction === 'income' ? '+' : '-' }}{{ formatFen(t.amountCents) }}</span>
            <span class="drill-note">{{ t.note || '' }}</span>
            <span v-if="t.status === 'voided'" class="drill-voided">已作废</span>
          </div>
        </div>
        <div v-if="drillItems.length > 0" class="drill-total">
          收 {{ formatFen(drillIncome) }} · 支 {{ formatFen(drillExpense) }} · 结余 {{ formatFen(drillIncome - drillExpense) }}
        </div>
      </div>
    </van-popup>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import { downloadExport, type ExportContent, type ExportFormat } from '../../lib/download'
import { formatFen } from '../../types/api'
import { showToast } from 'vant'
import type { ApiResponse, Transaction } from '../../types/api'

import { popupPos } from '../../composables/useScreen';
interface CategorySummary {
  id: number
  name: string
  level: number
  parentId?: number
  kind?: 'equity' | 'asset'
  currentBalanceCents: number
  txnCount: number
  incomeCents: number
  expenseCents: number
  children?: CategorySummary[]
}

interface Capital {
  bankBalanceCents: number
  assetTotalCents: number
  equityTotalCents: number
  warning?: string
}

interface SummaryResponse {
  incomeTotal: number
  expenseTotal: number
  balance: number
  capital: Capital
  categories: CategorySummary[]
}

const router = useRouter()
const loading = ref(true)
const loadError = ref(false)
const currentYear = ref(new Date().getFullYear().toString())
const summary = ref<SummaryResponse>({ incomeTotal: 0, expenseTotal: 0, balance: 0, capital: { bankBalanceCents: 0, assetTotalCents: 0, equityTotalCents: 0 }, categories: [] })
const capital = computed(() => summary.value.capital)
const categories = computed(() => summary.value.categories)
const totalAssets = computed(() => (capital.value?.bankBalanceCents ?? 0) + (capital.value?.assetTotalCents ?? 0))
const expanded = ref<Record<number, boolean>>({})

onMounted(async () => {
  await loadSummary()
})

async function loadSummary() {
  loading.value = true
  loadError.value = false
  try {
    const y = currentYear.value
    const from = `${y}-01-01`
    const to = `${y}-12-31`
    const res = await api.get<ApiResponse<SummaryResponse>>('/summary', { from, to })
    summary.value = res.data
    // 默认展开第一个一级科目
    if (res.data.categories.length > 0) {
      expanded.value[res.data.categories[0].id] = true
    }
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

function toggleExpand(id: number) {
  expanded.value[id] = !expanded.value[id]
}

// ---- 科目当月流水弹层（点二级科目） ----
const showDrill = ref(false)
const drillCat = ref<CategorySummary | null>(null)
const drillYear = ref(new Date().getFullYear().toString())
const drillItems = ref<Transaction[]>([])
const drillLoading = ref(false)

const drillIncome = computed(() =>
  drillItems.value.filter(i => i.status === 'normal' && i.direction === 'income')
    .reduce((s, i) => s + i.amountCents, 0))
const drillExpense = computed(() =>
  drillItems.value.filter(i => i.status === 'normal' && i.direction === 'expense')
    .reduce((s, i) => s + i.amountCents, 0))

async function openDrill(l2: CategorySummary) {
  drillCat.value = l2
  drillYear.value = new Date().getFullYear().toString()
  showDrill.value = true
  await loadDrill()
}

async function loadDrill() {
  if (!drillCat.value) return
  drillLoading.value = true
  drillItems.value = []
  try {
    const y = parseInt(drillYear.value)
    const from = `${y}-01-01`
    const to = `${y}-12-31`
    const res = await api.get<ApiResponse<{ items: Transaction[] }>>('/transactions', {
      from,
      to,
      categoryId: drillCat.value.id,
      pageSize: 1000,
      includeVoided: 'true',
    })
    drillItems.value = res.data.items || []
  } catch {
    drillItems.value = []
  } finally {
    drillLoading.value = false
  }
}

function shiftDrillYear(delta: number) {
  drillYear.value = String(parseInt(drillYear.value) + delta)
  void loadDrill()
}

function drillPrevYear() {
  shiftDrillYear(-1)
}

function drillNextYear() {
  shiftDrillYear(1)
}

// 点某条流水跳转到流水页，并带「该科目 + 该年」筛选
function goListWithFilter(t: Transaction) {
  if (!drillCat.value) return
  const y = drillYear.value
  const from = `${y}-01-01`
  const to = `${y}-12-31`
  void t
  router.push({
    path: '/transactions',
    query: {
      from,
      to,
      categoryId: String(drillCat.value.id),
      categoryName: drillCat.value.name,
    },
  })
}

function prevYear() {
  currentYear.value = String(parseInt(currentYear.value) - 1)
  loadSummary()
}

function nextYear() {
  currentYear.value = String(parseInt(currentYear.value) + 1)
  loadSummary()
}

function goHome() {
  router.push('/')
}

// ---- 导出（后端生成：收支汇总带当前月份区间；科目余额表为最新时点） ----
const showExportSheet = ref(false)
interface ExportAction {
  name: string
  content: ExportContent
  format: ExportFormat
}
const exportActions: ExportAction[] = [
  { name: '收支汇总 · Excel', content: 'summary', format: 'xlsx' },
  { name: '收支汇总 · CSV', content: 'summary', format: 'csv' },
  { name: '科目余额表 · Excel', content: 'balance_sheet', format: 'xlsx' },
  { name: '科目余额表 · CSV', content: 'balance_sheet', format: 'csv' },
]

function onExportSelect(action: ExportAction) {
  showExportSheet.value = false
  void handleExport(action)
}

async function handleExport(action: ExportAction) {
  try {
    const params: Record<string, string> = {}
    if (action.content === 'summary') {
      params.from = `${currentYear.value}-01-01`
      params.to = `${currentYear.value}-12-31`
    }
    await downloadExport(params, action.content, action.format)
    showToast('导出成功')
  } catch (e: any) {
    showToast(e.message || '导出失败')
  }
}
</script>

<style scoped>
.summary-page {
  padding: 16px;
padding-bottom: 60px;
  min-height: 100vh;
  background: var(--paper);
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.page-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: var(--ink);
  margin: 0;
}

.month-selector {
  display: flex;
  align-items: center;
  gap: 8px;
}

.current-month {
  font-size: 14px;
  font-weight: 500;
  color: var(--ink);
  min-width: 80px;
  text-align: center;
}

.loading-state {
  padding: 16px;
  background: #fff;
  border-radius: 12px;
}

.error-state {
  text-align: center;
  padding: 40px 20px;
  background: #fff;
  border-radius: 12px;
  color: var(--expense);
}

.capital-cards {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.capital-card {
  flex: 1;
  background: #e8f0fb;
  border-radius: 12px;
  padding: 12px;
  text-align: center;
}

.capital-card.warning {
  border: 1px solid var(--warn);
}

.capital-card.asset {
  background: #fdf3e3;
}

.capital-card.equity {
  background: var(--jade-light);
}

.capital-card.total {
  background: var(--indigo-light);
}

.capital-card.earmarked {
  background: var(--indigo-light);
}

.capital-card.unallocated {
  background: var(--jade-light);
}

.card-label {
  font-size: 11px;
  color: var(--ink-muted);
  margin-bottom: 4px;
}

.card-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--ink);
  font-variant-numeric: tabular-nums;
}

.warning-banner {
  background: #fef3e8;
  border: 1px solid var(--warn);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
  color: var(--warn);
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.income-expense-bar {
  display: flex;
  gap: 12px;
  justify-content: center;
  padding: 10px;
  background: #fff;
  border-radius: 12px;
  margin-bottom: 12px;
  font-size: 13px;
}

.income-label { color: var(--jade); }
.expense-label { color: var(--expense); }
.balance-label.positive { color: var(--jade); }
.balance-label.negative { color: var(--expense); }

.empty-state {
  text-align: center;
  padding: 40px 20px;
  background: #fff;
  border-radius: 12px;
  color: var(--ink-muted);
}

.category-section {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}

.section-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--ink);
  padding: 12px 16px;
  border-bottom: 1px solid var(--line-soft);
}

.l1-group {
  border-bottom: 1px solid var(--line-soft);
}

.l1-group:last-child {
  border-bottom: none;
}

.l1-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  cursor: pointer;
}

.l1-info {
  display: flex;
  align-items: center;
  gap: 6px;
}

.l1-name {
  font-size: 14px;
  font-weight: 500;
}

.l1-chip {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #e3e2dd;
  color: var(--ink-muted);
}

.l1-chip.residual { border-color: var(--jade); color: var(--jade); }
.l1-chip.spending { border-color: var(--indigo); color: var(--indigo); }

.l1-balance {
  font-size: 14px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  color: var(--ink);
}

.l2-list {
  padding: 0 16px 8px 40px;
}

.l2-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0;
}

.l2-info {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.l2-name {
  font-size: 13px;
  color: var(--ink-soft);
}

.l2-chip {
  font-size: 10px;
  border-radius: 99px;
  padding: 1px 6px;
  border: 1px solid var(--line);
  color: var(--ink-muted);
}

.l2-chip.residual { border-color: var(--jade); color: var(--jade); }
.l2-chip.spending { border-color: var(--indigo); color: var(--indigo); }
.l2-chip.equity { border-color: var(--jade); color: var(--jade); background: var(--jade-light); }
.l2-chip.asset { border-color: var(--asset); color: var(--asset); background: var(--asset-bg); }
.l2-chip.reconcile { border-color: var(--indigo); color: var(--indigo); background: var(--indigo-light); }

.l2-count {
  font-size: 10px;
  color: var(--ink-muted);
}

.l2-balance {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: var(--ink-soft);
}

.l2-empty {
  padding: 8px 0 8px 40px;
  font-size: 12px;
  color: var(--ink-muted);
}

.l2-row {
  cursor: pointer;
}

.l2-arrow {
  color: var(--ink-faint);
  margin-left: 4px;
  vertical-align: middle;
}

/* 科目当月流水弹层 */
.drill-popup {
  padding: 16px 0 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.drill-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px 8px;
}

.drill-title {
  font-size: 16px;
  font-weight: 500;
  color: var(--ink);
}

.drill-month {
  padding: 0 16px;
  margin-bottom: 8px;
}

.drill-empty {
  text-align: center;
  color: var(--ink-muted);
  font-size: 13px;
  padding: 24px 0;
}

.drill-list {
  padding: 0 16px;
}

.drill-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 0;
  border-bottom: 1px solid var(--line-soft);
  cursor: pointer;
}

.drill-row:last-child {
  border-bottom: none;
}

.drill-row.voided {
  opacity: 0.55;
}

.drill-date {
  font-size: 12px;
  color: var(--ink-muted);
}

.drill-dir {
  font-size: 11px;
  border-radius: 4px;
  padding: 1px 5px;
  flex: none;
}

.drill-dir.income { background: var(--jade-light); color: var(--jade); }
.drill-dir.expense { background: var(--danger-bg); color: var(--expense); }

.drill-amount {
  font-size: 13px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  flex: none;
}

.drill-amount.income { color: var(--jade); }
.drill-amount.expense { color: var(--expense); }

.drill-note {
  flex: 1;
  font-size: 12px;
  color: var(--ink-soft);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: right;
}

.drill-voided {
  font-size: 10px;
  color: var(--expense);
  flex: none;
}

.drill-total {
  margin: 12px 16px 0;
  padding-top: 10px;
  border-top: 1px solid var(--line-soft);
  font-size: 12px;
  color: var(--ink-soft);
  display: flex;
  justify-content: space-between;
}
</style>