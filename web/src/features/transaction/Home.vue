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
        <div class="record-modes">
          <van-button class="mode-btn" :type="recordMode === 'normal' ? 'primary' : 'default'" @click="recordMode = 'normal'">普通记账</van-button>
          <van-button class="mode-btn" :type="recordMode === 'quick' ? 'primary' : 'default'" @click="recordMode = 'quick'">快速记账</van-button>
        </div>

        <template v-if="recordMode === 'normal'">
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
            class="mobile-field"
          />
          <div class="desktop-field">
            <span class="d-label">科目</span>
            <select v-model="record.categoryId" class="d-select" @change="onCatNativeChange">
              <option :value="0" disabled>请选择科目</option>
              <option v-for="opt in catColumns" :key="opt.value" :value="opt.value">{{ opt.text }}</option>
            </select>
          </div>
          <van-field v-model="record.amount" label="金额" type="number" placeholder="0.00" inputmode="decimal" />
        </template>

        <template v-else>
          <van-field v-model="quick.date" label="日期" placeholder="YYYY-MM-DD" />
          <van-field label="业务">
            <template #input>
              <select v-model="quick.biz" class="qselect" @change="onQuickBizChange">
                <option value="" disabled>选择业务</option>
                <option v-for="b in quickBusinesses" :key="b.key" :value="b.key">{{ b.label }}</option>
              </select>
            </template>
          </van-field>
          <van-field v-if="quickNeedAsset" label="公司（资产科目）">
            <template #input>
              <select v-model="quick.assetCategoryId" class="qselect">
                <option :value="0" disabled>选择投资对象</option>
                <option v-for="a in assetOptions" :key="a.value" :value="a.value">{{ a.text }}</option>
              </select>
            </template>
          </van-field>
          <van-field v-if="quickNeedParty" label="往来单位">
            <template #input>
              <select v-model="quick.partyId" class="qselect">
                <option :value="0" disabled>选择单位</option>
                <option v-for="p in partyOptions" :key="p.value" :value="p.value">{{ p.text }}</option>
              </select>
            </template>
          </van-field>
          <van-field v-model="quick.amount" label="金额" type="number" placeholder="0.00" inputmode="decimal" />
        </template>

        <div v-if="recordError" class="record-error">{{ recordError }}</div>
        <div class="record-save">
          <van-button round block type="primary" :loading="saving" @click="saveRecord">
            {{ recordMode === 'quick' ? '保存（自动入账）' : '保存' }}
          </van-button>
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
import type { Category, Party, Receivable, ReceivableListResponse, RecvKind, ApiResponse } from '../../types/api'

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

// ---- 快速记账（业务模板自动入账） ----
const recordMode = ref<'normal' | 'quick'>('normal')
const fullCats = ref<Category[]>([])
const assetOptions = ref<{ text: string; value: number }[]>([])
const partyOptions = ref<{ text: string; value: number }[]>([])
const quick = ref({ date: todayStr(), biz: '', assetCategoryId: null as number | null, partyId: null as number | null, amount: '' })

interface QuickBiz {
  key: string
  label: string
  dir?: 'income' | 'expense'
  catName?: string
  invest?: boolean
  recover?: boolean
  needParty?: boolean
}
const quickBusinesses: QuickBiz[] = [
  { key: 'grant', label: '收上级财政补助', dir: 'income', catName: '上级补助' },
  { key: 'dividend', label: '收到投资收益/分红', dir: 'income', catName: '投资收益' },
  { key: 'rent', label: '收到土地流转费（单位缴款）', needParty: true, catName: '土地流转费收入' },
  { key: 'service', label: '土地流转服务费收入', dir: 'income', catName: '土地流转服务费收入' },
  { key: 'toHousehold', label: '拨付土地流转费给农户', dir: 'expense', catName: '土地流转费-转付农户' },
  { key: 'interest', label: '银行存款利息', dir: 'income', catName: '其他收入' },
  { key: 'member', label: '532-成员分配发放', dir: 'expense', catName: '成员分红' },
  { key: 'welfare', label: '532-公益支出', dir: 'expense', catName: '公益支出' },
  { key: 'invest', label: '投资给公司', invest: true },
  { key: 'recover', label: '收回投资', recover: true },
]

const quickNeedAsset = computed(() => {
  const b = quickBusinesses.find(x => x.key === quick.value.biz)
  return !!(b && (b.invest || b.recover))
})
const quickNeedParty = computed(() => {
  const b = quickBusinesses.find(x => x.key === quick.value.biz)
  return !!(b && b.needParty)
})

function onQuickBizChange() {
  quick.value.assetCategoryId = null
  quick.value.partyId = null
}

function findEquityCat(name: string): Category | null {
  for (const l1 of fullCats.value) {
    if (l1.children) {
      for (const l2 of l1.children) {
        if (l2.kind === 'equity' && l2.name === name) return l2
      }
    }
  }
  return null
}

async function loadPartiesQuick() {
  try {
    const res = await api.get<ApiResponse<Party[]>>('/parties')
    partyOptions.value = (res.data || []).map(p => ({ text: p.name, value: p.id }))
  } catch {
    partyOptions.value = []
  }
}

