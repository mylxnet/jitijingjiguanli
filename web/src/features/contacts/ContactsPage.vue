<template>
  <div class="contacts-page">
    <!-- 列表模式头部 -->
    <div class="page-header" v-if="!currentParty">
      <h3>往来</h3>
      <div class="header-actions">
        <van-button type="primary" plain size="small" @click="openAccrue">批量计提</van-button>
        <van-button type="primary" size="small" icon="plus" @click="openAddParty">新增单位</van-button>
      </div>
    </div>

    <!-- 详情模式头部 -->
    <div class="page-header" v-else>
      <van-icon name="arrow-left" class="back-icon" @click="closeDetail" />
      <h3 class="detail-title">{{ currentParty.name }}</h3>
      <van-button size="small" plain type="primary" @click="openAddReceivable">登记应收</van-button>
    </div>

    <!-- 列表：往来单位 -->
    <template v-if="!currentParty">
      <div v-if="loading" class="loading-state"><van-skeleton title :row="4" /></div>
      <div v-else-if="parties.length === 0" class="empty-state">
        <p>还没有往来单位</p>
        <van-button size="small" type="primary" @click="openAddParty">新增单位</van-button>
      </div>
      <div v-else class="party-list">
        <div v-for="p in parties" :key="p.id" class="party-row" @click="openDetail(p)">
          <div class="party-main">
            <span class="party-name">{{ p.name }}</span>
          </div>
          <div class="party-owed">
            <div class="owed-value" :class="{ zero: p.outstandingCents <= 0 }">
              {{ formatFen(p.outstandingCents) }}
            </div>
            <div class="owed-label">欠款</div>
          </div>
        </div>
      </div>
    </template>

    <!-- 详情：应收单 -->
    <template v-else>
      <div class="party-summary" v-if="currentParty">
        <div class="summary-item">
          <div class="summary-value" :class="{ zero: currentParty.outstandingCents <= 0 }">
            {{ formatFen(currentParty.outstandingCents) }}
          </div>
          <div class="summary-label">合计欠款</div>
        </div>
        <div class="summary-item">
          <div class="summary-value">{{ receivableItems.length }}</div>
          <div class="summary-label">应收单</div>
        </div>
        <van-button size="small" plain @click="openAddPartyEdit">编辑单位</van-button>
      </div>

      <div v-if="detailLoading" class="loading-state"><van-skeleton title :row="4" /></div>
      <div v-else-if="receivableItems.length === 0" class="empty-state">
        <p>该单位暂无应收记录</p>
        <van-button size="small" type="primary" @click="openAddReceivable">登记应收</van-button>
      </div>

      <div v-else class="recv-list">
        <div v-for="r in receivableItems" :key="r.id" class="recv-card">
          <div class="recv-head" @click="toggleReceipts(r)">
            <div class="recv-title-wrap">
              <span class="recv-title">{{ r.title }}</span>
              <span class="l2-chip" :class="r.recvKind">{{ recvKindLabel[r.recvKind] }}</span>
              <span class="l2-chip" :class="r.status">{{ r.status === 'open' ? '未结清' : '已结清' }}</span>
            </div>
            <van-icon :name="expandedReceipts[r.id] ? 'arrow-up' : 'arrow-down'" />
          </div>
          <div class="recv-amounts">
            <div class="amount-cell"><span class="amount-label">应收</span>{{ formatFen(r.amountCents) }}</div>
            <div class="amount-cell"><span class="amount-label">已收</span>{{ formatFen(r.paidCents) }}</div>
            <div class="amount-cell strong"><span class="amount-label">未收</span>{{ formatFen(r.outstandingCents) }}</div>
          </div>
          <div class="recv-actions">
            <van-button v-if="r.status === 'open'" size="mini" plain type="primary" @click="openReceipt(r)">收款 / 抵销</van-button>
          </div>

          <!-- 核销记录 -->
          <div v-if="expandedReceipts[r.id]" class="receipt-list">
            <div v-if="!receiptsByRec[r.id] || receiptsByRec[r.id].length === 0" class="receipt-empty">暂无核销记录</div>
            <div v-for="rc in receiptsByRec[r.id] || []" :key="rc.id" class="receipt-row" :class="{ voided: rc.status === 'voided' }">
              <span class="receipt-method" :class="rc.method">{{ rc.method === 'cash' ? '现金' : '抵销' }}</span>
              <span class="receipt-amount">{{ rc.method === 'cash' ? '+' : '抵' }}{{ formatFen(rc.amountCents) }}</span>
              <span class="receipt-date">{{ rc.receiptDate }}</span>
              <span v-if="rc.status === 'voided'" class="l2-chip stopped">已作废</span>
              <van-button
                v-if="rc.status === 'normal'"
                size="mini"
                plain
                type="warning"
                @click="voidReceipt(rc)"
              >作废</van-button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- 新增/编辑往来单位 -->
    <van-dialog v-model:show="showAddParty" :title="editParty ? '编辑单位' : '新增往来单位'" show-cancel-button @confirm="saveParty">
      <van-field v-model="partyForm.name" label="单位名称" placeholder="如：XX公司 / XX合作社" :rules="[{ required: true }]" />
      <van-field v-model="partyForm.note" label="备注" placeholder="备注（可选）" />
    </van-dialog>

    <!-- 批量计提 / 流转费标准 -->
    <van-popup v-model:show="showAccrue" position="bottom" round closeable style="max-height: 92vh">
      <div class="accrue-popup">
        <van-tabs v-model:active="accrueTab">
          <!-- 手动批量计提 -->
          <van-tab title="批量计提" name="batch">
            <div class="accrue-body">
              <van-field v-model="accrueYear" type="digit" label="年度" placeholder="如 2026" />
              <van-field v-model="accrueTitle" label="事由" placeholder="如：2026年度土地流转费" />
              <div class="accrue-rows">
                <div v-for="(row, idx) in accrueRows" :key="idx" class="accrue-row">
                  <van-field
                    :model-value="row.partyName || '请选择单位'"
                    is-link readonly label="单位"
                    @click="openAccrueUnit('row', idx)"
                    class="only-mobile"
                  />
                  <NativeSelect
                    label="单位"
                    placeholder="请选择单位"
                    v-model="row.partyId"
                    :options="accrueUnitActions"
                  />
                  <van-field label="类别">
                    <template #input>
                      <van-radio-group v-model="row.recvKind" direction="horizontal">
                        <van-radio name="rent">流转费</van-radio>
                        <van-radio name="dividend">收益</van-radio>
                        <van-radio name="other">其他</van-radio>
                      </van-radio-group>
                    </template>
                  </van-field>
                  <div class="accrue-row-foot">
                    <van-field v-model="row.amountYuan" type="number" label="金额" placeholder="0.00" inputmode="decimal" />
                    <van-button
                      v-if="accrueRows.length > 1" size="mini" plain type="danger"
                      @click="accrueRows.splice(idx, 1)"
                    >删除</van-button>
                  </div>
                </div>
              </div>
              <div class="accrue-add">
                <van-button size="small" plain type="primary" @click="addAccrueRow">+ 添加一行</van-button>
              </div>
              <div class="accrue-save">
                <van-button round block type="primary" :loading="savingAccrue" @click="saveBatch">保存批量计提</van-button>
              </div>
            </div>
          </van-tab>

          <!-- 流转费标准 + 一键结转 -->
          <van-tab title="流转费标准" name="std">
            <div class="accrue-body">
              <div class="std-form">
                <van-field
                  :model-value="stdForm.partyName || '请选择单位'"
                  is-link readonly label="单位"
                  @click="openAccrueUnit('std', 0)"
                  class="only-mobile"
                />
                <NativeSelect
                  label="单位"
                  placeholder="请选择单位"
                  v-model="stdForm.partyId"
                  :options="accrueUnitActions"
                />
                <van-field v-model="stdForm.amountYuan" type="number" label="年度流转费" placeholder="0.00" inputmode="decimal" />
                <van-button size="small" round block type="primary" :loading="savingStd" @click="saveStandard">保存标准</van-button>
              </div>
              <div class="std-list">
                <div v-for="s in standards" :key="s.id" class="std-row">
                  <span class="std-name">{{ s.partyName }}</span>
                  <span class="std-amount">{{ formatFen(s.amountCents) }}</span>
                  <van-switch :model-value="s.active" size="20" @update:model-value="(v:boolean) => toggleStandard(s, v)" />
                </div>
                <div v-if="standards.length === 0" class="accrue-empty">还没有流转费标准，先在上方添加</div>
              </div>
              <div class="accrue-save">
                <van-button round block type="primary" :loading="accruing" @click="accrueNow">
                  一键结转 {{ accrueYear }} 年度流转费
                </van-button>
              </div>
            </div>
          </van-tab>
        </van-tabs>
      </div>
    </van-popup>

    <!-- 批量计提/标准：单位选择 -->
    <van-action-sheet
      v-model:show="showAccrueUnitPicker"
      title="选择单位"
      :actions="accrueUnitActions"
      @select="onAccrueUnitSelect"
      @cancel="showAccrueUnitPicker = false"
    />

    <!-- 登记应收 -->
    <van-dialog v-model:show="showAddReceivable" title="登记应收" show-cancel-button @confirm="saveReceivable">
      <van-field v-if="currentParty" :model-value="currentParty.name" label="单位" readonly />
      <van-field label="类别">
        <template #input>
          <van-radio-group v-model="recvForm.recvKind" direction="horizontal">
            <van-radio name="rent">流转费</van-radio>
            <van-radio name="dividend">投资收益</van-radio>
            <van-radio name="other">其他</van-radio>
          </van-radio-group>
        </template>
      </van-field>
      <van-field v-model="recvForm.title" label="事由" placeholder="如：2026年度土地流转费" :rules="[{ required: true }]" />
      <van-field v-model="recvForm.amount" label="应收金额" type="number" placeholder="0.00" inputmode="decimal" />
      <van-field
        :model-value="recvForm.incomeCategoryName || '（收款时再指定，可不选）'"
        is-link
        readonly
        label="收款入账科目"
        @click="openIncomeCatPicker('recv')"
        class="only-mobile"
      />
      <NativeSelect
        label="收款入账科目"
        placeholder="不预设（收款时再指定）"
        :model-value="recvForm.incomeCategoryId"
        :options="incomeCatOptions"
        @update:model-value="(v:number|string|null) => recvForm.incomeCategoryId = (v == null ? null : Number(v))"
      />
      <van-field v-model="recvForm.note" label="备注" placeholder="备注（可选）" />
    </van-dialog>

    <!-- 收款 / 抵销 -->
    <van-popup v-model:show="showReceipt" position="bottom" round closeable style="max-height: 92vh">
      <div class="receipt-popup">
        <div class="popup-title">收款 / 抵销</div>
        <van-field label="应收单">
          <template #input>
            <div class="static-text">
              {{ currentReceivable?.title }}（未收 {{ currentReceivable ? formatFen(currentReceivable.outstandingCents) : '' }}）
            </div>
          </template>
        </van-field>
        <van-field label="方式">
          <template #input>
            <van-radio-group v-model="receiptForm.method" direction="horizontal">
              <van-radio name="cash">现金入账</van-radio>
              <van-radio name="offset">抵销</van-radio>
            </van-radio-group>
          </template>
        </van-field>
        <van-field v-model="receiptForm.amount" label="金额" type="number" placeholder="0.00" inputmode="decimal" />
        <van-field v-model="receiptForm.date" label="日期" placeholder="YYYY-MM-DD" />
        <van-field v-if="receiptForm.method === 'offset'" class="only-mobile">
          <template #label>抵销流水</template>
          <template #input>
            <div class="static-text" v-if="receiptForm.txnLabel">{{ receiptForm.txnLabel }}</div>
            <van-button v-else size="mini" type="primary" plain @click="loadOffsetTxns">选择分红支出流水</van-button>
          </template>
        </van-field>
        <NativeSelect
          v-if="receiptForm.method === 'offset'"
          label="抵销流水"
          placeholder="选择分红支出流水"
          :model-value="receiptForm.txnId"
          :options="offsetTxnOptions"
          @update:model-value="(v:number|string|null) => { receiptForm.txnId = (v == null ? null : Number(v)); const o=offsetTxnOptions.find(x=>x.value===Number(v)); receiptForm.txnLabel = o ? o.name : ''; }"
        />
        <div v-if="receiptForm.method === 'cash'" class="dialog-tip">
          现金收款将自动记一笔银行收入
          <span v-if="cashCategoryResolved">{{ cashCategoryResolved }}</span>
          <template v-else>（需要选择入账科目）</template>
        </div>
        <van-field
          v-if="receiptForm.method === 'cash' && !cashCategoryPreset"
          :model-value="receiptForm.categoryName || '请选择入账科目'"
          is-link
          readonly
          label="入账科目"
          @click="openIncomeCatPicker('receipt')"
          class="only-mobile"
        />
        <NativeSelect
          v-if="receiptForm.method === 'cash' && !cashCategoryPreset"
          label="入账科目"
          placeholder="请选择入账科目"
          :model-value="receiptForm.categoryId"
          :options="incomeCatOptions"
          @update:model-value="(v:number|string|null) => { receiptForm.categoryId = (v == null ? null : Number(v)); const o=incomeCatOptions.find(x=>x.value===Number(v)); receiptForm.categoryName = o ? o.name : ''; }"
        />
        <van-field v-model="receiptForm.note" label="备注" placeholder="备注（可选）" />
        <div class="receipt-save">
          <van-button round block type="primary" :disabled="!canSubmitReceipt" :loading="savingReceipt" @click="saveReceipt">
            保存核销
          </van-button>
        </div>
      </div>
    </van-popup>

    <!-- 入账科目选择器 -->
    <van-action-sheet
      v-model:show="showIncomeCatPicker"
      title="选择入账科目"
      :actions="incomeCatOptions"
      @select="onIncomeCatSelect"
      @cancel="showIncomeCatPicker = false"
    />
    <!-- 抵销流水选择器 -->
    <van-action-sheet
      v-model:show="showTxnPicker"
      title="选择分红支出流水"
      :actions="offsetTxnOptions"
      @select="onTxnSelect"
      @cancel="showTxnPicker = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../lib/http'
