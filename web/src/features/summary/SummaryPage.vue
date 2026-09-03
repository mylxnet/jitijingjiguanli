<template>
  <div class="summary-page">
    <div class="page-header">
      <h3>汇总</h3>
      <div class="month-selector">
        <van-button size="small" plain @click="prevMonth">&lt;</van-button>
        <span class="current-month">{{ currentMonth }}</span>
        <van-button size="small" plain @click="nextMonth">&gt;</van-button>
      </div>
    </div>

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
      <!-- 资金构成三卡 -->
      <div class="capital-cards">
        <div class="capital-card" :class="{ warning: capital?.warning }">
          <div class="card-label">银行存款</div>
          <div class="card-value">{{ formatFen(capital?.bankBalanceCents ?? 0) }}</div>
        </div>
        <div class="capital-card earmarked">
          <div class="card-label">专项资金</div>
          <div class="card-value">{{ formatFen(capital?.earmarkedCents ?? 0) }}</div>
        </div>
        <div class="capital-card unallocated">
          <div class="card-label">未分配资金</div>
          <div class="card-value">{{ formatFen(capital?.unallocatedCents ?? 0) }}</div>
        </div>
      </div>

      <!-- 未分配为负警告 -->
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
              <span class="l1-chip" :class="l1.balanceType">{{ l1.balanceType === 'residual' ? '余粮型' : '花费型' }}</span>
            </div>
            <div class="l1-balance">{{ formatFen(l1.currentBalanceCents) }}</div>
          </div>
          <div v-if="expanded[l1.id] && l1.children && l1.children.length > 0" class="l2-list">
            <div v-for="l2 in l1.children" :key="l2.id" class="l2-row">
              <div class="l2-info">
                <span class="l2-name">{{ l2.name }}</span>
                <span class="l2-chip" :class="l2.balanceType">{{ l2.balanceType === 'residual' ? '余粮' : '花费' }}</span>
                <span v-if="l2.includeInReconciliation" class="l2-chip reconcile">勾稽</span>
                <span class="l2-count">{{ l2.txnCount }}笔</span>
              </div>
              <div class="l2-balance">{{ formatFen(l2.currentBalanceCents) }}</div>
            </div>
          </div>
          <div v-else-if="expanded[l1.id] && (!l1.children || l1.children.length === 0)" class="l2-empty">
            暂无二级科目
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import { formatFen, currentMonthStr, getMonthRange } from '../../types/api'
import type { ApiResponse } from '../../types/api'

interface CategorySummary {
  id: number
  name: string
  level: number
  parentId?: number
  balanceType: 'residual' | 'spending'
  openingBalanceCents: number
  includeInReconciliation: boolean
  currentBalanceCents: number
  txnCount: number
  incomeCents: number
  expenseCents: number
  children?: CategorySummary[]
}

interface Capital {
  bankBalanceCents: number
  earmarkedCents: number
  unallocatedCents: number
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
const currentMonth = ref(currentMonthStr())
const summary = ref<SummaryResponse>({ incomeTotal: 0, expenseTotal: 0, balance: 0, capital: { bankBalanceCents: 0, earmarkedCents: 0, unallocatedCents: 0 }, categories: [] })
const capital = computed(() => summary.value.capital)
const categories = computed(() => summary.value.categories)
const expanded = ref<Record<number, boolean>>({})

onMounted(async () => {
  await loadSummary()
})

async function loadSummary() {
  loading.value = true
  loadError.value = false
  try {
    const range = getMonthRange(currentMonth.value)
    const res = await api.get<ApiResponse<SummaryResponse>>('/summary', { from: range.from, to: range.to })
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

function prevMonth() {
  const [y, m] = currentMonth.value.split('-').map(Number)
  const d = new Date(y, m - 2, 1)
  currentMonth.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  loadSummary()
}

function nextMonth() {
  const [y, m] = currentMonth.value.split('-').map(Number)
  const d = new Date(y, m, 1)
  currentMonth.value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  loadSummary()
}

function goHome() {
  router.push('/')
}
</script>

<style scoped>
.summary-page {
  padding: 16px;
  padding-bottom: 60px;
  min-height: 100vh;
  background: #f7f7f5;
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
  color: #2c2c2a;
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
  color: #2c2c2a;
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
  color: #a32d2d;
}

.capital-cards {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.capital-card {
  flex: 1;
  background: #fff;
  border-radius: 12px;
  padding: 12px;
  text-align: center;
}

.capital-card.warning {
  border: 1px solid #e88a3a;
}

.capital-card.earmarked {
  background: #e6f1fb;
}

.capital-card.unallocated {
  background: #eaf5ed;
}

.card-label {
  font-size: 11px;
  color: #8f8e88;
  margin-bottom: 4px;
}

.card-value {
  font-size: 16px;
  font-weight: 600;
  color: #2c2c2a;
  font-variant-numeric: tabular-nums;
}

.warning-banner {
  background: #fef3e8;
  border: 1px solid #e88a3a;
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
  color: #a8601a;
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

.income-label { color: #0f6e56; }
.expense-label { color: #a32d2d; }
.balance-label.positive { color: #0f6e56; }
.balance-label.negative { color: #a32d2d; }

.empty-state {
  text-align: center;
  padding: 40px 20px;
  background: #fff;
  border-radius: 12px;
  color: #8f8e88;
}

.category-section {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}

.section-title {
  font-size: 14px;
  font-weight: 500;
  color: #2c2c2a;
  padding: 12px 16px;
  border-bottom: 1px solid #f0f0eb;
}

.l1-group {
  border-bottom: 1px solid #f0f0eb;
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
  color: #8f8e88;
}

.l1-chip.residual { border-color: #0f6e56; color: #0f6e56; }
.l1-chip.spending { border-color: #185fa5; color: #185fa5; }

.l1-balance {
  font-size: 14px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  color: #2c2c2a;
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
  color: #5f5e5a;
}

.l2-chip {
  font-size: 10px;
  border-radius: 99px;
  padding: 1px 6px;
  border: 1px solid #e3e2dd;
  color: #8f8e88;
}

.l2-chip.residual { border-color: #0f6e56; color: #0f6e56; }
.l2-chip.spending { border-color: #185fa5; color: #185fa5; }
.l2-chip.reconcile { border-color: #185fa5; color: #185fa5; background: #e6f1fb; }

.l2-count {
  font-size: 10px;
  color: #8f8e88;
}

.l2-balance {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: #5f5e5a;
}

.l2-empty {
  padding: 8px 0 8px 40px;
  font-size: 12px;
  color: #8f8e88;
}
</style>