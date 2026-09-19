<template>
  <div class="rl-page" :class="'theme-' + theme">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ pageTitle }}</h2>
        <p class="page-sub">共 {{ rows.length }} 笔应收</p>
      </div>
      <div class="rl-header-actions">
        <van-button type="primary" size="small" icon="plus" @click="openAddReceivable">登记欠款</van-button>
        <van-button size="small" @click="exportCSV">导出</van-button>
      </div>
    </div>

    <!-- 登记欠款（选单位 + 历史年度，金额默认该单位应收标准可修改） -->
    <van-dialog
      v-model:show="showRecvDialog"
      :title="pageTitle + ' - 登记欠款'"
      class="recv-dialog"
      :show-confirm-button="false"
      :show-cancel-button="false"
    >
      <div class="recv-form">
        <van-field label="单位">
          <template #input>
            <select v-model.number="recvForm.partyId" class="recv-party-select">
              <option :value="0" disabled>请选择单位</option>
              <option v-for="p in recvPartyOptions" :key="p.id" :value="p.id">
                {{ p.name }}{{ p.type === 'reinvest' ? '（再投资）' : '' }}
              </option>
            </select>
          </template>
        </van-field>
        <van-field label="年度">
          <template #input>
            <select v-model="recvForm.year" class="recv-year-select">
              <option v-for="y in recvYearOptions" :key="y" :value="y">{{ y }} 年</option>
            </select>
          </template>
        </van-field>
        <van-field v-model="recvForm.title" label="事由" placeholder="留空自动生成，如：2024年度土地流转费" />
        <van-field v-model="recvForm.amount" label="金额" type="number" placeholder="默认该单位应收标准，可修改" inputmode="decimal" />
        <van-field v-model="recvForm.note" label="备注" placeholder="可选" />
        <div class="recv-form-actions">
          <van-button size="small" @click="showRecvDialog = false">取消</van-button>
          <van-button type="primary" size="small" :loading="savingRecv" @click="saveReceivable">保存</van-button>
        </div>
      </div>
    </van-dialog>

    <div class="rl-stats">
      <div class="rl-stat"><div class="rl-stat-label">应收合计</div><div class="rl-stat-value">{{ fmt(total) }}</div></div>
      <div class="rl-stat"><div class="rl-stat-label">已收</div><div class="rl-stat-value paid">{{ fmt(paid) }}</div></div>
      <div class="rl-stat clickable" @click="openBadDebts">
        <div class="rl-stat-label">坏账</div>
        <div class="rl-stat-value writeoff">{{ fmt(writeoff) }}</div>
        <div class="rl-stat-hint">{{ badDebtCount }} 笔 · 点击查看 ›</div>
      </div>
      <div class="rl-stat"><div class="rl-stat-label">未收</div><div class="rl-stat-value unpaid">{{ fmt(unpaid) }}</div></div>
      <div class="rl-stat"><div class="rl-stat-label">收缴率</div><div class="rl-stat-value">{{ rate }}%</div></div>
    </div>

    <div class="rl-filter-bar">
      <label class="rl-year-select">
        <span class="rl-year-label">年度</span>
        <select class="rl-year-native" :value="String(activeYear)" @change="onYearChange($event)">
          <option v-for="o in yearSelectOptions" :key="String(o.value)" :value="String(o.value)">{{ o.text }}</option>
        </select>
      </label>
    </div>

    <van-loading v-if="loading && rows.length === 0" />
    <div v-else-if="rows.length === 0" class="empty">
      <van-empty description="暂无应收记录" />
    </div>
    <van-cell-group v-else inset class="rl-list">
      <van-cell
        v-for="r in rows" :key="r.id"
        :label="`${kindLabel(r.kind)} · ${r.recvYear || '—'}`"
      >
        <template #title>
          <div class="rl-title">
            <span>{{ partyName(r.partyId) }}</span>
            <van-tag round :type="statusMeta[statusOf(r)].type" class="rl-badge">{{ statusMeta[statusOf(r)].text }}</van-tag>
          </div>
        </template>
        <template #right-icon>
          <div class="rl-row-right">
            <div class="rl-amounts">
              <div class="rl-amount">{{ fmt(r.amountCents) }}</div>
              <div class="rl-progress">
                <div class="rl-progress-bar" :class="'st-' + statusOf(r)" :style="{ width: progressPct(r) + '%' }"></div>
              </div>
              <div class="rl-amount-sub">已 {{ fmt(cashPaid(r)) }} · 坏账 {{ fmt(r.writeoffCents || 0) }} · 未 {{ fmt(r.outstandingCents) }}</div>
            </div>
            <van-button
              size="mini" type="primary" :disabled="r.outstandingCents <= 0"
              @click="openCollect(r)"
            >收缴</van-button>
          </div>
        </template>
      </van-cell>
    </van-cell-group>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, defineProps, inject } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showDialog } from 'vant'