async function saveQuick() {
  const amount = Math.round(parseFloat(quick.value.amount || '0') * 100)
  const biz = quickBusinesses.find(x => x.key === quick.value.biz)
  if (!biz) {
    showToast('请选择业务')
    return
  }
  if (amount <= 0) {
    showToast('金额必须大于 0')
    return
  }
  if (quickNeedAsset.value && !quick.value.assetCategoryId) {
    showToast('请选择投资对象（公司）')
    return
  }
  if (quickNeedParty.value && !quick.value.partyId) {
    showToast('请选择往来单位')
    return
  }
  saving.value = true
  recordError.value = ''
  try {
    if (biz.invest || biz.recover) {
      await api.post('/fund-moves', {
        moveDate: quick.value.date,
        kind: biz.invest ? 'invest' : 'recover',
        assetCategoryId: quick.value.assetCategoryId,
        amountCents: amount,
        note: biz.label,
      })
    } else if (biz.key === 'rent' && quick.value.partyId) {
      // 优先冲该单位未收流转费欠款；无欠款则直记收入
      const list = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', {
        partyId: quick.value.partyId,
        kind: 'rent',
        status: 'open',
        pageSize: 50,
      })
      const best = (list.data.items || []).filter(r => r.outstandingCents > 0)
        .sort((a, b) => b.outstandingCents - a.outstandingCents)[0]
      if (best && amount <= best.outstandingCents) {
        await api.post(`/receivables/${best.id}/receipts`, {
          amountCents: amount,
          receiptDate: quick.value.date,
          method: 'cash',
        })
      } else if (best && amount > best.outstandingCents) {
        throw new Error(`该单位待收 ${formatFen(best.outstandingCents)}，不能超过`)
      } else {
        const cat = findEquityCat('土地流转费收入')
        if (!cat) throw new Error('缺少科目：土地流转费收入')
        await api.post('/transactions', {
          txnDate: quick.value.date, direction: 'income', amountCents: amount, categoryId: cat.id, note: biz.label,
        })
      }
    } else {
      const cat = findEquityCat(biz.catName || '')
      if (!cat) throw new Error(`缺少科目：${biz.catName}`)
      await api.post('/transactions', {
        txnDate: quick.value.date,
        direction: biz.dir || 'income',
        amountCents: amount,
        categoryId: cat.id,
        note: biz.label,
      })
    }
    showToast('保存成功（已自动入账）')
    showRecord.value = false
    await loadAll()
  } catch (e: any) {
    recordError.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
}

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
    fullCats.value = cats
    const options: { text: string; value: number }[] = []
    const assets: { text: string; value: number }[] = []
    for (const l1 of cats) {
      if (l1.children) {
        for (const l2 of l1.children) {
          if (l2.status !== 'active') continue
          if (l2.kind === 'equity') {
            options.push({ text: `${l1.name} / ${l2.name}`, value: l2.id })
          } else if (l2.kind === 'asset') {
            assets.push({ text: `${l1.name} / ${l2.name}`, value: l2.id })
          }
        }
      }
    }
    catColumns.value = options
    assetOptions.value = assets
    noCategory.value = options.length === 0 && assets.length === 0
  } catch {
    catColumns.value = []
    assetOptions.value = []
    noCategory.value = true
  }
}

function openRecord() {
  recordMode.value = 'normal'
  record.value = { date: todayStr(), note: '', categoryName: '', categoryId: null, amount: '', direction: 'expense' }
  quick.value = { date: todayStr(), biz: '', assetCategoryId: null, partyId: null, amount: '' }
  recordError.value = ''
  showRecord.value = true
  void loadPartiesQuick()
  void loadCats()
}

function onCatConfirm({ selectedOptions }: any) {
  const opt = selectedOptions[0]
  if (opt) {
    record.value.categoryName = opt.text
    record.value.categoryId = opt.value
  }
  showCatPicker.value = false
}

function onCatNativeChange() {
  const opt = catColumns.value.find(o => o.value === record.value.categoryId)
  record.value.categoryName = opt ? opt.text : ''
}

async function saveRecord() {
  if (recordMode.value === 'quick') {
    await saveQuick()
    return
  }
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

.record-modes {
  display: flex;
  gap: 10px;
  margin: 4px 16px 10px;
}

.mode-btn {
  flex: 1 1 0%;
  min-width: 0;
  height: 44px;
  font-size: 15px;
  border-radius: 8px;
}

.qselect {
  flex: 1;
  width: 100%;
  height: 40px;
  border: 1px solid #dcdee0;
  border-radius: 8px;
  font-size: 15px;
  padding: 0 10px;
  background: #fff;
  color: #323233;
}

.desktop-field {
  display: none;
}

@media (min-width: 992px) {
  .quick-record {
    bottom: 40px;
  }

  .cards .dash-card {
    flex: 1 1 23%;
  }

  .desktop-field {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 16px;
  }

  .mobile-field {
    display: none !important;
  }

  .d-label {
    width: 70px;
    font-size: 15px;
    color: #969799;
  }

  .d-select {
    flex: 1;
    height: 40px;
    border: 1px solid #dcdee0;
    border-radius: 8px;
    font-size: 15px;
    padding: 0 10px;
    background: #fff;
    color: #323233;
  }
}
</style>
