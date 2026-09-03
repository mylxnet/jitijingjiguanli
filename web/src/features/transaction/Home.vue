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
      <div class="dash-card asset">
        <div class="dash-label">资产类（对外投资）</div>
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

    <!-- 记一笔弹层 -->
    <van-popup v-model:show="showRecord" position="bottom" round closeable style="max-height: 90vh">
      <div class="record-popup">
        <div class="popup-title">记一笔</div>
        <div class="direction-toggle">
          <van-button :type="record.direction === 'income' ? 'primary' : 'default'" size="small" @click="record.direction = 'income'">收入</van-button>
          <van-button :type="record.direction === 'expense' ? 'primary' : 'default'" size="small" @click="record.direction = 'expense'">支出</van-button>
        </div>
        <van-field v-model="record.date" label="日期" placeholder="YYYY-MM-DD" />
        <van-field v-model="record.note" label="摘要" placeholder="买了什么、给了谁（可选）" />
        <van-field
          :model-value="record.categoryName || '请选择科目'"
          is-link readonly label="科目"
          @click="showCatPicker = true"
        />
        <van-field v-model="record.amount" label="金额" type="number" placeholder="0.00" inputmode="decimal" />
        <div v-if="recordError" class="record-error">{{ recordError }}</div>
        <div class="record-save">
          <van-button round block type="primary" :loading="saving" @click="saveRecord">保存</van-button>
        </div>
      </div>
    </van-popup>

    <!-- 科目选择 -->
    <van-popup v-model:show="showCatPicker" position="bottom">
      <van-picker :columns="catColumns" @confirm="onCatConfirm" @cancel="showCatPicker = false" />
    </van-popup>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import { formatFen, todayStr, currentMonthStr, recvKindLabel } from '../../types/api'
import { showToast } from 'vant'
import type { Category, Receivable, ReceivableListResponse, RecvKind, ApiResponse } from '../../types/api'

const router = useRouter()

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
const owedTotal = computed(() => oweRows.value.reduce((s, r) => s + r.outstanding, 0))

const showRecord = ref(false)
const saving = ref(false)
const recordError = ref('')
const record = ref({ date: todayStr(), note: '', categoryName: '', categoryId: null as number | null, amount: '', direction: 'expense' as 'income' | 'expense' })
const showCatPicker = ref(false)
const catColumns = ref<{ text: string; value: number }[]>([])

onMounted(loadAll)

async function loadAll() {
  await Promise.all([loadSummary(), loadOwed(), loadCats()])
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
    const options: { text: string; value: number }[] = []
    for (const l1 of cats) {
      if (l1.children) {
        for (const l2 of l1.children) {
          if (l2.status === 'active' && l2.kind === 'equity') {
            options.push({ text: `${l1.name} / ${l2.name}`, value: l2.id })
          }
        }
      }
    }
    catColumns.value = options
    noCategory.value = options.length === 0
  } catch {
    catColumns.value = []
    noCategory.value = true
  }
}

function openRecord() {
  if (noCategory.value) {
    showToast('还没有科目，请先创建')
    return
  }
  record.value = { date: todayStr(), note: '', categoryName: '', categoryId: null, amount: '', direction: 'expense' }
  recordError.value = ''
  showRecord.value = true
}

function onCatConfirm({ selectedOptions }: any) {
  const opt = selectedOptions[0]
  if (opt) {
    record.value.categoryName = opt.text
    record.value.categoryId = opt.value
  }
  showCatPicker.value = false
}

async function saveRecord() {
  const amount = Math.round(parseFloat(record.value.amount || '0') * 100)
  if (!record.value.categoryId) {
    showToast('请选择科目')
    return
  }
  if (amount <= 0) {
    showToast('金额必须大于 0')
    return
  }
  saving.value = true
  recordError.value = ''
  try {
    await api.post('/transactions', {
      txnDate: record.value.date,
      direction: record.value.direction,
      amountCents: amount,
      categoryId: record.value.categoryId,
      note: record.value.note || '',
    })
    showToast('保存成功')
    showRecord.value = false
    await loadAll()
  } catch (e: any) {
    recordError.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
}

function goContacts() {
  router.push('/contacts')
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

.record-popup {
  padding: 16px 0 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.popup-title {
  font-size: 16px;
  font-weight: 500;
  padding: 0 16px 12px;
}

.direction-toggle {
  display: flex;
  gap: 8px;
  padding: 0 16px 8px;
}

.record-error {
  color: #a32d2d;
  font-size: 13px;
  padding: 0 16px 8px;
}

.record-save {
  margin: 8px 16px 0;
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