import { api } from '../../../lib/http'

// 统一从 API 响应里提取数组（兼容 data / data.items / data.Items / 直接数组）
function extractList(r: any) {
  if (r == null) return [];
  if (Array.isArray(r)) return r;
  if (Array.isArray(r.data)) return r.data;
  if (r.data && Array.isArray(r.data.Items)) return r.data.Items;
  if (r.data && Array.isArray(r.data.items)) return r.data.items;
  return [];
}

interface Receivable {
  id: number; partyId: number; kind: string; recvYear?: number;
  amountCents: number; paidCents: number; writeoffCents: number; outstandingCents: number;
}
interface Party {
  id: number
  name: string
  type?: string
  expectedLandFeeCents?: number
  expectedMgmtFeeCents?: number
  expectedReturnCents?: number
}

const props = defineProps<{
  kind: string[]
  theme: 'blue' | 'orange' | 'purple'
  pageTitle: string
}>()

const receivables = ref<Receivable[]>([])
const parties = ref<Party[]>([])
const loading = ref(false)

type YearKey = 'all' | 'none' | number
const activeYear = ref<YearKey>('all')

// 状态判定：按已收/未收对照应收金额；坏账结清单独标识
const statusOf = (r: Receivable) => r.outstandingCents <= 0 ? ((r.writeoffCents || 0) > 0 ? 'writeoff' : 'closed') : (r.paidCents > 0 ? 'partial' : 'open')
const statusMeta: Record<string, { text: string; type: string }> = {
  open:     { text: '未收',   type: 'danger' },
  partial:  { text: '部分收', type: 'warning' },
  closed:   { text: '结清',   type: 'success' },
  writeoff: { text: '坏账',   type: 'default' },
}

const kindLabel = (k: string) => ({
  dividend: '投资收益', reinvest_dividend: '再投资收益',
  rent: '土地流转费', service: '管理费',
}[k] || k)

// 本页类别下的全部应收（所有年份，含坏账单）
const kindAll = computed(() => receivables.value.filter(r => props.kind.includes(r.kind)))
// 已收（现金口径）= 全部核销 − 坏账；坏账不是真实收款
const cashPaid = (r: Receivable) => Math.max(0, (r.paidCents || 0) - (r.writeoffCents || 0))
// 正常页面可见：非坏账单，或坏账单但「实际已收 > 0」；全额坏账（一分没收）完全剔除，只在坏账清单页归档
const visible = (r: Receivable) => (r.writeoffCents || 0) === 0 || cashPaid(r) > 0
// 年度匹配规则（列表展示与合计口径共用）
function yearMatch(r: Receivable) {
  if (activeYear.value === 'all') return true
  if (activeYear.value === 'none') return !r.recvYear || r.recvYear <= 0
  return r.recvYear === activeYear.value
}

