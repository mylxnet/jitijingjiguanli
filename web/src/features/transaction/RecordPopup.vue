<template>
  <van-popup
    v-model:show="showRecord"
    :position="isDesktop ? 'center' : 'bottom'"
    :transition="isDesktop ? '' : undefined"
    :round="!isDesktop"
    closeable
    :style="isDesktop ? 'width:600px;border-radius:12px;max-height:85vh;overflow:auto' : 'max-height: 90vh'"
  >
    <div class="record-popup">
      <div class="popup-title">记一笔</div>
      <div class="record-modes">
        <van-button class="mode-btn" :type="recordMode === 'quick' ? 'primary' : 'default'" @click="recordMode = 'quick'">快速记账</van-button>
        <van-button class="mode-btn" :type="recordMode === 'normal' ? 'primary' : 'default'" @click="recordMode = 'normal'">普通记账</van-button>
      </div>

      <template v-if="recordMode === 'normal'">
        <div class="direction-toggle">
          <van-button :type="record.direction === 'income' ? 'primary' : 'default'" size="small" @click="record.direction = 'income'">收入</van-button>
          <van-button :type="record.direction === 'expense' ? 'primary' : 'default'" size="small" @click="record.direction = 'expense'">支出</van-button>
        </div>
        <van-field v-model="record.date" label="日期" placeholder="YYYY-MM-DD" />
        <van-field v-model="record.note" label="摘要" placeholder="收到某某公司的什么款" />
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
        <van-field v-if="quickNeedCompany" label="投资对象（长期投资）">
          <template #input>
            <select v-model="quick.companyId" class="qselect">
              <option :value="0" disabled>选择投资公司</option>
              <option v-for="a in investCompanyOptions" :key="a.value" :value="a.value">{{ a.text }}</option>
            </select>
          </template>
        </van-field>
        <div v-if="quickNeedCompany && !showNewCompany" class="new-company-link" @click="showNewCompany = true">
          ＋ 没有这家公司？新增长期投资公司
        </div>
        <template v-if="quickNeedCompany && showNewCompany">
          <van-field v-model="newCompanyName" label="新公司名" placeholder="输入公司全称" />
          <div class="new-company-hint">创建后自动在「长期投资」下建该公司科目，并在往来中新增同名单位</div>
          <div class="new-company-actions">
            <van-button size="small" plain @click="showNewCompany = false">取消</van-button>
            <van-button size="small" type="primary" :loading="creatingCompany" @click="handleCreateCompany">创建并选中</van-button>
          </div>
        </template>
        <van-field v-if="quickNeedParty" label="往来单位">
          <template #input>
            <select v-model="quick.partyId" class="qselect">
              <option :value="0" disabled>选择单位</option>
              <option v-for="p in partyOptions" :key="p.value" :value="p.value">{{ p.text }}</option>
            </select>
          </template>
        </van-field>
        <van-field v-if="quickManualNote" v-model="quick.note" label="摘要" placeholder="请输入摘要" />
        <van-field v-model="quick.amount" label="金额" type="number" placeholder="0.00" inputmode="decimal" />
      </template>

      <div v-if="recordError" class="record-error">{{ recordError }}</div>
      <div class="record-save">
        <van-button round block type="primary" :loading="saving" @click="saveRecord">
          {{ recordMode === 'quick' ? '保存（自动入账）' : '保存' }}
        </van-button>
      </div>
    </div>

    <!-- 科目选择 -->
    <van-popup v-model:show="showCatPicker" position="bottom">
      <van-picker :columns="catColumns" @confirm="onCatConfirm" @cancel="showCatPicker = false" />
    </van-popup>
  </van-popup>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../lib/http'
import { formatFen, todayStr, recvKindLabel } from '../../types/api'
import { showToast } from 'vant'
import { useContactIssues } from '../contacts/useContactIssues'
import type { Category, Party, Receivable, ReceivableListResponse, RecvKind, ApiResponse, Transaction } from '../../types/api'

// 记账会改动单位投资本金，同步刷新侧边栏「数据缺失」角标
const { refresh: refreshIssues } = useContactIssues()

const emit = defineEmits<{
  saved: []
}>()