import { showToast, showDialog } from 'vant'
import type {
  Category, Party, Receivable, Receipt, ReceivableDetail,
  ReceivableListResponse, Transaction, ApiResponse, RecvKind, AccrualStandard, AccrueResult,
} from '../../types/api'
import { formatFen, todayStr, recvKindLabel } from '../../types/api'
import NativeSelect from '../../components/NativeSelect.vue'

const loading = ref(false)
const detailLoading = ref(false)
const parties = ref<Party[]>([])

// ---- 往来单位 ----
const currentParty = ref<Party | null>(null)
const receivableItems = ref<Receivable[]>([])
const receiptsByRec = ref<Record<number, Receipt[]>>({})
const expandedReceipts = ref<Record<number, boolean>>({})

const showAddParty = ref(false)
const editParty = ref(false)
const partyForm = ref({ id: 0, name: '', note: '' })

const showAddReceivable = ref(false)
const recvForm = ref({
  recvKind: 'rent' as RecvKind,
  title: '',
  amount: '',
  incomeCategoryId: null as number | null,
  incomeCategoryName: '',
  note: '',
})

// ---- 收款/抵销 ----
const showReceipt = ref(false)
const currentReceivable = ref<Receivable | null>(null)
const receiptForm = ref({
  method: 'cash' as 'cash' | 'offset',
  amount: '',
  date: todayStr(),
  categoryId: null as number | null,
  categoryName: '',
  txnId: null as number | null,
  txnLabel: '',
  note: '',
})
const savingReceipt = ref(false)