// 年度选项：全部 + 各年份（倒序）+ 无年度（若有未标注年份的记录）
const yearOptions = computed<{ key: YearKey; label: string }[]>(() => {
  const map = new Map<string, { key: YearKey; label: string }>()
  map.set('all', { key: 'all', label: '全部年度' })
  kindAll.value.filter(visible).forEach(r => {
    const y = r.recvYear
    if (!y || y <= 0) map.set('none', { key: 'none', label: '无年度' })
    else if (!map.has(String(y))) map.set(String(y), { key: y, label: y + '年' })
  })
  return [...map.values()].sort((a, b) => {
    if (a.key === 'all') return -1
    if (b.key === 'all') return 1
    if (a.key === 'none') return 1
    if (b.key === 'none') return -1
    return (b.key as number) - (a.key as number)
  })
})

// 年份下拉选项（Vant dropdown-item 用 { text, value }）
const yearSelectOptions = computed(() => yearOptions.value.map(o => ({ text: o.label, value: o.key })))

// 原生下拉选择年度（桌面端）：value 回填为匹配的类型
function onYearChange(e: Event) {
  const v = (e.target as HTMLSelectElement).value
  activeYear.value = (v === 'all' || v === 'none') ? v as 'all' | 'none' : Number(v)
}

// 页面显示集：明细与合计共用同一集合（保证「合计 = 明细之和」）
const rows = computed(() => kindAll.value.filter(r => yearMatch(r) && visible(r)))

const total = computed(() => rows.value.reduce((s, r) => s + r.amountCents, 0))
const paid  = computed(() => rows.value.reduce((s, r) => s + cashPaid(r), 0))
const writeoff = computed(() => rows.value.reduce((s, r) => s + (r.writeoffCents || 0), 0))
const badDebtCount = computed(() => rows.value.filter(r => (r.writeoffCents || 0) > 0).length)
const router = useRouter()
const openBadDebts = () => router.push('/contacts/receivables/bad-debt')
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
  const headers = ['单位名称', '应收类型', '年度', '应收金额', '已收金额', '坏账金额', '未收金额', '收缴率']
  // 导出与页面显示集一致
  const data = rows.value.map(r => [
    partyName(r.partyId),
    kindLabel(r.kind),
    String(r.recvYear || ''),
    fmt(r.amountCents),
    fmt(cashPaid(r)),
    fmt(r.writeoffCents || 0),
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
    // 后端字段为 recvKind，前端统一归一化为 kind 使用
    receivables.value = (extractList(r) || []).map((it: any) => ({ ...it, kind: it.recvKind }))
  } finally { loading.value = false }
}

// ---- 登记欠款（按本页应收类别锁定可选单位类型与 recvKind） ----
const kindTitleOf = (k: string) => ({
  rent: '土地流转费', service: '流转管理费',
  dividend: '投资收益', reinvest_dividend: '再投资收益',
}[k] || k)

// 本页 kinds 中，与某单位类型匹配时对应登记的 recvKind（null=该单位不适用本页）
function recvKindOfParty(type?: string): string | null {
  const t = type || ''
  const ks = props.kind
  if (ks.includes('rent') && t === 'flow') return 'rent'
  if (ks.includes('service') && t === 'flow') return 'service'
  if (ks.includes('dividend') && t === 'invest') return 'dividend'
  if (ks.includes('reinvest_dividend') && t === 'reinvest') return 'reinvest_dividend'
  return null
}

const showRecvDialog = ref(false)
const savingRecv = ref(false)
const curYear = new Date().getFullYear()
// 年度下拉：不含本年度，往前 5 年（2026 → 2025…2021），用于补录历年欠款
const recvYearOptions = computed(() => {
  const arr: number[] = []
  for (let y = curYear - 1; y >= curYear - 5; y--) arr.push(y)
  return arr
})
const recvForm = ref({
  partyId: 0,
  year: curYear - 1,
  title: '',
  amount: '',
  note: '',
})
const recvPartyOptions = computed(() =>
  parties.value.filter(p => recvKindOfParty(p.type) !== null)
)