const isDesktop = window.innerWidth > 768
const showRecord = ref(false)
const saving = ref(false)
const recordError = ref('')
const record = ref({ date: todayStr(), note: '', categoryName: '', categoryId: null as number | null, amount: '', direction: 'expense' as 'income' | 'expense' })
const showCatPicker = ref(false)
const catColumns = ref<{ text: string; value: number }[]>([])

// ---- 快速记账（业务模板自动入账） ----
const recordMode = ref<'normal' | 'quick'>('quick')
const fullCats = ref<Category[]>([])
const investCompanyOptions = ref<{ text: string; value: number }[]>([])
const allParties = ref<Party[]>([])
const partyOptions = computed(() => {
  const b = quickBusinesses.find(x => x.key === quick.value.biz)
  if (b?.partyTypesFilter?.length) {
    const allowed = new Set(b.partyTypesFilter)
    return allParties.value
      .filter(p => {
        const types = p.types && p.types.length ? p.types : (p.type ? [p.type] : [])
        return types.some(t => allowed.has(t))
      })
      .map(p => ({ text: p.name, value: p.id }))
  }
  return allParties.value.map(p => ({ text: p.name, value: p.id }))
})
const quick = ref({ date: todayStr(), biz: '', companyId: null as number | null, partyId: null as number | null, recvId: null as number | null, amount: '', note: '' })
const showNewCompany = ref(false)
const newCompanyName = ref('')
const creatingCompany = ref(false)

interface QuickBiz {
  key: string
  label: string
  dir?: 'income' | 'expense'
  catName?: string
  invest?: boolean
  needParty?: boolean
  autoBuildL1?: string
  defaultNote?: string
  partyTypesFilter?: ('flow' | 'invest' | 'reinvest' | 'other')[]
  group?: string
  // 投资模板：主流水直接记入「待投资」科目（银行存款、待投资同步减少），而非计入该公司科目，
  // 公司投资额由往来单位投资额字段展示，不涉科目结转。
  recordToPool?: boolean
  // 快速记账需要用户输入摘要（弹窗显示「摘要」输入框，默认值 = defaultNote）。
  manualNote?: boolean
}
const quickBusinesses: QuickBiz[] = [
  // 资金类
  { key: 'grant', label: '收上级财政补助', dir: 'income', catName: '上级补助', defaultNote: '收上级财政补助', group: '资金类' },
  { key: 'otherFinance', label: '收到其他财政性收入', dir: 'income', catName: '待投资', defaultNote: '收到其他财政性收入', group: '资金类' },
  { key: 'invest', label: '长期投资给公司', invest: true, dir: 'expense', group: '资金类' },
  { key: 'otherInvest', label: '其他财政性收入投资给公司', invest: true, dir: 'expense', catName: '待投资', recordToPool: true, defaultNote: '其他财政性收入投资给公司', group: '资金类' },
  { key: 'dividend', label: '收到投资收益/分红', dir: 'income', needParty: true, autoBuildL1: '投资收益', defaultNote: '收到 {party} 投资收益', partyTypesFilter: ['invest', 'reinvest'], group: '资金类' },
  { key: 'recover', label: '收回投资', dir: 'income', needParty: true, catName: '待投资', defaultNote: '收回 {party} 投资', partyTypesFilter: ['invest', 'reinvest'], group: '资金类' },
  { key: 'reinvest', label: '532-再投资', dir: 'expense', needParty: true, autoBuildL1: '再投资', defaultNote: '532再投资给 {party}', partyTypesFilter: ['invest', 'reinvest'], group: '资金类' },
  { key: 'member', label: '532-成员分配发放', dir: 'expense', catName: '成员分红', defaultNote: '成员分红', group: '资金类' },
  { key: 'welfare', label: '532-公益支出', dir: 'expense', catName: '公益支出', defaultNote: '公益支出', group: '资金类' },
  // 流转类
  { key: 'rent', label: '收到土地流转费', dir: 'income', needParty: true, autoBuildL1: '土地流转费收入', defaultNote: '收到 {party} 土地流转费', partyTypesFilter: ['flow'], group: '流转类' },
  { key: 'service', label: '收到流转管理费', dir: 'income', needParty: true, autoBuildL1: '流转管理费', defaultNote: '收到 {party} 流转管理费', partyTypesFilter: ['flow'], group: '流转类' },
  { key: 'toHousehold', label: '拨付流转费给农户', dir: 'expense', catName: '土地流转费-转付农户', defaultNote: '拨付流转费给农户', group: '流转类' },
  { key: 'mgmtFee', label: '支出管理费', dir: 'expense', catName: '管理费支出', defaultNote: '管理费支出', group: '流转类' },
  // 其他
  { key: 'otherIncome', label: '收到其他收入', dir: 'income', catName: '其他收入', defaultNote: '收到其他收入', manualNote: true, group: '其他' },
  { key: 'otherPay', label: '拨出其他收入', dir: 'expense', catName: '其他收入', defaultNote: '拨付其他收入', manualNote: true, group: '其他' },
  { key: 'interest', label: '银行存款利息', dir: 'income', catName: '其他收入', defaultNote: '银行存款利息', group: '其他' },
]

