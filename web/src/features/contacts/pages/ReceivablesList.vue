<template>
  <div class="rl-page" :class="'theme-' + theme">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ pageTitle }}</h2>
        <p class="page-sub">共 {{ rows.length }} 笔应收</p>
      </div>
      <div class="rl-header-actions">
        <van-button size="small" @click="exportCSV">导出</van-button>
        <van-button type="primary" plain size="small" @click="openAccrue">年度计提</van-button>
      </div>
    </div>

    <div class="rl-stats">
      <div class="rl-stat"><div class="rl-stat-label">应收合计</div><div class="rl-stat-value">{{ fmt(total) }}</div></div>
      <div class="rl-stat"><div class="rl-stat-label">已收</div><div class="rl-stat-value paid">{{ fmt(paid) }}</div></div>
      <div class="rl-stat"><div class="rl-stat-label">未收</div><div class="rl-stat-value unpaid">{{ fmt(unpaid) }}</div></div>
      <div class="rl-stat"><div class="rl-stat-label">收缴率</div><div class="rl-stat-value">{{ rate }}%</div></div>
    </div>

    <div class="rl-status-tabs">
      <button
        v-for="s in statusOptions" :key="s.value"
        class="rl-status-btn" :class="{ active: activeStatus === s.value }"
        @click="activeStatus = s.value"
      >{{ s.label }}</button>
    </div>

    <van-loading v-if="loading && filtered.length === 0" />
    <div v-else-if="filtered.length === 0" class="empty">
      <van-empty description="暂无应收记录" />
    </div>
    <van-cell-group v-else inset class="rl-list">
      <van-cell
        v-for="r in filtered" :key="r.id"
        :title="partyName(r.partyId)"
        :label="`${kindLabel(r.kind)} · ${r.recvYear || '—'}`"
      >
        <template #right-icon>
          <div class="rl-row-right">
            <div class="rl-amounts">
              <div class="rl-amount">{{ fmt(r.amountCents) }}</div>
              <div class="rl-progress">
                <div class="rl-progress-bar" :style="{ width: progressPct(r) + '%' }"></div>
              </div>
              <div class="rl-amount-sub">已 {{ fmt(r.paidCents) }} · 未 {{ fmt(r.outstandingCents) }}</div>
            </div>
            <van-button
              size="mini" type="primary" :disabled="r.outstandingCents <= 0"
              @click="openCollect(r)"
            >收缴</van-button>
          </div>
        </template>
      </van-cell>
    </van-cell-group>

    <!-- 年度计提预览弹窗 -->
    <van-dialog
      v-model:show="showAccrueDialog"
      title="年度计提预览"
      show-cancel-button
      confirm-button-text="确认计提"
      @confirm="confirmAccrue"
      @cancel="showAccrueDialog = false"
      class="accrue-dialog"
    >
      <div v-if="accrueLoading" class="accrue-loading"><van-loading /></div>
      <div v-else-if="accrueItems.length === 0" class="accrue-empty">暂无待计提数据</div>
      <div v-else class="accrue-body">
        <div class="accrue-year">计提年份：{{ accrueYear }} 年</div>
        <div class="accrue-hint">已存在的条目将自动跳过，可修改金额后确认</div>
        <div class="accrue-list">
          <div v-for="(item, i) in accrueItems" :key="i" class="accrue-item" :class="{ exists: item.exists }">
            <div class="accrue-item-left">
              <div class="accrue-party">{{ item.partyName }}</div>
              <div class="accrue-kind">{{ kindLabel(item.kind) }}</div>
            </div>
            <div class="accrue-item-right">
              <template v-if="item.exists">
                <span class="accrue-exists">已存在</span>
              </template>
              <template v-else>
                <input
                  type="number"
                  class="accrue-input"
                  :value="(item.amountCents / 100).toFixed(2)"
                  @input="onAccrueAmountChange(i, $event)"
                />
                <span class="accrue-unit">元</span>
              </template>
            </div>
          </div>
        </div>
      </div>
    </van-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, defineProps } from 'vue'
import { api } from '../../../lib/http'

// 统一从 API 响应里提取数组（兼容 data / data.items / 直接数组）
function extractList(r: any) {
  if (r == null) return [];
  if (Array.isArray(r)) return r;
  if (Array.isArray(r.data)) return r.data;
  if (r.data && Array.isArray(r.data.items)) return r.data.items;
  return [];
}

interface Receivable {
  id: number; partyId: number; kind: string; recvYear?: number;
  amountCents: number; paidCents: number; outstandingCents: number;
}
interface Party { id: number; name: string }

