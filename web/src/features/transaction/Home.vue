<template>
  <div class="home-page">
    <div class="page-header">
      <h3>看板</h3>
      <span class="today">{{ todayStr() }}</span>
    </div>

    <!-- 资产快览 -->
    <div class="cards">
      <div class="dash-card">
        <div class="dash-label">银行存款</div>
        <div class="dash-value">{{ formatFen(capital?.bankBalanceCents ?? 0) }}</div>
      </div>
      <div v-if="(capital?.assetTotalCents ?? 0) !== 0" class="dash-card asset">
        <div class="dash-label">长期投资（在外）</div>
        <div class="dash-value">{{ formatFen(capital?.assetTotalCents ?? 0) }}</div>
      </div>
      <div class="dash-card owe">
        <div class="dash-label">待收欠款</div>
        <div class="dash-value">{{ formatFen(owedTotal) }}</div>
      </div>
      <div class="dash-card equity">
        <div class="dash-label">净资产</div>
        <div class="dash-value">{{ formatFen(capital?.equityTotalCents ?? 0) }}</div>
      </div>
    </div>

    <!-- 本年收益 -->
    <div class="income-strip">
      <span>本年收益（到账）</span>
      <strong>{{ formatFen(summary?.incomeTotal ?? 0) }}</strong>
      <span class="sub">收 {{ formatFen(summary?.incomeTotal ?? 0) }} · 支 {{ formatFen(summary?.expenseTotal ?? 0) }}</span>
    </div>

    <!-- 最新流水 -->
    <div class="section-head">
      <span>最新流水</span>
      <span class="more" @click="goTransactions">查看全部 ›</span>
    </div>
    <div v-if="latestTxn" class="latest-txn" @click="goTransactions">
      <div class="latest-main">
        <span class="latest-date">{{ latestTxn.txnDate }}</span>
        <span class="latest-cat">{{ latestCatName }}</span>
        <span v-if="latestTxn.note" class="latest-note">{{ latestTxn.note }}</span>
      </div>
      <span class="latest-amount" :class="latestTxn.direction">
        {{ latestTxn.direction === 'income' ? '+' : '-' }}{{ formatFen(latestTxn.amountCents) }}
      </span>
    </div>
    <div v-else class="owe-empty">暂无流水</div>

    <!-- 欠款明细 -->
    <div class="section-head">
      <span>欠款明细</span>
      <span class="more" @click="goContacts">查看全部 ›</span>
    </div>
    <div class="owe-list">
      <div v-if="oweRows.length === 0" class="owe-empty">暂无欠款（全部已收）</div>
      <div v-for="row in oweRows" :key="row.key" class="owe-row" @click="goContacts">
        <div class="owe-main">
          <span class="owe-party">{{ row.partyName }}</span>
          <span class="owe-kind" :class="row.recvKind">{{ recvKindLabel[row.recvKind] }}</span>
          <span class="owe-title">{{ row.title }}</span>
        </div>
        <span class="owe-amount">{{ formatFen(row.outstanding) }}</span>
      </div>
    </div>

    <!-- 无科目引导 -->
    <div v-if="noCategory" class="empty-guide">
      <p>还没有科目，先去创建科目才能记一笔</p>
      <van-button size="small" type="primary" @click="goCategories">去建科目</van-button>
    </div>

    <!-- 记一笔（悬浮于底部） -->
    <div class="quick-record" @click="openRecord">
      <van-icon name="plus" class="quick-icon" />
      <span>记一笔</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, inject } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import { formatFen, todayStr, recvKindLabel } from '../../types/api'
import type { Category, Receivable, ReceivableListResponse, RecvKind, Transaction } from '../../types/api'

const router = useRouter()

const openRecord = inject<() => void>('openRecord', () => {})

interface Cap {
  bankBalanceCents: number
  assetTotalCents: number
  equityTotalCents: number
}
interface SummaryPayload {
  incomeTotal: number
  expenseTotal: number
  capital: Cap
}

const capital = ref<Cap | null>(null)
const summary = ref<SummaryPayload | null>(null)
const oweRows = ref<{ key: string; partyName: string; recvKind: RecvKind; title: string; outstanding: number }[]>([])
const noCategory = ref(false)
const latestTxn = ref<Transaction | null>(null)
const latestCatName = ref('')
const owedTotal = computed(() => oweRows.value.reduce((s, r) => s + r.outstanding, 0))
const fullCats = ref<Category[]>([])

onMounted(loadAll)

async function loadAll() {
  await Promise.all([loadSummary(), loadOwed(), loadCats()])
  await loadLatest()
}

async function loadLatest() {
  try {
    const res = await api.get<ApiResponse<{ items: Transaction[]; total: number }>>('/transactions', { pageSize: 1 })
    const t = res.data.items?.[0] || null
    latestTxn.value = t
    latestCatName.value = ''
    if (t) {
      for (const l1 of fullCats.value) {
        if (l1.children) {
          const l2 = l1.children.find(c => c.id === t.categoryId)
          if (l2) {
            latestCatName.value = `${l1.name} / ${l2.name}`
            break
          }
        }
      }
    }
  } catch {
    latestTxn.value = null
    latestCatName.value = ''
  }
}

async function loadSummary() {
  try {
    const y = new Date().getFullYear()
    const from = `${y}-01-01`
    const to = todayStr()
    const res = await api.get<ApiResponse<SummaryPayload>>('/summary', { from, to })
    summary.value = res.data
    capital.value = res.data.capital
  } catch {
    summary.value = null
    capital.value = null
  }
}