const cats = ref<Category[]>([])
const incomeCatOptions = ref<{ name: string; value: number }[]>([])
const showIncomeCatPicker = ref(false)
let incomeCatPickerFor = 'receipt' as 'recv' | 'receipt'

const showTxnPicker = ref(false)
const offsetTxnOptions = ref<{ name: string; value: number }[]>([])
const catNameById = computed<Record<number, string>>(() => {
  const m: Record<number, string> = {}
  for (const l1 of cats.value) {
    if (l1.children) {
      for (const l2 of l1.children) m[l2.id] = `${l1.name} / ${l2.name}`
    }
  }
  return m
})

const cashCategoryPreset = computed(() => {
  return currentReceivable.value?.incomeCategoryId != null
})
const cashCategoryResolved = computed(() => {
  if (receiptForm.value.method !== 'cash') return ''
  const id = cashCategoryPreset.value ? currentReceivable.value?.incomeCategoryId : receiptForm.value.categoryId
  if (id == null) return ''
  return '（入账科目：' + (catNameById.value[id] || `#${id}`) + '）'
})

onMounted(async () => {
  await Promise.all([loadParties(), loadCategories()])
})

// ---- 单位列表 ----
async function loadParties() {
  loading.value = true
  try {
    const res = await api.get<ApiResponse<Party[]>>('/parties')
    parties.value = res.data
  } catch {
    parties.value = []
  } finally {
    loading.value = false
  }
}

