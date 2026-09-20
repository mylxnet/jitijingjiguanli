<template>
  <div class="home-page">
    <!-- 顶栏：标题 + 副时间线 + 记一笔 -->
    <div class="page-header">
      <div class="ph-left">
        <h3>看板</h3>
        <span class="today">{{ rangeLabel }}</span>
      </div>
      <van-button size="small" class="record-btn" @click="openRecord">
        <van-icon name="plus" /> 记一笔
      </van-button>
    </div>

    <!-- 银行存款主卡（独占一行，点击打开银行流水明细弹窗） -->
    <div class="bank-card" @click="openBankFlow">
      <div class="bank-label">银行存款</div>
      <div class="bank-value">{{ formatFen(capital?.bankBalanceCents ?? 0) }}</div>
      <div class="bank-more">查看流水 ›</div>
    </div>

    <!-- 环形构成 2×2：资金 / 在外投资 / 欠款 / 可支出 -->
    <div class="donut-grid">
      <div class="donut-card" v-for="(blk, bi) in blocks" :key="blk.title">
        <div class="donut-title">{{ blk.title }}</div>
        <div class="donut-body">
          <svg viewBox="0 0 120 120" class="donut-svg">
            <circle cx="60" cy="60" r="44" fill="none" :stroke="'var(--jade-light)'" stroke-width="22" />
            <circle
              v-for="(seg, si) in blk.segs"
              :key="si"
              cx="60" cy="60" r="44" fill="none"
              :stroke="seg.color" stroke-width="22"
              :stroke-dasharray="donutDash(seg)"
              :stroke-dashoffset="donutOffset(seg)"
              transform="rotate(-90 60 60)"
              stroke-linecap="butt"
            />
          </svg>
          <div class="donut-center">
            <div class="donut-total" :title="formatFen(blk.total)">{{ formatShort(blk.total) }}</div>
            <div class="donut-total-label">合计</div>
          </div>
        </div>
        <div class="donut-legend">
          <div v-for="(seg, si) in blk.segs" :key="si" class="legend-row">
            <span class="legend-dot" :style="{ background: seg.color }"></span>
            <span class="legend-name">{{ seg.name }}</span>
            <span class="legend-val">{{ formatFen(seg.value) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 欠款明细 TOP -->
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
        <span class="owe-amount" :class="{ danger: row.outstanding > 0 }">{{ formatFen(row.outstanding) }}</span>
      </div>
    </div>

    <!-- 无科目引导 -->
    <div v-if="noCategory" class="empty-guide">
      <p>还没有科目，先去创建科目才能记一笔</p>
      <van-button size="small" type="primary" @click="goCategories">去建科目</van-button>
    </div>

    <!-- 银行存款流水弹窗 -->
    <van-popup v-model:show="showBankFlow" :position="popupPos()" round :style="{ maxHeight: '80vh' }">
      <div class="bf-popup">
        <div class="bf-head">
          <span class="bf-title">银行存款流水</span>
          <van-icon name="cross" class="bf-close" @click="showBankFlow = false" />
        </div>
        <van-loading v-if="bankFlowLoading" class="bf-loading" />
        <div v-else-if="bankFlowRows.length === 0" class="bf-empty">暂无流水</div>
        <div v-else class="bf-table-wrap">
          <table class="bf-table">
            <thead>
              <tr>
                <th>日期</th>
                <th>摘要</th>
                <th class="num">收入</th>
                <th class="num">支出</th>
                <th class="num">余额</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in bankFlowRows" :key="r.id">
                <td>{{ r.date }}</td>
                <td class="note" :title="r.note">{{ r.note || '—' }}</td>
                <td class="num income">{{ r.incomeCents ? formatFen(r.incomeCents) : '—' }}</td>
                <td class="num expense">{{ r.expenseCents ? formatFen(r.expenseCents) : '—' }}</td>
                <td class="num balance">{{ formatFen(r.balanceCents) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, inject } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import { popupPos } from '../../composables/useScreen'
import { formatFen, todayStr, recvKindLabel } from '../../types/api'
import type { ApiResponse, Category, ReceivableListResponse, RecvKind, Transaction } from '../../types/api'

const router = useRouter()
const openRecord = inject<() => void>('openRecord', () => {})

interface Cap {
  bankBalanceCents: number
  assetTotalCents: number
  equityTotalCents: number
}
interface Slice { name: string; value: number }
interface Composition {
  fund: Slice[]
  invest: Slice[]
  owe: Slice[]
  expense: Slice[]
}
interface SummaryPayload {
  incomeTotal: number
  expenseTotal: number
  capital: Cap
  composition: Composition
}

const capital = ref<Cap | null>(null)
const summary = ref<SummaryPayload | null>(null)
const oweRows = ref<{ key: string; partyName: string; recvKind: RecvKind; title: string; outstanding: number }[]>([])
const noCategory = ref(false)
const fullCats = ref<Category[]>([])

const rangeLabel = todayStr()

// —— 环形构成调色板（运行时读 theme.css 的 --chart-1..5） ——
const PALETTE = Array.from({ length: 5 }, (_, i) =>
  getComputedStyle(document.documentElement).getPropertyValue(`--chart-${i + 1}`).trim() || '#2B5876'
)

interface Seg { name: string; value: number; color: string; cumulative: number; frac: number; total: number }
interface DonutBlock { title: string; total: number; segs: Seg[] }

function slicesToSegs(title: string, slices?: Slice[]): DonutBlock {
  // 保留全部非零分项（含负数，如实反映余额）；负数在环形图上不画扇区，但计入合计与图例
  const list = (slices || []).filter(s => s.value !== 0)
  const total = list.reduce((sum, s) => sum + s.value, 0)
  // 扇区只由正分项构成，分母必须也用正分项之和：用净值当分母会让 frac > 1，
  // stroke-dasharray 出现负值即被 SVG 判为非法，整环渲染失效。
  const positiveTotal = list.reduce((sum, s) => sum + (s.value > 0 ? s.value : 0), 0)
  let acc = 0
  const segs = list.map((s, i) => {
    const frac = positiveTotal > 0 && s.value > 0 ? s.value / positiveTotal : 0
    const seg: Seg = { name: s.name, value: s.value, color: PALETTE[i % PALETTE.length], cumulative: acc, frac: frac > 0 ? frac : 0, total: positiveTotal }
    if (s.value > 0) acc += s.value
    return seg
  })
  return { title, total, segs }
}

const blocks = computed<DonutBlock[]>(() => {
  const c = summary.value?.composition
  return [
    slicesToSegs('资金构成', c?.fund),
    slicesToSegs('在外投资构成', c?.invest),
    slicesToSegs('欠款构成', c?.owe),
    slicesToSegs('可支出构成', c?.expense),
  ]
})

// SVG 环形表
const R = 44
const CIRC = 2 * Math.PI * R
function donutDash(seg: Seg): string {
  const len = seg.frac * CIRC
  return `${len} ${CIRC - len}`
}
function donutOffset(seg: Seg): string {
  // 逆时针累计角度换算成 dashoffset（防止除零）
  if (!seg.total) return '0'
  const offset = -(seg.cumulative / seg.total) * CIRC
  return `${offset}`
}

// 环形中心简写：1 万以内一律给准确数（原「千」档会把 5,999 四舍五入成 6.0千，与图例对不上）
function formatShort(cents: number): string {
  const yuan = cents / 100
  if (Math.abs(yuan) >= 10000) return (yuan / 10000).toFixed(1) + '万'
  return yuan.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

onMounted(loadAll)

async function loadAll() {
  await Promise.all([loadSummary(), loadOwed(), loadCats()])
}

async function loadSummary() {
  try {
    const y = new Date().getFullYear()
    const from = `${y}-01-01`
    const res = await api.get<ApiResponse<SummaryPayload>>('/summary', { from, to: todayStr() })
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
    fullCats.value = res.data
    const options: { text: string; value: number }[] = []
    const companies: { text: string; value: number }[] = []
    const INVEST_L1_NAMES = ['长期投资', '对外投资']
    for (const l1 of res.data) {
      const isInvest = INVEST_L1_NAMES.includes(l1.name)
      if (l1.children) {
        for (const l2 of l1.children) {
          if (l2.status !== 'active') continue
          if (l2.kind === 'equity') {
            if (isInvest) {
              const invested = (l2.balanceCents ?? 0) < 0 ? -l2.balanceCents! : 0
              companies.push({ text: invested > 0 ? `${l2.name}（已投 ${formatFen(invested)}）` : l2.name, value: l2.id })
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

function goContacts() { router.push('/contacts') }
function goCategories() { router.push('/categories') }

// ---- 银行存款流水弹窗（沿用老看板）----
interface BankFlowRow { id: number; date: string; note: string; incomeCents: number; expenseCents: number; balanceCents: number }
const showBankFlow = ref(false)
const bankFlowLoading = ref(false)
const bankFlowRows = ref<BankFlowRow[]>([])

async function openBankFlow() {
  showBankFlow.value = true
  if (bankFlowRows.value.length > 0) return
  bankFlowLoading.value = true
  try {
    let opening = 0
    try {
      const setRes = await api.get<ApiResponse<{ bankOpeningBalanceCents: number }>>('/settings')
      opening = setRes.data.bankOpeningBalanceCents || 0
    } catch {
      opening = 0
    }
    const res = await api.get<ApiResponse<{ items: Transaction[] }>>('/transactions', { pageSize: 10000 })
    const list = (res.data.items || []).slice()
    list.sort((a, b) => (a.txnDate === b.txnDate ? a.id - b.id : (a.txnDate < b.txnDate ? -1 : 1)))
    let running = opening
    const asc: BankFlowRow[] = list.map(t => {
      const income = t.direction === 'income' ? t.amountCents : 0
      const expense = t.direction === 'expense' ? t.amountCents : 0
      running += income - expense
      return { id: t.id, date: t.txnDate, note: t.note || '', incomeCents: income, expenseCents: expense, balanceCents: running }
    })
    bankFlowRows.value = asc.reverse()
  } catch {
    bankFlowRows.value = []
  } finally {
    bankFlowLoading.value = false
  }
}
</script>

<style scoped>
.home-page {
  padding: 16px;
  padding-bottom: 110px;
  min-height: 100vh;
  background: var(--jade-bg, #f2f7fa);
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.ph-left { display: flex; flex-direction: column; gap: 2px; }
.page-header h3 { font-size: 18px; font-weight: 600; color: var(--jade-deep, #16384d); margin: 0; }
.today { font-size: 12px; color: var(--ink-muted, #7a7770); }

.record-btn {
  color: #fff;
  --van-button-default-color: #fff;
  border-radius: 999px;
  background: var(--jade, #2b5876);
  border: none;
  display: flex;
  align-items: center;
  gap: 4px;
}
.record-btn .van-button__text { color: #fff; }
.record-btn .van-icon { color: #fff; }
.record-btn:active { background: var(--jade-hover, #3a6e8f); }

/* 银行存款主卡（浅色，略深） */
.bank-card {
  background: #d6e4ef;
  color: var(--jade-deep, #16384d);
  border: 1px solid var(--jade-bg, #f2f7fa);
  border-radius: 16px;
  padding: 20px;
  margin-bottom: 16px;
  cursor: pointer;
  position: relative;
  box-shadow: var(--shadow-sm);
  transition: box-shadow .15s;
}
.bank-card:hover { box-shadow: var(--shadow-lg); }
.bank-label { font-size: 13px; opacity: .85; }
.bank-value { font-size: 30px; font-weight: 700; margin-top: 6px; font-variant-numeric: tabular-nums; }
.bank-more { position: absolute; right: 20px; bottom: 20px; font-size: 12px; opacity: .85; }

/* 环形构成 2×2 */
.donut-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 18px;
}
.donut-card {
  background: var(--paper, #faf8f4);
  border-radius: 14px;
  padding: 14px;
  box-shadow: var(--shadow-sm);
}
.donut-title { font-size: 13px; font-weight: 600; color: var(--ink-soft, #3a3936); margin-bottom: 10px; }
.donut-body { position: relative; display: flex; justify-content: center; }
.donut-svg { width: 120px; height: 120px; display: block; }
.donut-center {
  position: absolute; top: 50%; left: 50%;
  transform: translate(-50%, -50%);
  text-align: center; pointer-events: none;
}
.donut-total { font-size: 16px; font-weight: 700; color: var(--ink, #1a1a18); font-variant-numeric: tabular-nums; }
.donut-total-label { font-size: 10px; color: var(--ink-muted, #7a7770); }
.donut-legend { margin-top: 10px; display: flex; flex-direction: column; gap: 5px; }
.legend-row { display: flex; align-items: center; gap: 7px; font-size: 12px; }
.legend-dot { width: 9px; height: 9px; border-radius: 50%; flex: 0 0 auto; }
.legend-name { color: var(--ink-soft, #3a3936); flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.legend-val { color: var(--ink, #1a1a18); font-weight: 600; font-variant-numeric: tabular-nums; }

/* 欠款明细 */
.section-head { display: flex; justify-content: space-between; align-items: center; padding: 4px 0 8px; font-size: 14px; font-weight: 600; color: var(--ink-soft, #3a3936); }
.more { font-size: 12px; color: var(--jade, #2b5876); font-weight: 500; cursor: pointer; }
.owe-list { background: var(--paper, #faf8f4); border-radius: 14px; overflow: hidden; box-shadow: var(--shadow-sm); }
.owe-empty { text-align: center; color: var(--ink-muted, #7a7770); font-size: 13px; padding: 24px 0; }
.owe-row { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; border-bottom: 1px solid var(--line, #e8e3d8); cursor: pointer; }
.owe-row:last-child { border-bottom: none; }
.owe-main { display: flex; align-items: center; gap: 7px; flex-wrap: wrap; min-width: 0; }
.owe-party { font-size: 14px; font-weight: 500; color: var(--ink, #1a1a18); }
.owe-kind { font-size: 11px; border-radius: 99px; padding: 1px 8px; border: 1px solid var(--line, #e8e3d8); color: var(--ink-muted, #7a7770); }
.owe-kind.rent { border-color: var(--terracotta); color: var(--faint-on-warn); background: var(--terracotta-bg); }
.owe-kind.dividend { border-color: var(--indigo, #5a9cb8); color: var(--indigo, #5a9cb8); background: var(--indigo-bg, #f2f8fb); }
.owe-kind.other { border-color: var(--line, #e8e3d8); color: var(--ink-muted, #7a7770); }
.owe-title { font-size: 12px; color: var(--ink-muted, #7a7770); }
.owe-amount { font-size: 14px; font-weight: 600; color: var(--expense, #a33a2d); font-variant-numeric: tabular-nums; white-space: nowrap; }

.empty-guide { text-align: center; background: var(--paper, #faf8f4); border-radius: 14px; padding: 24px; margin-top: 12px; color: var(--ink-muted, #7a7770); box-shadow: var(--shadow-sm); }

/* 银行流水弹窗 */
.bf-popup { padding: 14px 12px 18px; }
.bf-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.bf-title { font-size: 15px; font-weight: 600; color: var(--ink, #1a1a18); }
.bf-close { font-size: 18px; color: var(--ink-muted, #7a7770); cursor: pointer; padding: 2px; }
.bf-loading, .bf-empty { padding: 40px 0; text-align: center; color: var(--ink-muted, #7a7770); font-size: 13px; }
.bf-table-wrap { max-height: 62vh; overflow: auto; border: 1px solid var(--line, #e8e3d8); border-radius: 8px; }
.bf-table { width: 100%; min-width: 560px; border-collapse: collapse; font-size: 12.5px; }
.bf-table th {
  position: sticky; top: 0; z-index: 1;
  background: var(--jade-light, #eaf1f6); color: var(--ink-muted, #7a7770); font-weight: 500; font-size: 11px;
  padding: 8px 8px; text-align: left; border-bottom: 1px solid var(--line, #e8e3d8); white-space: nowrap;
}
.bf-table td { padding: 8px 8px; border-bottom: 1px solid var(--line, #e8e3d8); color: var(--ink, #1a1a18); white-space: nowrap; }
.bf-table tr:last-child td { border-bottom: none; }
.bf-table .num { text-align: right; font-variant-numeric: tabular-nums; }
.bf-table td.income { color: var(--jade, #2b5876); }
.bf-table td.expense { color: var(--expense, #a33a2d); }
.bf-table td.balance { font-weight: 600; }
.bf-table td.note { white-space: normal; min-width: 150px; max-width: 260px; line-height: 1.45; }

@media (min-width: 992px) {
  .donut-grid { grid-template-columns: repeat(4, 1fr); }
  .home-page { max-width: none; }
}
</style>