async function loadOwed() {
  try {
    const res = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', { status: 'open', pageSize: 200 })
    const map = new Map<string, { key: string; partyName: string; recvKind: RecvKind; title: string; outstanding: number }>()
    for (const it of res.data.items || []) {
      if (it.outstandingCents <= 0) continue
      const key = `${it.partyId}:${it.recvKind}:${it.title}`
      const cur = map.get(key)
      if (cur) cur.outstanding += it.outstandingCents
      else map.set(key, { key, partyName: it.partyName, recvKind: it.recvKind, title: it.title, outstanding: it.outstandingCents })
    }
    oweRows.value = [...map.values()].sort((a, b) => b.outstanding - a.outstanding).slice(0, 8)
  } catch {
    oweRows.value = []
  }
}

async function loadCats() {
  try {
    const res = await api.get<ApiResponse<Category[]>>('/categories')
    const cats = res.data
    fullCats.value = cats
    const options: { text: string; value: number }[] = []
    const companies: { text: string; value: number }[] = []
    const INVEST_L1_NAMES = ['长期投资', '对外投资']
    for (const l1 of cats) {
      const isInvest = INVEST_L1_NAMES.includes(l1.name)
      if (l1.children) {
        for (const l2 of l1.children) {
          if (l2.status !== 'active') continue
          if (l2.kind === 'equity') {
            if (isInvest) {
              const invested = (l2.balanceCents ?? 0) < 0 ? -l2.balanceCents! : 0
              companies.push({
                text: invested > 0 ? `${l2.name}（已投 ${formatFen(invested)}）` : l2.name,
                value: l2.id,
              })
            } else {
              options.push({ text: `${l1.name} / ${l2.name}`, value: l2.id })
            }
          }
        }
      }
    }
    noCategory.value = options.length === 0 && companies.length === 0
  } catch {
    noCategory.value = true
  }
}

function goContacts() {
  router.push('/contacts')
}

function goTransactions() {
  router.push('/transactions')
}

function goCategories() {
  router.push('/categories')
}
</script>

<style scoped>
.home-page {
  padding: 16px;
  padding-bottom: 110px;
  min-height: 100vh;
  background: #f7f7f5;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.page-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: #2c2c2a;
  margin: 0;
}

.today {
  font-size: 12px;
  color: #8f8e88;
}

.cards {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.dash-card {
  flex: 1 1 calc(50% - 8px);
  box-sizing: border-box;
  background: #fff;
  border-radius: 12px;
  padding: 12px;
}

.dash-card.asset {
  background: #fdf3e3;
}

.dash-card.owe {
  background: #fcebeb;
}

.dash-card.equity {
  background: #eaf5ed;
}

.dash-label {
  font-size: 11px;
  color: #8f8e88;
  margin-bottom: 4px;
}

.dash-value {
  font-size: 17px;
  font-weight: 600;
  color: #2c2c2a;
  font-variant-numeric: tabular-nums;
}

.income-strip {
  background: #fff;
  border-radius: 12px;
  padding: 12px 16px;
  margin-bottom: 12px;
  display: flex;
  align-items: baseline;
  gap: 12px;
  font-size: 13px;
  color: #5f5e5a;
  flex-wrap: wrap;
}

.income-strip strong {
  color: #0f6e56;
  font-size: 18px;
}

.income-strip .sub {
  margin-left: auto;
  font-size: 12px;
  color: #8f8e88;
}

.section-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0 8px;
  font-size: 14px;
  font-weight: 500;
  color: #2c2c2a;
}

.more {
  font-size: 12px;
  color: #0f6e56;
}

.owe-list {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}

.latest-txn {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  background: #fff;
  border-radius: 12px;
  padding: 12px 14px;
  margin-bottom: 12px;
  cursor: pointer;
}

.latest-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}

.latest-date {
  font-size: 12px;
  color: #8f8e88;
}

.latest-cat {
  font-size: 14px;
  font-weight: 500;
}

.latest-note {
  font-size: 12px;
  color: #8f8e88;
}

.latest-amount {
  font-size: 16px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.latest-amount.income {
  color: #185fa5;
}

.latest-amount.expense {
  color: #a32d2d;
}

.owe-empty {
  text-align: center;
  color: #8f8e88;
  font-size: 13px;
  padding: 24px 0;
}

.owe-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid #f0f0eb;
  cursor: pointer;
}

.owe-row:last-child {
  border-bottom: none;
}

.owe-main {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.owe-party {
  font-size: 14px;
  font-weight: 500;
}

.owe-kind {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #e3e2dd;
  color: #8f8e88;
}

.owe-kind.rent { border-color: #7a4f0f; color: #7a4f0f; background: #fdf3e3; }
.owe-kind.dividend { border-color: #0f6e56; color: #0f6e56; background: #eaf5ed; }
.owe-kind.other { border-color: #8f8e88; color: #5f5e5a; }

.owe-title {
  font-size: 12px;
  color: #8f8e88;
}

.owe-amount {
  font-size: 14px;
  font-weight: 600;
  color: #a32d2d;
  font-variant-numeric: tabular-nums;
}

.empty-guide {
  text-align: center;
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  margin-top: 12px;
  color: #8f8e88;
}

.quick-record {
  position: fixed;
  left: 50%;
  transform: translateX(-50%);
  bottom: 76px;
  z-index: 50;
  background: #0f6e56;
  color: #fff;
  border-radius: 999px;
  padding: 12px 26px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.18);
  cursor: pointer;
}

.quick-icon {
  font-size: 18px;
}

@media (min-width: 992px) {
  .quick-record {
    bottom: 40px;
  }

  .cards .dash-card {
    flex: 1 1 23%;
  }
}
</style>