const quickNeedCompany = computed(() => {
  const b = quickBusinesses.find(x => x.key === quick.value.biz)
  return !!(b && b.invest)
})
const quickNeedParty = computed(() => {
  const b = quickBusinesses.find(x => x.key === quick.value.biz)
  return !!(b && b.needParty)
})
const quickManualNote = computed(() => {
  const b = quickBusinesses.find(x => x.key === quick.value.biz)
  return !!(b && b.manualNote)
})

function onQuickBizChange() {
  quick.value.companyId = null
  quick.value.partyId = null
  showNewCompany.value = false
  newCompanyName.value = ''
  // 需要手动摘要的业务：默认填入模板预设摘要
  const b = quickBusinesses.find(x => x.key === quick.value.biz)
  if (b?.manualNote) quick.value.note = b.defaultNote || b.label
}

function partyNameById(id: number | null | undefined): string {
  if (!id) return ''
  return allParties.value.find(p => p.id === id)?.name || ''
}
function fillNote(template: string, partyName: string): string {
  return template.replace('{party}', partyName || '')
}
function findL1ByName(name: string): Category | null {
  return fullCats.value.find(l1 => l1.name === name && l1.level === 1) || null
}
async function ensureL2InL1(l1Name: string, l2Name: string): Promise<Category> {
  let l1 = findL1ByName(l1Name)
  if (!l1) throw new Error(`缺少一级科目：${l1Name}`)
  const existing = (l1.children || []).find(c => c.name === l2Name)
  if (existing) return existing
  const res = await api.post<ApiResponse<Category>>('/categories', {
    name: l2Name, level: 2, parentId: l1.id, kind: 'equity',
  })
  const newL2 = res.data
  if (l1.children) l1.children.push(newL2)
  else l1.children = [newL2]
  return newL2
}
const INVEST_L1_NAMES = ['长期投资', '对外投资']
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
function investL1(): Category | null {
  for (const l1 of fullCats.value) {
    if (INVEST_L1_NAMES.includes(l1.name)) return l1
  }
  return null
}
async function ensureInvestL1(): Promise<Category> {
  const existing = investL1()
  if (existing) return existing
  await api.post('/categories', { name: '长期投资', level: 1 })
  await loadCats()
  const created = investL1()
  if (!created) throw new Error('创建「长期投资」分组失败，请刷新后重试')
  return created
}

async function loadPartiesQuick() {
  try {
    const res = await api.get<ApiResponse<Party[]>>('/parties')
    allParties.value = res.data || []
  } catch {
    allParties.value = []
  }
}

async function loadCats() {
  try {
    const res = await api.get<ApiResponse<Category[]>>('/categories')
    const cats = res.data
    fullCats.value = cats
    const options: { text: string; value: number }[] = []
    const companies: { text: string; value: number }[] = []
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
    catColumns.value = options
    investCompanyOptions.value = companies
  } catch {
    catColumns.value = []
    investCompanyOptions.value = []
  }
}