function openAddParty() {
  editParty.value = false
  partyForm.value = { id: 0, name: '', note: '' }
  showAddParty.value = true
}

function openAddPartyEdit() {
  if (!currentParty.value) return
  editParty.value = true
  partyForm.value = {
    id: currentParty.value.id,
    name: currentParty.value.name,
    note: currentParty.value.note || '',
  }
  showAddParty.value = true
}

async function saveParty() {
  if (!partyForm.value.name) {
    showToast('请填写单位名称')
    return
  }
  try {
    const payload: Record<string, unknown> = { name: partyForm.value.name }
    if (partyForm.value.note) payload.note = partyForm.value.note
    if (editParty.value) {
      await api.put(`/parties/${partyForm.value.id}`, payload)
      showToast('更新成功')
    } else {
      await api.post('/parties', payload)
      showToast('创建成功')
    }
    await loadParties()
    if (currentParty.value) {
      const updated = parties.value.find(p => p.id === currentParty.value!.id)
      if (updated) currentParty.value = updated
    }
  } catch (e: any) {
    showDialog({ title: '保存失败', message: e.message || '保存失败，请重试' })
  }
}

// ---- 详情 ----
async function openDetail(p: Party) {
  currentParty.value = p
  detailLoading.value = true
  receivableItems.value = []
  receiptsByRec.value = {}
  expandedReceipts.value = {}
  try {
    const res = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', { partyId: p.id, pageSize: 200 })
    receivableItems.value = res.data.items || []
    for (const r of receivableItems.value) {
      if (r.status === 'open' || r.paidCents > 0) {
        await loadReceipts(r)
      }
    }
    // 刷新对象欠款合计（应收单核销会改变欠款）
    await loadParties()
    const fresh = parties.value.find(x => x.id === p.id)
    if (fresh) currentParty.value = fresh
  } catch {
    receivableItems.value = []
  } finally {
    detailLoading.value = false
  }
}

