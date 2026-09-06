<template>
  <div class="pl-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">单位列表</h2>
        <p class="page-sub">共 {{ parties.length }} 个单位</p>
      </div>
      <div class="pl-header-actions">
        <van-button size="small" @click="exportCSV">导出</van-button>
        <van-button type="primary" size="small" icon="plus" @click="openAdd">新增单位</van-button>
      </div>
    </div>

    <div class="pl-filters">
      <div class="pl-types">
        <button
          v-for="t in typeOptions" :key="t.value"
          class="pl-type-btn"
          :class="{ active: activeType === t.value }"
          @click="activeType = t.value"
        >{{ t.label }}</button>
      </div>
      <van-search v-model="keyword" placeholder="搜索单位名 / 电话" shape="round" background="transparent" class="pl-search" />
    </div>

    <van-loading v-if="loading && filtered.length === 0" />
    <div v-else-if="filtered.length === 0" class="empty">
      <van-empty description="暂无单位" />
    </div>
    <div v-else class="pl-list">
      <table class="pl-table">
        <thead>
          <tr class="pl-tr-head">
            <th class="pl-th">单位名称</th>
            <th class="pl-th">联系电话</th>
            <th class="pl-th">投资金额</th>
            <th class="pl-th">流转面积</th>
            <th class="pl-th">是否有合同</th>
            <th class="pl-th">备注</th>
            <th class="pl-th pl-th-action">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in filtered" :key="p.id" class="pl-tr">
            <td class="pl-td pl-td-name" @click="openDetail(p)">
              <span class="pl-name-link">{{ p.name }}</span>
              <span class="pl-tag" :class="'tag-' + (p.type || 'other')">{{ partyTypeLabel(p.type) }}</span>
            </td>
            <td class="pl-td">{{ p.contactPhone || '—' }}</td>
            <td class="pl-td">
              <template v-if="p.type === 'invest' || p.type === 'reinvest'">{{ fmtYuan(p.investAmountCents) }}</template>
              <span v-else class="pl-na">—</span>
            </td>
            <td class="pl-td">
              <template v-if="p.type === 'flow'">{{ p.landMu ?? p.areaMu ?? 0 }} 亩</template>
              <span v-else class="pl-na">—</span>
            </td>
            <td class="pl-td">
              <span class="pl-has-contract" :class="{ has: contractCount(p.id) > 0 }">
                {{ contractCount(p.id) > 0 ? '有 ' + contractCount(p.id) + ' 份' : '无' }}
              </span>
            </td>
            <td class="pl-td pl-td-note">{{ p.note || '—' }}</td>
            <td class="pl-td pl-td-action">
              <van-button size="mini" plain @click.stop="openEdit(p)">编辑</van-button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 欠款条目弹窗 -->
    <van-dialog v-model:show="showDetailDialog" :title="detailTitle" class="detail-dialog">
      <div v-if="detailLoading" class="detail-loading"><van-loading /></div>
      <div v-else-if="detailItems.length === 0" class="detail-empty">该单位暂无欠款条目</div>
      <div v-else class="detail-list">
        <div v-for="r in detailItems" :key="r.id" class="detail-item">
          <div class="di-head">
            <span class="di-title">{{ r.title }}</span>
            <span class="di-kind" :class="r.recvKind">{{ recvKindLabel(r.recvKind) }}</span>
            <span class="di-status" :class="r.status">{{ r.status === 'open' ? '未结清' : '已结清' }}</span>
          </div>
          <div class="di-amounts">
            <span class="di-amt">应收 <strong>{{ fmtYuan(r.amountCents) }}</strong></span>
            <span class="di-amt di-paid">已收 <strong>{{ fmtYuan(r.paidCents) }}</strong></span>
            <span class="di-amt di-owe">未收 <strong>{{ fmtYuan(r.outstandingCents) }}</strong></span>
          </div>
        </div>
      </div>
    </van-dialog>

    <!-- 新增/编辑往来单位 -->
    <van-dialog v-model:show="showPartyDialog" :title="isEditing ? '编辑往来单位' : '新增往来单位'" show-cancel-button @confirm="saveParty" class="party-edit-dialog">
      <van-form>
        <van-tabs v-model:active="formTab" sticky shrink class="party-form-tabs">
          <van-tab title="基本情况" name="basic">
            <van-cell-group inset>
              <van-field v-model="partyForm.name" label-width="150" label="单位名称" placeholder="如：XX公司 / XX合作社" required />
              <van-field label="单位类型">
                <template #input>
                  <van-radio-group v-model="partyForm.type" direction="horizontal">
                    <van-radio name="invest">长期投资</van-radio>
                    <van-radio name="reinvest">再投资</van-radio>
                    <van-radio name="flow">土地流转</van-radio>
                    <van-radio name="other">其他</van-radio>
                  </van-radio-group>
                </template>
              </van-field>
              <van-field v-model="partyForm.contactPhone" label-width="150" label="联系电话" placeholder="可选" />
            </van-cell-group>

            <van-cell-group inset title="合同与备注">
              <div class="inline-upload">
                <input
                  type="file"
                  id="inline-file-edit"
                  class="hidden-file-input"
                  @change="handleInlineUpload"
                />
                <label for="inline-file-edit" class="upload-trigger inline">
                  <van-icon name="plus" />
                  <span>{{ inlineUploading ? '上传中…' : '点击选择合同/附件' }}</span>
                </label>
                <div v-if="inlineContracts.length > 0" class="inline-contract-list">
                  <div v-for="(c, i) in inlineContracts" :key="i" class="inline-contract-item">
                    <van-icon name="description" class="contract-icon" />
                    <span class="inline-ctitle">{{ splitCleanFileName(c.fileName).cleanName + splitCleanFileName(c.fileName).ext }}</span>
                    <span class="inline-cstate" v-if="c._status === 'uploading'">上传中…</span>
                    <span class="inline-cstate ok" v-else>✓</span>
                    <van-icon name="cross" class="inline-cremove" @click="inlineContracts.splice(i, 1)" />
                  </div>
                </div>
              </div>
              <van-field v-model="partyForm.note" label-width="150" label="备注" placeholder="备注（可选）" />
            </van-cell-group>
          </van-tab>

          <van-tab title="年度数据" name="data">
            <van-cell-group inset v-if="partyForm.type === 'invest' || partyForm.type === 'reinvest'" title="投资信息">
              <van-field v-model="partyForm.investAmountYuan" label-width="150" type="number" label="投资本金（元）" placeholder="如 500000" inputmode="decimal" />
              <van-field v-model="partyForm.returnRatePercent" label-width="150" type="number" label="年收益率" placeholder="如 0.04" inputmode="decimal" />
              <van-field v-model="partyForm.expectedReturnYuan" label-width="150" type="number" label="年收益（元）" placeholder="自动计算，可修改" inputmode="decimal" />
            </van-cell-group>

            <van-cell-group inset v-else-if="partyForm.type === 'flow'" title="土地流转信息">
              <van-field v-model="partyForm.landMuYuan" label-width="150" type="number" label="流转亩数" placeholder="如 200" inputmode="decimal" />
              <van-field v-model="partyForm.landFeePerMuYuan" label-width="150" type="number" label="每亩年流转费" placeholder="如 150" inputmode="decimal" />
              <van-field v-model="partyForm.expectedLandFeeYuan" label-width="150" type="number" label="总流转费（元）" placeholder="自动计算，可修改" inputmode="decimal" />
              <van-field v-model="partyForm.mgmtFeePerMuYuan" label-width="150" type="number" label="每亩年管理费" placeholder="如 15" inputmode="decimal" />
              <van-field v-model="partyForm.expectedMgmtFeeYuan" label-width="150" type="number" label="总管理费（元）" placeholder="自动计算，可修改" inputmode="decimal" />
            </van-cell-group>

            <van-cell-group inset v-else title="其他类型">
              <van-field label="说明" readonly value="该单位不参与年度投资/流转费结转，需要时手动登记应收" />
            </van-cell-group>
          </van-tab>
        </van-tabs>
      </van-form>
    </van-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { api } from '../../../lib/http'