const props = defineProps<{
  kind: string[]
  theme: 'blue' | 'orange' | 'purple'
  pageTitle: string
}>()

const receivables = ref<Receivable[]>([])
const parties = ref<Party[]>([])
const loading = ref(false)
const activeStatus = ref<'all' | 'open' | 'partial' | 'closed'>('all')

// 年度计提预览
const showAccrueDialog = ref(false)
const accrueLoading = ref(false)
const accrueYear = ref(0)
const accrueItems = ref<{ partyId: number; partyName: string; kind: string; amountCents: number; exists: boolean; _amountCents: number }[]>([])

const statusOptions = [
  { value: 'all',     label: '全部' },
  { value: 'open',   label: '未收' },
  { value: 'partial', label: '部分收' },
  { value: 'closed',  label: '已清' },
] as const

const kindLabel = (k: string) => ({
  dividend: '投资收益', reinvest_dividend: '再投资收益',
  rent: '土地流转费', service: '管理费',
}[k] || k)

const rows = computed(() => receivables.value.filter(r => props.kind.includes(r.kind)))

const filtered = computed(() => rows.value.filter(r => {
  if (activeStatus.value === 'all') return true
  if (activeStatus.value === 'open')   return r.paidCents === 0
  if (activeStatus.value === 'closed') return r.outstandingCents <= 0
  return r.paidCents > 0 && r.outstandingCents > 0
}))

const total = computed(() => rows.value.reduce((s, r) => s + r.amountCents, 0))
const paid  = computed(() => rows.value.reduce((s, r) => s + r.paidCents, 0))
const unpaid = computed(() => rows.value.reduce((s, r) => s + r.outstandingCents, 0))
const rate = computed(() => total.value ? Math.round(paid.value / total.value * 100) : 0)

const partyName = (pid: number) => parties.value.find(p => p.id === pid)?.name || `单位#${pid}`
const progressPct = (r: Receivable) => r.amountCents ? Math.round(r.paidCents / r.amountCents * 100) : 0

const fmt = (c: number) => '¥' + (c / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })

function downloadCSV(filename: string, headers: string[], rows: string[][]) {
  const csv = [headers.join(','), ...rows.map(r => r.map(c => '"' + (c || '').replace(/"/g, '""') + '"').join(','))].join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = filename; a.click()
  URL.revokeObjectURL(url)
}

function exportCSV() {
  const headers = ['单位名称', '应收类型', '年度', '应收金额', '已收金额', '未收金额', '收缴率']
  const data = rows.value.map(r => [
    partyName(r.partyId),
    kindLabel(r.kind),
    String(r.recvYear || ''),
    fmt(r.amountCents),
    fmt(r.paidCents),
    fmt(r.outstandingCents),
    progressPct(r) + '%',
  ])
  downloadCSV(props.pageTitle + '.csv', headers, data)
}

async function load() {
  loading.value = true
  try {
    const [p, r] = await Promise.all([
      api.get<{ data: Party[] } | Party[]>('/parties'),
      api.get<{ data: Receivable[] } | Receivable[]>('/receivables'),
    ])
    parties.value = extractList(p)
    receivables.value = extractList(r)
  } finally { loading.value = false }
}

async function openAccrue() {
  const year = new Date().getFullYear()
  accrueYear.value = year
  showAccrueDialog.value = true
  accrueLoading.value = true
  accrueItems.value = []
  try {
    const r = await api.get<any>('/recv-standards/preview?year=' + year)
    const data = r?.data || r
    accrueItems.value = (data?.items || []).map((item: any) => ({ ...item, _amountCents: item.amountCents }))
  } catch (e: any) {
    alert('获取预览数据失败：' + (e?.message || '未知错误'))
    showAccrueDialog.value = false
  } finally { accrueLoading.value = false }
}

function onAccrueAmountChange(i: number, e: Event) {
  const val = parseFloat((e.target as HTMLInputElement).value)
  if (!isNaN(val) && val >= 0) {
    accrueItems.value[i]._amountCents = Math.round(val * 100)
  }
}

async function confirmAccrue() {
  const items = accrueItems.value
    .filter(item => !item.exists)
    .map(item => ({ partyId: item.partyId, kind: item.kind, amountCents: item._amountCents }))
  if (items.length === 0) { alert('没有需要计提的条目'); return }
  try {
    await api.post('/recv-standards/accrue', { year: accrueYear.value, items })
    alert('计提完成，共 ' + items.length + ' 条')
    showAccrueDialog.value = false
    load()
  } catch (e: any) { alert(e?.message || '计提失败') }
}

function openCollect(r: Receivable) {
  const amt = prompt('本次收缴金额（元）', (r.outstandingCents / 100).toFixed(2))
  if (!amt) return
  const yuan = parseFloat(amt)
  if (isNaN(yuan) || yuan <= 0) return alert('金额不合法')
  api.post('/receipts', { receivableId: r.id, amountCents: Math.round(yuan * 100) })
    .then(() => { alert('收缴成功'); load() })
    .catch((e: any) => alert(e?.message || '收缴失败'))
}

onMounted(load)
</script>

<style scoped>
.page-header { margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; }
.page-title { font-size: 18px; font-weight: 600; margin: 0; }
.page-sub   { font-size: 12px; color: #969799; margin: 4px 0 0; }
.rl-header-actions { display: flex; gap: 8px; align-items: center; }

.rl-stats {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-bottom: 14px;
}
.rl-stat {
  background: #fff; border: 1px solid var(--line-soft, #eaeaea); border-radius: 8px; padding: 10px 12px;
}
.rl-stat-label { font-size: 12px; color: #969799; }
.rl-stat-value { font-size: 16px; font-weight: 600; color: var(--ink-900, #1f2329); margin-top: 2px; font-variant-numeric: tabular-nums; }
.rl-stat-value.paid   { color: #07c160; }
.rl-stat-value.unpaid { color: #ee0a24; }

.rl-status-tabs { display: flex; gap: 6px; margin-bottom: 12px; flex-wrap: wrap; }
.rl-status-btn { padding: 6px 12px; border-radius: 20px; font-size: 13px; border: 1px solid #eaeaea; background: #fff; cursor: pointer; }
.rl-status-btn.active { background: #1989fa; color: #fff; border-color: #1989fa; }

.rl-list { margin-top: 4px; }
.rl-row-right { display: flex; align-items: center; gap: 12px; }
.rl-amounts { text-align: right; min-width: 140px; }
.rl-amount { font-weight: 600; color: #1f2329; font-size: 14px; }
.rl-progress { width: 140px; height: 4px; background: #f0f1f2; border-radius: 3px; margin: 4px 0 2px; overflow: hidden; }
.rl-progress-bar { height: 100%; background: var(--jade, #07c160); transition: width .3s; }
.rl-amount-sub { font-size: 11px; color: #969799; }
.empty { padding: 40px 0; }

/* theme accent for progress bar */
.theme-blue   .rl-progress-bar { background: #1989fa; }
.theme-orange .rl-progress-bar { background: #ff6034; }
.theme-purple .rl-progress-bar { background: #764ba2; }
.theme-blue   .rl-status-btn.active { background: #1989fa; border-color: #1989fa; }
.theme-orange .rl-status-btn.active { background: #ff6034; border-color: #ff6034; }
.theme-purple .rl-status-btn.active { background: #764ba2; border-color: #764ba2; }

/* 年度计提预览弹窗 */
.accrue-dialog { width: 90vw; max-width: 460px; }
.accrue-loading, .accrue-empty { padding: 40px 0; text-align: center; color: #969799; font-size: 13px; }
.accrue-body { padding: 0 16px 16px; }
.accrue-year { font-size: 14px; font-weight: 600; color: #1f2329; margin-bottom: 4px; }
.accrue-hint { font-size: 11px; color: #969799; margin-bottom: 12px; }
.accrue-list { display: flex; flex-direction: column; gap: 6px; max-height: 50vh; overflow-y: auto; }
.accrue-item {
  display: flex; justify-content: space-between; align-items: center;
  padding: 8px 10px; background: #f7f8fa; border-radius: 6px; gap: 8px;
}
.accrue-item.exists { opacity: 0.6; }
.accrue-item-left { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.accrue-party { font-size: 13px; font-weight: 500; color: #1f2329; }
.accrue-kind { font-size: 11px; color: #969799; }
.accrue-item-right { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }
.accrue-input {
  width: 100px; padding: 4px 6px; border: 1px solid #dcdee0; border-radius: 4px;
  font-size: 13px; text-align: right; outline: none; font-variant-numeric: tabular-nums;
}
.accrue-input:focus { border-color: #1989fa; }
.accrue-unit { font-size: 12px; color: #969799; }
.accrue-exists { font-size: 11px; color: #969799; background: #e8e8e8; padding: 2px 8px; border-radius: 4px; }
</style>