function closeDetail() {
  currentParty.value = null
}

async function loadReceipts(r: Receivable) {
  try {
    const res = await api.get<ApiResponse<ReceivableDetail>>(`/receivables/${r.id}`)
    receiptsByRec.value[r.id] = res.data.receipts || []
  } catch {
    receiptsByRec.value[r.id] = []
  }
}

function toggleReceipts(r: Receivable) {
  expandedReceipts.value[r.id] = !expandedReceipts.value[r.id]
}

// ---- 登记应收 ----
function openAddReceivable() {
  recvForm.value = { recvKind: 'rent', title: '', amount: '', incomeCategoryId: null, incomeCategoryName: '', note: '' }
  showAddReceivable.value = true
}

async function saveReceivable() {
  if (!currentParty.value) return
  const amountCents = parseFen(recvForm.value.amount)
  if (amountCents <= 0) {
    showToast('请填写正确的金额')
    return
  }
  if (!recvForm.value.title) {
    showToast('请填写事由')
    return
  }
  try {
    const payload: Record<string, unknown> = {
      partyId: currentParty.value.id,
      recvKind: recvForm.value.recvKind,
      title: recvForm.value.title,
      amountCents,
    }
    if (recvForm.value.incomeCategoryId) payload.incomeCategoryId = recvForm.value.incomeCategoryId
    if (recvForm.value.note) payload.note = recvForm.value.note
    await api.post('/receivables', payload)
    showToast('登记成功')
    showAddReceivable.value = false
    await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '登记失败', message: e.message || '登记失败，请重试' })
  }
}

// ---- 收款 / 抵销 ----
function openReceipt(r: Receivable) {
  currentReceivable.value = r
  receiptForm.value = {
    method: 'cash',
    amount: String((r.outstandingCents / 100).toFixed(2)),
    date: todayStr(),
    categoryId: null,
    categoryName: '',
    txnId: null,
    txnLabel: '',
    note: '',
  }
  showReceipt.value = true
  void fetchOffsetOptions()
}

const canSubmitReceipt = computed(() => {
  const amount = parseFen(receiptForm.value.amount)
  if (amount <= 0) return false
  if (receiptForm.value.method === 'offset' && !receiptForm.value.txnId) return false
  if (receiptForm.value.method === 'cash' && !cashCategoryPreset.value && !receiptForm.value.categoryId) return false
  return true
})

async function saveReceipt() {
  const amountCents = parseFen(receiptForm.value.amount)
  if (!currentReceivable.value || amountCents <= 0) {
    showToast('请填写正确的金额')
    return
  }
  savingReceipt.value = true
  try {
    const payload: Record<string, unknown> = {
      amountCents,
      receiptDate: receiptForm.value.date,
      method: receiptForm.value.method,
    }
    if (receiptForm.value.method === 'cash' && !cashCategoryPreset.value) {
      payload.categoryId = receiptForm.value.categoryId
    }
    if (receiptForm.value.method === 'offset') {
      payload.txnId = receiptForm.value.txnId
    }
    if (receiptForm.value.note) payload.note = receiptForm.value.note
    await api.post(`/receivables/${currentReceivable.value.id}/receipts`, payload)
    showToast('核销成功')
    showReceipt.value = false
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '核销失败', message: e.message || '核销失败，请重试' })
  } finally {
    savingReceipt.value = false
  }
}