import { showToast, showDialog } from 'vant'

// 统一从 API 响应里提取数组（兼容 data / data.items / 直接数组）
function extractList(r: any) {
  if (r == null) return [];
  if (Array.isArray(r)) return r;
  if (Array.isArray(r.data)) return r.data;
  if (r.data && Array.isArray(r.data.items)) return r.data.items;
  return [];
}

interface Party {
  id: number; name: string; type: string; contactPhone?: string;
  investAmountCents?: number; returnRateBps?: number; expectedReturnCents?: number;
  landMu?: number; landFeePerMuCents?: number; expectedLandFeeCents?: number;
  expectedMgmtFeeCents?: number; note?: string; areaMu?: number;
  mgmtFeePerMuCents?: number; outstandingCents?: number;
}

const typeOptions = [
  { value: 'all', label: '全部' },
  { value: 'invest', label: '长期投资单位' },
  { value: 'reinvest', label: '再投资单位' },
  { value: 'flow',    label: '土地流转企业' },
  { value: 'other',   label: '其他单位' },
]
const parties = ref<Party[]>([])
const loading = ref(false)
const activeType = ref('all')
const keyword = ref('')

const partyTypeLabel = (t?: string) => ({
  invest: '长投', reinvest: '再投', flow: '流转', other: '其他',
}[t || 'other'] || '其他')