async function handleCreateCompany() {
  const name = newCompanyName.value.trim()
  if (!name) {
    showToast('请填写公司名称')
    return
  }
  creatingCompany.value = true
  try {
    const existing = investCompanyOptions.value.find(o => o.text === name || o.text.startsWith(`${name}（`))
    if (existing) {
      quick.value.companyId = existing.value
      showNewCompany.value = false
      newCompanyName.value = ''
      showToast('该公司已存在，已自动选中')
      return
    }
    const l1 = await ensureInvestL1()
    const res = await api.post<ApiResponse<Category>>('/categories', {
      name,
      level: 2,
      parentId: l1.id,
      kind: 'equity',
    })
    if (!partyOptions.value.some(p => p.text === name)) {
      await api.post('/parties', { name, type: 'invest', note: '自动创建（长期投资）' })
    }
    await loadCats()
    await loadPartiesQuick()
    const cat = investCompanyOptions.value.find(o => o.value === (res.data?.id ?? -1))
      || investCompanyOptions.value.find(o => o.text === name || o.text.startsWith(`${name}（`))
    if (cat) quick.value.companyId = cat.value
    showNewCompany.value = false
    newCompanyName.value = ''
    showToast('已创建并选中')
  } catch (e: any) {
    showToast(e.message || '创建失败')
  } finally {
    creatingCompany.value = false
  }
}

// 长期投资给公司：累计该单位的投资本金（与投资页"投资金额"口径一致，便于对账）
async function syncInvestedAmount(amountCents: number) {
  const opt = investCompanyOptions.value.find(o => o.value === quick.value.companyId)
  const compName = (opt?.text || '').replace(/（.*$/, '').trim()
  if (!compName) return
  try {
    const res = await api.get<ApiResponse<Party[]>>('/parties')
    const list = res.data || []
    const p = list.find(x => x.name === compName && ((x.types && x.types.includes('invest')) || x.type === 'invest'))
    if (!p) return
    await api.put(`/parties/${p.id}`, { investAmountCents: (p.investAmountCents || 0) + amountCents })
    refreshIssues()
  } catch {
    // 投资额同步失败不阻断记账（科目余额为权威口径）
  }
}

// 业务 → 应收类别：收益类按所选单位类型区分（再投资单位 → 再投资收益）
function resolveRecvKind(bizKey: string): RecvKind | '' {
  if (bizKey === 'rent') return 'rent'
  if (bizKey === 'service') return 'service'
  if (bizKey === 'dividend') {
    const p = allParties.value.find(x => x.id === quick.value.partyId)
    const types = p?.types && p.types.length ? p.types : (p?.type ? [p.type] : [])
    return types.includes('reinvest') ? 'reinvest_dividend' : 'dividend'
  }
  return ''
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
  if (quickNeedCompany.value && !quick.value.companyId) {
    showToast('请选择投资公司')
    return
  }
  if (quickNeedParty.value && !quick.value.partyId) {
    showToast('请选择往来单位')
    return
  }
  const partyName = partyNameById(quick.value.partyId)
  // 可手动输入摘要的业务优先用用户输入，空则回退模板默认
  const note = biz.manualNote
    ? (quick.value.note.trim() || fillNote(biz.defaultNote || biz.label, partyName))
    : fillNote(biz.defaultNote || biz.label, partyName)
  saving.value = true
  recordError.value = ''
  try {
    const recvKind = resolveRecvKind(biz.key)
    let receiptCreated = false
    if (biz.dir === 'income' && quick.value.partyId && recvKind) {
      if (quick.value.recvId) {
        // 指定单张收缴（应收列表「收缴」入口）：只抵这张单，金额不能超过该单未收
        const list = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', {
          partyId: quick.value.partyId, pageSize: 200,
        })
        const target = (list.data.items || []).find(r => r.id === quick.value.recvId && r.recvKind === recvKind)
        if (!target) throw new Error('该应收单不存在或不属于所选单位')
        const outstanding = target.outstandingCents || 0
        if (amount > outstanding) {
          throw new Error(`该单 ${biz.label} 待收 ${formatFen(outstanding)}，不能超过`)
        }
        await api.post(`/receivables/${target.id}/receipts`, {
          amountCents: amount,
          receiptDate: quick.value.date,
          method: 'cash',
          note: '收缴核销（' + biz.label + '）',
        })
        receiptCreated = true
      } else {
        // 未指定单据：整额收款，后端按最早年度优先自动摊分核销该单位该类全部欠单并自动入账
        const list = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', {
          partyId: quick.value.partyId, status: 'open', pageSize: 200,
        })
        const matches = (list.data.items || []).filter(r => r.recvKind === recvKind && r.outstandingCents > 0)
        if (matches.length > 0) {
          const totalOutstanding = matches.reduce((s, r) => s + (r.outstandingCents || 0), 0)
          if (amount > totalOutstanding) {
            throw new Error(`该单位 ${recvKindLabel[recvKind]} 待收合计 ${formatFen(totalOutstanding)}，不能超过`)
          }
          await api.post('/party-collect', {
            partyId: quick.value.partyId,
            recvKind,
            amountCents: amount,
            receiptDate: quick.value.date,
            note,
          })
          receiptCreated = true
        }
      }
    }
    if (!receiptCreated) {
      let categoryId: number
      if (biz.recordToPool) {
        // 投资给公司但直接计入「待投资」：银行存款、待投资同步减少，公司投资额由往来单位字段展示
        if (!biz.catName) throw new Error('模板缺少 catName')
        const pool = findEquityCat(biz.catName)
        if (!pool) throw new Error(`缺少科目：${biz.catName}`)
        categoryId = pool.id
      } else if (biz.invest) {
        categoryId = quick.value.companyId!
      } else if (biz.autoBuildL1) {
        if (!quick.value.partyId) throw new Error('缺少往来单位')
        const l2 = await ensureL2InL1(biz.autoBuildL1, partyName)
        categoryId = l2.id
      } else {
        if (!biz.catName) throw new Error('模板缺少 catName')
        const cat = findEquityCat(biz.catName)
        if (!cat) throw new Error(`缺少科目：${biz.catName}`)
        categoryId = cat.id
      }
      // 投资给公司：累计该单位投资本金
      await api.post('/transactions', {
        txnDate: quick.value.date,
        direction: biz.dir || 'income',
        amountCents: amount,
        categoryId,
        note,
        partyId: quick.value.partyId || null,
      })
      if (biz.invest) {
        await syncInvestedAmount(amount)
      }
    }
    showToast(receiptCreated ? '保存成功（已核销应收并入账）' : '保存成功（已自动入账）')
    showRecord.value = false
    emit('saved')
  } catch (e: any) {
    recordError.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
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
    emit('saved')
  } catch (e: any) {
    recordError.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
}