// 预载发放支出流水选项（桌面下拉用；不主动弹层）
async function fetchOffsetOptions() {
  offsetTxnOptions.value = []
  try {
    const res = await api.get<ApiResponse<{ items: Transaction[] }>>('/transactions', { pageSize: 50 })
    const options: { name: string; value: number }[] = []
    for (const t of res.data.items || []) {
      if (t.direction === 'expense' && t.status === 'normal') {
        const cat = catNameById.value[t.categoryId] || `#${t.categoryId}`
        const note = t.note ? ` ${t.note}` : ''
        options.push({ name: `${t.txnDate} ${cat}${note} ${formatFen(t.amountCents)}`, value: t.id })
      }
    }
    offsetTxnOptions.value = options
  } catch {
    offsetTxnOptions.value = []
  }
}

// 抵销需要选择发放支出流水（移动端弹 action sheet）
async function loadOffsetTxns() {
  await fetchOffsetOptions()
  showTxnPicker.value = true
  if (offsetTxnOptions.value.length === 0) showToast('近期没有可抵销的支出流水（如分红发放）')
}

function onTxnSelect(action: { name: string; value: number }) {
  receiptForm.value.txnId = action.value
  receiptForm.value.txnLabel = action.name
  showTxnPicker.value = false
}

async function voidReceipt(rc: Receipt) {
  try {
    await showDialog({
      title: '确认作废',
      message: `作废这笔${rc.method === 'cash' ? '现金核销' : '抵销'}（${formatFen(rc.amountCents)}）？若为现金核销，对应的银行收入流水将同步作废。`,
      showCancelButton: true,
    })
  } catch {
    return
  }
  try {
    await api.put(`/receipts/${rc.id}`, { status: 'voided' })
    showToast('已作废')
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showToast(e.message || '操作失败')
  }
}