const filtered = computed(() => {
  let arr = parties.value
  if (activeType.value !== 'all') arr = arr.filter(p => p.type === activeType.value)
  const k = keyword.value.trim().toLowerCase()
  if (k) arr = arr.filter(p => (p.name || '').toLowerCase().includes(k) || (p.contactPhone || '').includes(k))
  return arr
})

const fmtYuan = (c?: number) => '¥' + ((c || 0) / 100).toLocaleString('zh-CN', { minimumFractionDigits: 0 })

// ---- 欠款条目弹窗 ----
const showDetailDialog = ref(false)
const detailCurrentParty = ref<Party | null>(null)
const detailLoading = ref(false)
const detailItems = ref<any[]>([])

const detailTitle = computed(() => {
  const p = detailCurrentParty.value
  return p ? p.name + ' - 欠款条目' : '欠款条目'
})

const recvKindLabel = (k?: string) => ({
  dividend: '投资收益', rent: '土地流转费', service: '流转管理费', other: '其他',
}[k || 'other'] || '其他')

async function openDetail(p: Party) {
  detailCurrentParty.value = p
  showDetailDialog.value = true
  detailLoading.value = true
  detailItems.value = []
  try {
    const r = await api.get<any>('/receivables')
    const list = extractList(r)
    detailItems.value = list.filter((item: any) => item.partyId === p.id)
  } catch {
    detailItems.value = []
  } finally { detailLoading.value = false }
}

interface Contract { id: number; partyId: number; fileName: string; }
const contracts = ref<Contract[]>([])

function contractCount(partyId: number): number {
  return contracts.value.filter(c => c.partyId === partyId).length
}