// 选中单位后，金额默认带出该单位本类应收标准（元），仍可修改
watch(() => recvForm.value.partyId, (id: number) => {
  if (!id) return
  const p = parties.value.find(x => x.id === id)
  if (!p) return
  const kind = recvKindOfParty(p.type)
  let cents = 0
  if (kind === 'rent') cents = p.expectedLandFeeCents || 0
  else if (kind === 'service') cents = p.expectedMgmtFeeCents || 0
  else if (kind === 'dividend' || kind === 'reinvest_dividend') cents = p.expectedReturnCents || 0
  recvForm.value.amount = cents > 0 ? (cents / 100).toFixed(2) : ''
})

function openAddReceivable() {
  recvForm.value = {
    partyId: 0,
    year: curYear - 1,
    title: '',
    amount: '',
    note: '',
  }
  showRecvDialog.value = true
}

async function saveReceivable() {
  const party = recvPartyOptions.value.find(x => x.id === recvForm.value.partyId)
  if (!party) {
    showToast('请选择单位')
    return
  }
  const recvKind = recvKindOfParty(party.type)
  if (!recvKind) {
    showToast('该单位类型不适用于本页，请重新选择')
    return
  }
  const amountCents = Math.round((parseFloat(recvForm.value.amount) || 0) * 100)
  if (amountCents <= 0) {
    showToast('请填写正确的金额')
    return
  }
  const year = Number(recvForm.value.year)
  if (!year || year <= 0) {
    showToast('请选择年度')
    return
  }
  let title = recvForm.value.title.trim()
  if (!title) title = `${year}年度${kindTitleOf(recvKind)}`
  // 同单位同类别同年度防重（与后端一致：流转费/管理费按年只允许一张单）
  if (recvKind === 'rent' || recvKind === 'service') {
    const dup = kindAll.value.some(r =>
      r.kind === recvKind && r.partyId === party.id && (r.recvYear || 0) === year
    )
    if (dup) {
      showDialog({
        title: '重复登记',
        message: `${party.name} ${year} 年度已登记过${kindTitleOf(recvKind)}欠款，请勿重复登记；如需调整金额，请先作废原单再重新登记。`,
      })
      return
    }
  }
  savingRecv.value = true
  try {
    const payload: Record<string, unknown> = {
      partyId: party.id, recvKind, recvYear: year, title, amountCents,
    }
    if (recvForm.value.note.trim()) payload.note = recvForm.value.note.trim()
    await api.post('/receivables', payload)
    showToast('登记成功')
    showRecvDialog.value = false
    await load()
  } catch (e: any) {
    showDialog({ title: '登记失败', message: e.message || '登记失败，请重试' })
  } finally {
    savingRecv.value = false
  }
}

async function openCollect(r: Receivable) {
  // 统一走快速记账通道：预填对应收款业务与该单位，并指定本单做单张收缴
  const bizKey: Record<string, string> = {
    rent: 'rent', dividend: 'dividend', reinvest_dividend: 'dividend', service: 'service',
  }
  openRecord({
    biz: bizKey[r.kind] || '',
    partyId: r.partyId,
    recvId: r.id,
    amount: (r.outstandingCents / 100).toFixed(2),
  })
}

const openRecord = inject<(opts?: { biz?: string; partyId?: number; recvId?: number; amount?: string }) => void>('openRecord', () => {})

onMounted(load)
</script>