// ---- 批量计提 / 流转费标准 ----
const showAccrue = ref(false)
const accrueTab = ref('batch')
const accrueYear = ref(String(new Date().getFullYear()))
const accrueTitle = ref('')
const savingAccrue = ref(false)
const accrueRows = ref<{ partyId: number | null; partyName: string; recvKind: RecvKind; amountYuan: string }[]>([
  { partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' },
])

const showAccrueUnitPicker = ref(false)
let accruePickFor = 'row' as 'row' | 'std'
let accruePickRow = 0

const accrueUnitActions = computed(() =>
  parties.value.map(p => ({ name: p.name, value: p.id })),
)

const standards = ref<AccrualStandard[]>([])
const stdForm = ref<{ partyId: number | null; partyName: string; amountYuan: string }>({ partyId: null, partyName: '', amountYuan: '' })
const savingStd = ref(false)
const accruing = ref(false)

async function openAccrue() {
  accrueTab.value = 'batch'
  accrueYear.value = String(new Date().getFullYear())
  accrueTitle.value = ''
  accrueRows.value = [{ partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' }]
  stdForm.value = { partyId: null, partyName: '', amountYuan: '' }
  showAccrue.value = true
  await loadStandards()
  await loadParties()
}

function addAccrueRow() {
  accrueRows.value.push({ partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' })
}

function openAccrueUnit(kind: 'row' | 'std', rowIndex: number) {
  accruePickFor = kind
  accruePickRow = rowIndex
  showAccrueUnitPicker.value = true
}

function onAccrueUnitSelect(action: { name: string; value: number }) {
  if (accruePickFor === 'row') {
    const row = accrueRows.value[accruePickRow]
    if (row) {
      row.partyId = action.value
      row.partyName = action.name
    }
  } else {
    stdForm.value.partyId = action.value
    stdForm.value.partyName = action.name
  }
  showAccrueUnitPicker.value = false
}

async function saveBatch() {
  const year = parseInt(accrueYear.value || '0', 10)
  if (!year || year < 2000 || year > 2100) {
    showToast('请填写正确年度')
    return
  }
  if (!accrueTitle.value.trim()) {
    showToast('请填写事由')
    return
  }
  const items: { partyId: number; recvKind: RecvKind; amountCents: number }[] = []
  for (const row of accrueRows.value) {
    const amount = parseFen(row.amountYuan)
    if (!row.partyId || amount <= 0) {
      showToast('请补全各单位与金额')
      return
    }
    items.push({ partyId: row.partyId, recvKind: row.recvKind, amountCents: amount })
  }
  savingAccrue.value = true
  try {
    const res = await api.post<ApiResponse<AccrueResult>>('/receivables/batch', {
      recvYear: year,
      title: accrueTitle.value.trim(),
      items,
    })
    showToast(`新增 ${res.data.created} 条${res.data.skipped ? `，跳过 ${res.data.skipped} 条` : ''}`)
    showAccrue.value = false
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '计提失败', message: e.message || '批量计提失败' })
  } finally {
    savingAccrue.value = false
  }
}

async function loadStandards() {
  try {
    const res = await api.get<ApiResponse<AccrualStandard[]>>('/recv-standards', { kind: 'rent' })
    standards.value = res.data || []
  } catch {
    standards.value = []
  }
}

async function saveStandard() {
  const amount = parseFen(stdForm.value.amountYuan)
  if (!stdForm.value.partyId || amount <= 0) {
    showToast('请选择单位并填写金额')
    return
  }
  savingStd.value = true
  try {
    await api.post('/recv-standards', {
      partyId: stdForm.value.partyId,
      recvKind: 'rent',
      amountCents: amount,
    })
    showToast('已保存')
    stdForm.value = { partyId: null, partyName: '', amountYuan: '' }
    await loadStandards()
  } catch (e: any) {
    showToast(e.message || '保存失败')
  } finally {
    savingStd.value = false
  }
}

async function toggleStandard(s: AccrualStandard, v: boolean) {
  try {
    await api.put(`/recv-standards/${s.id}`, { active: v })
    s.active = v
  } catch (e: any) {
    showToast(e.message || '操作失败')
  }
}

async function accrueNow() {
  const year = parseInt(accrueYear.value || '0', 10)
  if (!year) {
    showToast('请填写年度')
    return
  }
  accruing.value = true
  try {
    const res = await api.post<ApiResponse<AccrueResult>>('/recv-standards/accrue', {
      year,
      kind: 'rent',
      title: `${year}年度土地流转费`,
    })
    showToast(`结转完成：新增 ${res.data.created} 条${res.data.skipped ? `，跳过 ${res.data.skipped} 条` : ''}`)
    await loadParties()
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showToast(e.message || '结转失败')
  } finally {
    accruing.value = false
  }
}

// ---- 科目与选择器 ----
async function loadCategories() {
  try {
    const res = await api.get<ApiResponse<Category[]>>('/categories')
    cats.value = res.data
    const options: { name: string; value: number }[] = []
    for (const l1 of res.data) {
      if (l1.children) {
        for (const l2 of l1.children) {
          if (l2.status === 'active' && l2.kind === 'equity') {
            options.push({ name: `${l1.name} / ${l2.name}`, value: l2.id })
          }
        }
      }
    }
    incomeCatOptions.value = options
  } catch {
    cats.value = []
  }
}

function openIncomeCatPicker(forWhat: 'recv' | 'receipt') {
  incomeCatPickerFor = forWhat
  showIncomeCatPicker.value = true
}

function onIncomeCatSelect(action: { name: string; value: number }) {
  if (incomeCatPickerFor === 'recv') {
    recvForm.value.incomeCategoryId = action.value
    recvForm.value.incomeCategoryName = action.name
  } else {
    receiptForm.value.categoryId = action.value
    receiptForm.value.categoryName = action.name
  }
  showIncomeCatPicker.value = false
}

function parseFen(yuan: string): number {
  const cleaned = yuan.replace(/[^0-9.]/g, '')
  const num = parseFloat(cleaned)
  if (isNaN(num)) return 0
  return Math.round(num * 100)
}
</script>

<style scoped>
.contacts-page {
  padding: 16px;
  padding-bottom: 60px;
  min-height: 100vh;
  background: #f7f7f5;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.page-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: #2c2c2a;
  margin: 0;
}

.detail-title {
  font-size: 16px;
  font-weight: 500;
  flex: 1;
  text-align: center;
}

.back-icon {
  font-size: 18px;
  color: #5f5e5a;
  cursor: pointer;
}

.header-actions {
  display: flex;
  gap: 4px;
}

.kind-tabs {
  margin-bottom: 8px;
}

.loading-state {
  padding: 16px;
  background: #fff;
  border-radius: 12px;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  background: #fff;
  border-radius: 12px;
  color: #8f8e88;
}

.empty-state p {
  margin-bottom: 16px;
}

.party-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.party-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-radius: 12px;
  padding: 14px 16px;
  cursor: pointer;
}

.party-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.party-name {
  font-size: 15px;
  font-weight: 500;
}

.owed-value {
  font-size: 16px;
  font-weight: 600;
  color: #a32d2d;
  font-variant-numeric: tabular-nums;
}

.owed-value.zero {
  color: #0f6e56;
}

.owed-label {
  font-size: 11px;
  color: #8f8e88;
  text-align: right;
}

.party-summary {
  display: flex;
  align-items: center;
  gap: 24px;
  background: #fff;
  border-radius: 12px;
  padding: 14px 16px;
  margin-bottom: 12px;
}

.summary-value {
  font-size: 18px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.summary-value.zero {
  color: #0f6e56;
}

.summary-label {
  font-size: 11px;
  color: #8f8e88;
}

.recv-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.recv-card {
  background: #fff;
  border-radius: 12px;
  padding: 12px 16px;
}

.recv-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
}

.recv-title-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.recv-title {
  font-size: 14px;
  font-weight: 500;
}

.recv-amounts {
  display: flex;
  gap: 24px;
  padding: 8px 0;
}

.amount-cell {
  font-size: 14px;
  font-variant-numeric: tabular-nums;
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.amount-cell.strong {
  font-weight: 600;
  color: #a32d2d;
}

.amount-label {
  font-size: 11px;
  color: #8f8e88;
}

.recv-actions {
  display: flex;
  gap: 4px;
  justify-content: flex-end;
}

.receipt-list {
  border-top: 1px solid #f0f0eb;
  margin-top: 8px;
  padding-top: 4px;
}

.receipt-empty {
  text-align: center;
  color: #8f8e88;
  font-size: 12px;
  padding: 8px 0;
}

.receipt-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
}

.receipt-row.voided {
  opacity: 0.6;
}

.receipt-method {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #0f6e56;
  color: #0f6e56;
}

.receipt-method.offset {
  border-color: #185fa5;
  color: #185fa5;
}

.receipt-amount {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.receipt-date {
  font-size: 12px;
  color: #8f8e88;
}

.l2-chip {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #e3e2dd;
  color: #8f8e88;
}

.l2-chip.household { border-color: #0f6e56; color: #0f6e56; }
.l2-chip.unit { border-color: #185fa5; color: #185fa5; }
.l2-chip.rent { border-color: #7a4f0f; color: #7a4f0f; background: #fdf3e3; }
.l2-chip.dividend { border-color: #0f6e56; color: #0f6e56; background: #eaf5ed; }
.l2-chip.other { border-color: #8f8e88; color: #5f5e5a; }
.l2-chip.open { border-color: #e88a3a; color: #a8601a; background: #fef3e8; }
.l2-chip.closed { border-color: #0f6e56; color: #0f6e56; background: #eaf5ed; }
.l2-chip.stopped { background: #fcebeb; border-color: #a32d2d; color: #a32d2d; }

.receipt-popup {
  padding: 16px 0 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.popup-title {
  font-size: 16px;
  font-weight: 500;
  color: #2c2c2a;
  padding: 0 16px 12px;
}

.static-text {
  font-size: 13px;
  color: #2c2c2a;
}

.dialog-tip {
  font-size: 12px;
  color: #8f8e88;
  padding: 0 16px 8px;
  line-height: 1.5;
}

.receipt-save {
  margin: 8px 16px 0;
}

.accrue-popup {
  padding-bottom: 24px;
}

.accrue-body {
  padding: 8px 16px;
}

.accrue-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.accrue-row {
  background: #f7f7f5;
  border-radius: 8px;
  padding: 4px 8px;
}

.accrue-row-foot {
  display: flex;
  align-items: center;
  gap: 8px;
}

.accrue-add {
  padding: 8px 0;
}

.accrue-save {
  margin-top: 8px;
}

.accrue-empty {
  text-align: center;
  color: #8f8e88;
  font-size: 13px;
  padding: 16px 0;
}

.std-form {
  padding-bottom: 4px;
}

.std-list {
  margin-top: 8px;
  border-top: 1px solid #f0f0eb;
}

.std-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  border-bottom: 1px solid #f0f0eb;
}

.std-name {
  font-size: 14px;
}

.std-amount {
  font-size: 13px;
  color: #5f5e5a;
  font-variant-numeric: tabular-nums;
}
</style>