function downloadCSV(filename: string, headers: string[], rows: string[][]) {
  const csv = [headers.join(','), ...rows.map(r => r.map(c => '"' + (c || '').replace(/"/g, '""') + '"').join(','))].join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = filename; a.click()
  URL.revokeObjectURL(url)
}

function exportCSV() {
  const headers = ['单位名称', '单位类型', '联系电话', '投资金额', '流转面积', '是否有合同', '备注']
  const rows = parties.value.map(p => [
    p.name,
    partyTypeLabel(p.type),
    p.contactPhone || '',
    (p.type === 'invest' || p.type === 'reinvest') ? fmtYuan(p.investAmountCents) : '',
    p.type === 'flow' ? ((p.landMu ?? p.areaMu ?? 0) + ' 亩') : '',
    contractCount(p.id) > 0 ? '有 ' + contractCount(p.id) + ' 份' : '无',
    p.note || '',
  ])
  downloadCSV('单位列表.csv', headers, rows)
}

async function loadContracts() {
  try {
    const r = await api.get<{ data: Contract[] } | Contract[]>('/contracts')
    contracts.value = extractList(r)
  } catch { contracts.value = [] }
}

async function load() {
  loading.value = true
  try {
    const r = await api.get<{ data: Party[] } | Party[]>('/parties')
    parties.value = extractList(r)
  } catch {
    parties.value = []
  } finally { loading.value = false }
  await loadContracts()
}

// ---- 新增/编辑表单 ----
const showPartyDialog = ref(false)
const isEditing = ref(false)
const formTab = ref<'basic' | 'data'>('basic')

interface InlineContract { fileName: string; fileSize: number; mimeType: string; fileData: string; _status?: 'uploading' | 'ok' | 'error' }
const inlineContracts = ref<InlineContract[]>([])
const inlineUploading = ref(false)

const partyForm = ref({
  id: 0,
  name: '',
  type: 'flow' as 'flow' | 'invest' | 'reinvest' | 'other',
  contactPhone: '',
  note: '',
  investAmountYuan: '',
  returnRatePercent: '',
  expectedReturnYuan: '',
  landMuYuan: '',
  landFeePerMuYuan: '',
  expectedLandFeeYuan: '',
  mgmtFeePerMuYuan: '',
  expectedMgmtFeeYuan: '',
})

// 自动计算辅助
function autoInvestReturn() {
  const amtYuan = parseFloat(partyForm.value.investAmountYuan || '0') || 0
  const rate = parseFloat(partyForm.value.returnRatePercent || '0') || 0
  return Math.round(amtYuan * rate)
}
function autoLandFee() {
  const mu = parseFloat(partyForm.value.landMuYuan || '0') || 0
  const perMu = parseFloat(partyForm.value.landFeePerMuYuan || '0') || 0
  return Math.round(mu * perMu)
}
function autoMgmtFee() {
  const mu = parseFloat(partyForm.value.landMuYuan || '0') || 0
  const perMu = parseFloat(partyForm.value.mgmtFeePerMuYuan || '0') || 0
  return Math.round(mu * perMu)
}

let lastAutoReturn: number | null = null
let lastAutoLandFee: number | null = null
let lastAutoMgmt: number | null = null

watch(
  () => [partyForm.value.investAmountYuan, partyForm.value.returnRatePercent],
  () => {
    const auto = autoInvestReturn()
    const cur = parseFloat(partyForm.value.expectedReturnYuan || '0') || 0
    if (lastAutoReturn === null || cur === lastAutoReturn) {
      partyForm.value.expectedReturnYuan = auto > 0 ? String(auto) : ''
    }
    lastAutoReturn = auto
  }
)

watch(
  () => [partyForm.value.landMuYuan, partyForm.value.landFeePerMuYuan],
  () => {
    const auto = autoLandFee()
    const cur = parseFloat(partyForm.value.expectedLandFeeYuan || '0') || 0
    if (lastAutoLandFee === null || cur === lastAutoLandFee) {
      partyForm.value.expectedLandFeeYuan = auto > 0 ? String(auto) : ''
    }
    lastAutoLandFee = auto
  }
)

watch(
  () => [partyForm.value.landMuYuan, partyForm.value.mgmtFeePerMuYuan],
  () => {
    const auto = autoMgmtFee()
    const cur = parseFloat(partyForm.value.expectedMgmtFeeYuan || '0') || 0
    if (lastAutoMgmt === null || cur === lastAutoMgmt) {
      partyForm.value.expectedMgmtFeeYuan = auto > 0 ? String(auto) : ''
    }
    lastAutoMgmt = auto
  }
)

function resetForm() {
  partyForm.value = {
    id: 0, name: '', type: 'flow', contactPhone: '', note: '',
    investAmountYuan: '', returnRatePercent: '', expectedReturnYuan: '',
    landMuYuan: '', landFeePerMuYuan: '', expectedLandFeeYuan: '',
    mgmtFeePerMuYuan: '', expectedMgmtFeeYuan: '',
  }
  inlineContracts.value = []
  formTab.value = 'basic'
  lastAutoReturn = null
  lastAutoLandFee = null
  lastAutoMgmt = null
}

function openAdd() {
  resetForm()
  isEditing.value = false
  showPartyDialog.value = true
}

function openEdit(p: Party) {
  resetForm()
  isEditing.value = true
  const type = (p.type && ['flow', 'invest', 'reinvest', 'other'].includes(p.type)) ? p.type : 'flow'
  partyForm.value = {
    id: p.id,
    name: p.name,
    type: type as any,
    contactPhone: p.contactPhone || '',
    note: p.note || '',
    investAmountYuan: (p.investAmountCents! / 100).toFixed(2),
    returnRatePercent: p.returnRateBps ? (p.returnRateBps / 10000).toFixed(4) : '',
    expectedReturnYuan: (p.expectedReturnCents! / 100).toFixed(2),
    landMuYuan: p.landMu ? String(p.landMu) : (p.areaMu ? String(p.areaMu) : ''),
    landFeePerMuYuan: (p.landFeePerMuCents! / 100).toFixed(2),
    expectedLandFeeYuan: (p.expectedLandFeeCents! / 100).toFixed(2),
    mgmtFeePerMuYuan: (p.mgmtFeePerMuCents! / 100).toFixed(2),
    expectedMgmtFeeYuan: (p.expectedMgmtFeeCents! / 100).toFixed(2),
  }
  showPartyDialog.value = true
}

async function saveParty() {
  if (!partyForm.value.name) {
    showToast('请填写单位名称')
    return
  }
  const f = partyForm.value
  const toCents = (yuan: string) => Math.round(parseFloat(yuan || '0') * 100)
  const autoER = autoInvestReturn()
  const autoLF = autoLandFee()
  const autoMF = autoMgmtFee()
  const expectedReturnInput = parseFloat(f.expectedReturnYuan || '0')
  const expectedLandFeeInput = parseFloat(f.expectedLandFeeYuan || '0')
  const expectedMgmtFeeInput = parseFloat(f.expectedMgmtFeeYuan || '0')

  const payload: Record<string, unknown> = {
    name: f.name,
    type: f.type,
    types: [f.type],
    contactPhone: f.contactPhone.trim(),
    note: f.note,
    investAmountCents: toCents(f.investAmountYuan),
    returnRateBps: Math.round(parseFloat(f.returnRatePercent || '0') * 10000),
    expectedReturnCents: toCents(expectedReturnInput > 0 ? f.expectedReturnYuan : String(autoER)),
    landMu: parseFloat(f.landMuYuan || '0') || 0,
    areaMu: parseFloat(f.landMuYuan || '0') || 0,
    landFeePerMuCents: toCents(f.landFeePerMuYuan),
    expectedLandFeeCents: toCents(expectedLandFeeInput > 0 ? f.expectedLandFeeYuan : String(autoLF)),
    mgmtFeePerMuCents: toCents(f.mgmtFeePerMuYuan),
    expectedMgmtFeeCents: toCents(expectedMgmtFeeInput > 0 ? f.expectedMgmtFeeYuan : String(autoMF)),
  }
  try {
    let partyId = f.id
    if (isEditing.value) {
      await api.put(`/parties/${partyId}`, payload)
      showToast('更新成功')
    } else {
      const res = await api.post<any>('/parties', payload)
      partyId = res?.data?.id ?? res?.id ?? 0
      showToast('创建成功')
    }
    // 上传保存后关联的合同
    if (inlineContracts.value.length > 0 && partyId > 0) {
      await Promise.all(inlineContracts.value.map(c =>
        api.post('/contracts', {
          partyId,
          fileName: c.fileName,
          fileSize: c.fileSize,
          mimeType: c.mimeType,
          contractTitle: c.fileName,
          fileData: c.fileData,
        })
      ))
      inlineContracts.value = []
    }
    showPartyDialog.value = false
    await load()
  } catch (e: any) {
    showDialog({ title: '保存失败', message: e.message || '保存失败，请重试' })
  }
}

// ---- 合同上传 ----
function splitCleanFileName(name: string) {
  const dotIdx = name.lastIndexOf('.')
  if (dotIdx > 0) return { cleanName: name.slice(0, dotIdx), ext: name.slice(dotIdx) }
  return { cleanName: name, ext: '' }
}

async function handleInlineUpload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024) {
    showToast('文件超过 5MB 限制')
    return
  }
  const item: InlineContract = {
    fileName: file.name,
    fileSize: file.size,
    mimeType: file.type || 'application/octet-stream',
    fileData: '',
    _status: 'uploading',
  }
  inlineContracts.value.push(item)
  try {
    const dataUrl = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(reader.result as string)
      reader.onerror = () => reject(new Error('读取文件失败'))
      reader.readAsDataURL(file)
    })
    item.fileData = dataUrl
    item._status = 'ok'
  } catch {
    item._status = 'error'
  } finally {
    inlineUploading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page-header { margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; }
.page-title { font-size: 18px; font-weight: 600; margin: 0; }
.page-sub   { font-size: 12px; color: #969799; margin: 4px 0 0; }
.pl-header-actions { display: flex; gap: 8px; align-items: center; }

.pl-filters { display: flex; flex-direction: column; gap: 8px; margin-bottom: 12px; }
.pl-types { display: flex; gap: 6px; flex-wrap: wrap; }
.pl-type-btn {
  padding: 6px 12px; border-radius: 20px; font-size: 13px; border: 1px solid #eaeaea;
  background: #fff; color: #646566; cursor: pointer;
}
.pl-type-btn.active { background: var(--jade, #07c160); color: #fff; border-color: var(--jade, #07c160); }
.pl-search { padding: 0; }

.pl-list { overflow-x: auto; }
.pl-table { width: 100%; border-collapse: collapse; font-size: 12px; table-layout: auto; }
.pl-th {
  background: #f7f8fa; color: #969799; font-weight: 500; font-size: 11px;
  padding: 8px 6px; text-align: left; white-space: nowrap; border-bottom: 1px solid #ebedf0;
  position: sticky; top: 0; z-index: 1;
}
.pl-tr { cursor: pointer; transition: background .12s; }
.pl-tr:hover { background: #f7f8fa; }
.pl-td { padding: 8px 6px; border-bottom: 1px solid #f2f3f5; color: #1f2329; vertical-align: middle; }
.pl-td-name { white-space: nowrap; }
.pl-name-link { font-weight: 600; font-size: 13px; color: #1989fa; cursor: pointer; margin-right: 6px; }
.pl-name-link:hover { text-decoration: underline; }
.pl-th-action, .pl-td-action { text-align: center; width: 60px; }
.pl-name { font-weight: 600; font-size: 13px; color: #1f2329; margin-right: 6px; }
.pl-tag { font-size: 10px; padding: 1px 6px; border-radius: 8px; white-space: nowrap; }
.tag-invest   { background: #e6f1ff; color: #1989fa; }
.tag-reinvest { background: #f0e6ff; color: #764ba2; }
.tag-flow     { background: #fff2e6; color: #ff6034; }
.tag-other    { background: #f2f3f5; color: #646566; }
.pl-na { color: #c8c9cc; }
.pl-td-note { max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #969799; }
.pl-has-contract { color: #c8c9cc; }
.pl-has-contract.has { color: #07c160; font-weight: 600; }
.empty { padding: 40px 0; }

/* 表单样式 */
.hidden-file-input { display: none; }
.inline-upload { padding: 12px 16px; }
.upload-trigger {
  display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px;
  border: 1px dashed #dcdee0; border-radius: 8px; cursor: pointer; color: #1989fa; font-size: 13px;
}
.upload-trigger:hover { border-color: #1989fa; background: #f7f8fa; }
.inline-contract-list { margin-top: 8px; }
.inline-contract-item { display: flex; align-items: center; gap: 8px; padding: 6px 0; font-size: 13px; }
.inline-ctitle { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.inline-cstate { color: #969799; font-size: 12px; }
.inline-cstate.ok { color: #07c160; }
.inline-cremove { cursor: pointer; color: #969799; }
.inline-cremove:hover { color: #ee0a24; }
.contract-icon { font-size: 18px; color: #1989fa; }

/* 欠款弹窗 */
.detail-dialog { width: 90vw; max-width: 480px; }
.detail-loading, .detail-empty { padding: 40px 0; text-align: center; color: #969799; font-size: 13px; }
.detail-list { padding: 8px 16px 16px; display: flex; flex-direction: column; gap: 8px; max-height: 60vh; overflow-y: auto; }
.detail-item { background: #f7f8fa; border-radius: 8px; padding: 10px 12px; }
.di-head { display: flex; align-items: center; gap: 6px; margin-bottom: 6px; }
.di-title { font-weight: 500; font-size: 13px; color: #1f2329; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.di-kind { font-size: 10px; padding: 1px 6px; border-radius: 6px; background: #e6f1ff; color: #1989fa; white-space: nowrap; }
.di-kind.rent { background: #fff2e6; color: #ff6034; }
.di-kind.service { background: #f0e6ff; color: #764ba2; }
.di-status { font-size: 10px; padding: 1px 6px; border-radius: 6px; white-space: nowrap; }
.di-status.open { background: #fffbe6; color: #d4a017; }
.di-status.closed { background: #e6f7e6; color: #07c160; }
.di-amounts { display: flex; gap: 12px; font-size: 12px; color: #646566; }
.di-amt strong { font-weight: 600; color: #1f2329; margin-left: 2px; }
.di-paid strong { color: #07c160; }
.di-owe strong { color: #ee0a24; }
</style>