<style scoped>
.page-header { margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; }
.page-title { font-size: 18px; font-weight: 600; margin: 0; }
.page-sub   { font-size: 12px; color: #969799; margin: 4px 0 0; }
.rl-header-actions { display: flex; gap: 8px; align-items: center; }
.recv-dialog { width: 90vw; max-width: 460px; }
.recv-form { padding: 4px 0 0; }
.recv-party-select,
.recv-year-select {
  width: 100%; height: 28px; border: 1px solid #dcdee0; border-radius: 6px;
  font-size: 14px; color: #1f2329; background: #fff; outline: none; padding: 0 4px;
}
.recv-form-actions { display: flex; justify-content: flex-end; gap: 10px; padding: 8px 16px 16px; }

.rl-stats {
  display: grid; grid-template-columns: repeat(5, 1fr); gap: 10px; margin-bottom: 14px;
}
.rl-stat {
  background: #fff; border: 1px solid var(--line-soft, #eaeaea); border-radius: 8px; padding: 10px 12px;
}
.rl-stat-label { font-size: 12px; color: #969799; }
.rl-stat-value { font-size: 16px; font-weight: 600; color: var(--ink-900, #1f2329); margin-top: 2px; font-variant-numeric: tabular-nums; }
.rl-stat-value.paid     { color: var(--success); }
.rl-stat-value.unpaid   { color: var(--danger); }
.rl-stat-value.writeoff { color: var(--warn); }
.rl-stat.clickable { cursor: pointer; }
.rl-stat.clickable:hover { filter: brightness(0.97); }
.rl-stat-hint { font-size: 11px; color: var(--warn); margin-top: 2px; }
.rl-stats .rl-stat { border-color: transparent; }
.rl-stats .rl-stat:nth-child(1) { background: #f1edfc; }
.rl-stats .rl-stat:nth-child(2) { background: #eaf5ed; }
.rl-stats .rl-stat:nth-child(3) { background: #fdf3e3; }
.rl-stats .rl-stat:nth-child(4) { background: #fcecec; }
.rl-stats .rl-stat:nth-child(5) { background: #e8f0fb; }
@media (max-width: 720px) {
  .rl-stats { grid-template-columns: repeat(2, 1fr); }
}

.rl-filter-bar { display: flex; margin-bottom: 10px; }
.rl-year-select { display: inline-flex; align-items: center; gap: 8px; }
.rl-year-label { font-size: 13px; color: #969799; }
.rl-year-native {
  height: 32px; min-width: 120px; padding: 0 8px;
  border: 1px solid #dcdee0; border-radius: 6px;
  font-size: 13px; color: #1f2329; background: #fff; outline: none;
  -webkit-appearance: auto; appearance: auto;
}
.rl-year-native:focus { border-color: var(--info); }
.rl-title { display: flex; align-items: center; gap: 6px; min-width: 0; }
.rl-title > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rl-badge { flex-shrink: 0; border-radius: 999px; }

.rl-list { margin-top: 4px; }
.rl-row-right { display: flex; align-items: center; gap: 10px; }
.rl-row-right .van-button { min-width: 64px; }
/* 金额区固定宽度，确保各行进度条起点与右侧按钮对齐一致 */
.rl-amounts { text-align: right; width: 224px; flex: 0 0 auto; }
.rl-amount { font-weight: 600; color: #1f2329; font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rl-progress { width: 224px; height: 9px; background: #f0f1f2; border-radius: 4px; margin: 4px 0 2px; overflow: hidden; }
.rl-progress-bar { height: 100%; transition: width .3s; }
/* 进度条着色：未收红 / 部分收橙 / 结清绿 / 坏账金 */
.rl-progress-bar.st-open     { background: var(--danger); }
.rl-progress-bar.st-partial  { background: #ff976a; }
.rl-progress-bar.st-closed   { background: var(--jade, #07c160); }
.rl-progress-bar.st-writeoff { background: var(--warn); }
.rl-amount-sub { font-size: 11px; color: #969799; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
@media (max-width: 720px) {
  .rl-amounts { width: 150px; }
  .rl-progress { width: 150px; }
}
.empty { padding: 40px 0; }

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
.accrue-input:focus { border-color: var(--info); }
.accrue-unit { font-size: 12px; color: #969799; }
.accrue-exists { font-size: 11px; color: #969799; background: #e8e8e8; padding: 2px 8px; border-radius: 4px; }
</style>