interface OpenRecordOptions {
  biz?: string
  partyId?: number
  recvId?: number
  amount?: string
}

async function openRecord(opts?: OpenRecordOptions) {
  recordMode.value = 'quick'
  record.value = { date: todayStr(), note: '', categoryName: '', categoryId: null, amount: '', direction: 'expense' }
  quick.value = { date: todayStr(), biz: opts?.biz || '', companyId: null, partyId: null, recvId: opts?.recvId ?? null, amount: opts?.amount || '', note: '' }
  const initialBiz = quickBusinesses.find(x => x.key === quick.value.biz)
  if (initialBiz?.manualNote) quick.value.note = initialBiz.defaultNote || initialBiz.label
  showNewCompany.value = false
  newCompanyName.value = ''
  recordError.value = ''
  showRecord.value = true
  const loadP = loadPartiesQuick()
  void loadCats()
  if (opts?.partyId) {
    await loadP // 等待单位列表就绪后再回填 partyId，确保单位名能正确展示
    quick.value.partyId = opts.partyId
  }
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

defineExpose({ openRecord })
</script>

<style scoped>
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

/* 本弹窗 primary 主按钮局部调浅（不影响全局 --jade 主题） */
.mode-btn.van-button--primary,
:deep(.record-save .van-button--primary),
:deep(.record-save .van-button--primary:not(.van-button--disabled)) {
  background-color: #e5f5f4;
  border-color: #e5f5f4;
  color: #1f5c48;
}
.mode-btn.van-button--primary:hover,
:deep(.record-save .van-button--primary:hover) {
  background-color: #d3eeee;
  border-color: #d3eeee;
  color: #143d30;
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

.new-company-link {
  padding: 6px 16px 2px;
  font-size: 13px;
  color: #07c160;
  cursor: pointer;
}

.new-company-hint {
  padding: 0 16px;
  font-size: 12px;
  color: #8f8e88;
}

.new-company-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 8px 16px;
}

.desktop-field {
  display: none;
}

@media (min-width: 992px) {
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