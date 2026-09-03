<template>
  <div class="list-page">
    <div class="list-header">
      <h3>流水</h3>
      <div class="header-actions">
          <div class="type-filter">
            <van-button
              v-for="t in typeOptions"
              :key="t.value"
              :type="filterType === t.value ? 'primary' : 'default'"
              size="mini"
              plain
              @click="filterType = t.value; loadData()"
            >{{ t.label }}</van-button>
          </div>
          <van-button icon="filter" size="mini" @click="showFilterDialog = true">筛选</van-button>
          <van-button icon="export" size="mini" type="primary" @click="showExportPicker = true">导出</van-button>
        </div>
    </div>

    <!-- 筛选条件提示 -->
    <div v-if="hasActiveFilters" class="filter-hint">
      <span>已筛选：{{ activeFilterText }}</span>
      <van-button size="mini" plain type="danger" @click="clearFilters">清空</van-button>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="loading-state">
      <van-skeleton title :row="5" />
    </div>

    <!-- 空数据 -->
    <div v-else-if="mergedItems.length === 0" class="empty-state">
      <p>还没有流水</p>
      <van-button type="primary" size="small" @click="goHome">去记一笔</van-button>
    </div>

    <!-- 列表 -->
    <div v-else class="list-content">
      <div v-for="item in mergedItems" :key="item._key" class="txn-row" :class="{ voided: item.status === 'voided' }" @click="openEdit(item)">
        <!-- 收支行 -->
        <template v-if="item._type === 'transaction'">
          <div class="txn-main">
            <div class="txn-date">{{ formatDate(item.txnDate) }}</div>
            <div class="txn-category">{{ categoryName(item.categoryId) }}</div>
            <div class="txn-note" v-if="item.note">{{ item.note }}</div>
          </div>
          <div class="txn-amount" :class="item.direction === 'income' ? 'income' : 'expense'">
            {{ item.direction === 'income' ? '+' : '-' }}{{ formatFen(item.amountCents ?? 0) }}
          </div>
        </template>
        <!-- 转账行 -->
        <template v-else>
          <div class="txn-main">
            <div class="txn-date">{{ formatDate(item.txnDate) }}</div>
            <div class="txn-category">
              <span class="transfer-tag">转账</span>
              转出 {{ categoryName(item.sourceCategoryId) }}
              <span class="transfer-arrow">→</span>
              <span v-for="(leg, i) in item.legs" :key="leg.id">
                {{ i > 0 ? '、' : '' }}{{ categoryName(leg.categoryId) }}
              </span>
            </div>
            <div class="txn-note" v-if="item.note">{{ item.note }}</div>
          </div>
          <div class="txn-amount transfer">
            {{ formatFen(item.sourceAmountCents ?? 0) }}
          </div>
        </template>
      </div>
    </div>

    <!-- 底部摘要 -->
    <div v-if="summary" class="list-summary">
      <span>收 {{ formatFen(summary.incomeTotal) }}</span>
      <span>支 {{ formatFen(summary.expenseTotal) }}</span>
      <span>结余 {{ formatFen(summary.balance) }}</span>
    </div>

    <!-- 筛选弹窗 -->
    <van-popup v-model:show="showFilterDialog" position="bottom" round closeable style="max-height: 80vh">
      <div class="filter-popup">
        <div class="filter-title">筛选条件</div>

        <van-cell-group inset>
          <!-- 日期范围 -->
          <van-field
            :model-value="filters.dateFrom ? `${filters.dateFrom} ~ ${filters.dateTo}` : ''"
            readonly
            label="日期范围"
            is-link
            placeholder="选择日期范围"
            @click="showDatePicker = true"
          />
          <van-calendar
            v-model:show="showDatePicker"
            v-model:range="filters.dateRange"
            type="range"
            @confirm="onDateConfirm"
          />

          <!-- 关键词 -->
          <van-field
            v-model="filters.keyword"
            label="关键词"
            placeholder="搜索摘要"
          />

          <!-- 科目选择 -->
          <van-field
            v-model="filters.categoryName"
            readonly
            is-link
            label="科目"
            placeholder="选择科目"
            @click="showCategoryFilterPicker = true"
          />
          <van-popup v-model:show="showCategoryFilterPicker" position="bottom">
            <van-picker
              :columns="categoryFilterOptions"
              @confirm="onCategoryFilterConfirm"
              @cancel="showCategoryFilterPicker = false"
            />
          </van-popup>

          <!-- 金额范围 -->
          <div class="amount-range">
            <van-field
              v-model="filters.minAmount"
              label="最小金额"
              type="number"
              placeholder="0.00"
            />
            <span class="amount-sep">-</span>
            <van-field
              v-model="filters.maxAmount"
              label="最大金额"
              type="number"
              placeholder="不限"
            />
          </div>
        </van-cell-group>

        <div class="filter-actions">
          <van-button plain @click="resetFilters">重置</van-button>
          <van-button type="primary" @click="applyFilters">确定</van-button>
        </div>
      </div>
    </van-popup>

    <!-- 导出格式选择 -->
    <van-action-sheet v-model:show="showExportPicker" :actions="exportActions" @select="onExportSelect" cancel-text="取消" />

    <!-- 编辑/详情弹窗 -->
    <van-popup v-model:show="showEditDialog" position="bottom" round closeable style="max-height: 90vh">
      <div class="edit-popup">
        <div class="edit-title">{{ editTransfer ? '转账详情' : '编辑流水' }}</div>

        <!-- 转账详情（只读） -->
        <template v-if="editTransfer">
          <van-cell-group inset>
            <van-cell title="日期" :value="editTransfer.txnDate" />
            <van-cell title="类型" value="转账" />
            <van-cell title="转出科目" :value="categoryName(editTransfer.sourceCategoryId)" />
            <van-cell title="转出金额" :value="formatFen(editTransfer.sourceAmountCents ?? 0)" />
            <van-cell v-for="leg in editTransfer.legs" :key="leg.id"
              :title="'转入: ' + categoryName(leg.categoryId)"
              :value="formatFen(leg.amountCents)" />
            <van-cell title="摘要" :value="editTransfer.note || '-'" />
            <van-cell title="状态" :value="editTransfer.status === 'normal' ? '正常' : '已作废'" />
          </van-cell-group>
          <div class="edit-actions">
            <van-button
              v-if="editTransfer.status === 'normal'"
              type="danger"
              plain
              round
              block
              @click="handleVoidTransfer"
            >作废此转账</van-button>
            <van-button
              v-else
              type="primary"
              plain
              round
              block
              @click="handleUnvoidTransfer"
            >撤销作废</van-button>
          </div>
          <!-- 变更历史 -->
          <div class="changelog-section" v-if="editTransferChangelog.length > 0">
            <div class="section-title">变更历史</div>
            <div v-for="log in editTransferChangelog" :key="log.id" class="changelog-item">
              <span class="changelog-action">{{ changelogActionText(log.action) }}</span>
              <span class="changelog-time">{{ log.changedAt }}</span>
            </div>
          </div>
        </template>

        <!-- 编辑流水 -->
        <template v-else>
          <van-form @submit="handleSaveEdit">
            <van-field
              v-model="editForm.date"
              label="日期"
              placeholder="YYYY-MM-DD"
              :rules="[{ required: true, message: '请填写日期' }]"
            />
            <van-field
              v-model="editForm.note"
              label="摘要"
              placeholder="买了什么、给了谁（可选）"
            />
            <van-field
              v-model="editForm.categoryName"
              is-link
              readonly
              label="科目"
              placeholder="请选择科目"
              :rules="[{ required: true, message: '请选择科目' }]"
              @click="showEditCategoryPicker = true"
            />
            <van-popup v-model:show="showEditCategoryPicker" position="bottom">
              <van-picker
                :columns="categoryOptions"
                @confirm="onEditCategoryConfirm"
                @cancel="showEditCategoryPicker = false"
              />
            </van-popup>
            <van-field
              v-model="editForm.amount"
              label="金额"
              type="number"
              placeholder="0.00"
              :rules="[
                { required: true, message: '请填写金额' },
                { validator: editValidateAmount, message: '金额必须大于 0' }
              ]"
            />
            <div class="edit-direction-toggle">
              <van-button
                :type="editForm.direction === 'income' ? 'primary' : 'default'"
                size="small"
                @click="editForm.direction = 'income'"
              >收入</van-button>
              <van-button
                :type="editForm.direction === 'expense' ? 'primary' : 'default'"
                size="small"
                @click="editForm.direction = 'expense'"
              >支出</van-button>
            </div>
            <div style="margin: 16px">
              <van-button round block type="primary" native-type="submit" :loading="savingEdit">
                保存修改
              </van-button>
            </div>
          </van-form>
          <div class="edit-actions">
            <van-button
              v-if="editTransaction && editTransaction.status === 'normal'"
              type="danger"
              plain
              round
              block
              @click="handleVoidTransaction"
            >作废此流水</van-button>
            <van-button
              v-else-if="editTransaction && editTransaction.status === 'voided'"
              type="primary"
              plain
              round
              block
              @click="handleUnvoidTransaction"
            >撤销作废</van-button>
          </div>
          <!-- 变更历史 -->
          <div class="changelog-section" v-if="editChangelog.length > 0">
            <div class="section-title">变更历史</div>
            <div v-for="log in editChangelog" :key="log.id" class="changelog-item">
              <span class="changelog-action">{{ changelogActionText(log.action) }}</span>
              <span v-if="log.field" class="changelog-field">{{ changelogFieldText(log.field) }}</span>
              <span v-if="log.oldValue != null && log.newValue != null" class="changelog-diff">
                {{ formatChangelogValue(log.field, log.oldValue) }} → {{ formatChangelogValue(log.field, log.newValue) }}
              </span>
              <span class="changelog-time">{{ log.changedAt }}</span>
            </div>
          </div>
        </template>
      </div>
    </van-popup>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import { formatFen, formatDate } from '../../types/api'
import { showToast } from 'vant'
import * as XLSX from 'xlsx'
import type { Transaction, ApiResponse, Category } from '../../types/api'

interface TransferLeg {
  id: number
  transferId: number
  categoryId: number
  amountCents: number
}

interface TransferItem {
  id: number
  txnDate: string
  sourceCategoryId: number
  sourceAmountCents: number
  note: string | null
  status: string
  legs: TransferLeg[]
}

interface ChangeLogItem {
  id: number
  entityType: string
  entityId: number
  action: string
  field: string | null
  oldValue: string | null
  newValue: string | null
  changedAt: string
}

// 合并后的行
interface MergedRow {
  _key: string
  _type: 'transaction' | 'transfer'
  txnDate: string
  status: string
  // 收支字段
  direction?: string
  amountCents?: number
  categoryId?: number
  note?: string | null
  // 转账字段
  sourceCategoryId?: number
  sourceAmountCents?: number
  legs?: TransferLeg[]
}

const router = useRouter()
const loading = ref(true)
const transactions = ref<Transaction[]>([])
const transfers = ref<TransferItem[]>([])
const categories = ref<Category[]>([])
const filterType = ref<'all' | 'transaction' | 'transfer'>('all')

const typeOptions = [
  { label: '全部', value: 'all' as const },
  { label: '收支', value: 'transaction' as const },
  { label: '转账', value: 'transfer' as const },
]

const mergedItems = computed(() => {
  let rows: MergedRow[] = []

  if (filterType.value === 'all' || filterType.value === 'transaction') {
    rows.push(...transactions.value.map(t => ({
      _key: `txn-${t.id}`,
      _type: 'transaction' as const,
      txnDate: t.txnDate,
      status: t.status,
      direction: t.direction,
      amountCents: t.amountCents,
      categoryId: t.categoryId,
      note: t.note,
    })))
  }

  if (filterType.value === 'all' || filterType.value === 'transfer') {
    rows.push(...transfers.value.map(t => ({
      _key: `trf-${t.id}`,
      _type: 'transfer' as const,
      txnDate: t.txnDate,
      status: t.status,
      sourceCategoryId: t.sourceCategoryId,
      sourceAmountCents: t.sourceAmountCents,
      note: t.note,
      legs: t.legs,
    })))
  }

  rows.sort((a, b) => {
    if (a.txnDate !== b.txnDate) return b.txnDate.localeCompare(a.txnDate)
    return a._key.localeCompare(b._key)
  })

  return rows
})

const summary = computed(() => {
  const normals = transactions.value.filter(i => i.status === 'normal')
  const income = normals.filter(i => i.direction === 'income').reduce((s, i) => s + i.amountCents, 0)
  const expense = normals.filter(i => i.direction === 'expense').reduce((s, i) => s + i.amountCents, 0)
  return { incomeTotal: income, expenseTotal: expense, balance: income - expense }
})

onMounted(async () => {
  await loadData()
})

function categoryName(catId: number | undefined): string {
  if (catId == null) return '-'
  for (const l1 of categories.value) {
    if (l1.children) {
      for (const l2 of l1.children) {
        if (l2.id === catId) return l2.name
      }
    }
  }
  return `#${catId}`
}

function goHome() {
  router.push('/')
}

// ---- 筛选功能 ----
const showFilterDialog = ref(false)
const showDatePicker = ref(false)
const showCategoryFilterPicker = ref(false)

interface FilterState {
  dateFrom: string
  dateTo: string
  dateRange: (Date | null)[] | []
  keyword: string
  categoryId: number | null
  categoryName: string
  minAmount: string
  maxAmount: string
}

const filters = ref<FilterState>({
  dateFrom: '',
  dateTo: '',
  dateRange: [],
  keyword: '',
  categoryId: null,
  categoryName: '',
  minAmount: '',
  maxAmount: '',
})

const categoryFilterOptions = computed(() => {
  const options: { text: string; value: number }[] = [{ text: '全部科目', value: 0 }]
  for (const l1 of categories.value) {
    if (l1.children) {
      for (const l2 of l1.children) {
        if (l2.status === 'active') {
          options.push({
            text: `${l1.name} / ${l2.name}`,
            value: l2.id,
          })
        }
      }
    }
  }
  return options
})

const hasActiveFilters = computed(() => {
  return filters.value.dateFrom !== '' || filters.value.keyword !== '' ||
    filters.value.categoryId !== null || filters.value.minAmount !== ''
})

const activeFilterText = computed(() => {
  const parts: string[] = []
  if (filters.value.dateFrom) {
    parts.push(`${filters.value.dateFrom} ~ ${filters.value.dateTo}`)
  }
  if (filters.value.keyword) parts.push(`"${filters.value.keyword}"`)
  if (filters.value.categoryName) parts.push(filters.value.categoryName)
  if (filters.value.minAmount) parts.push(`≧${filters.value.minAmount}`)
  return parts.join('、')
})

function onDateConfirm() {
  const range = filters.value.dateRange
  if (range && range.length === 2 && range[0] && range[1]) {
    const fmt = (d: Date) => {
      const y = d.getFullYear()
      const m = String(d.getMonth() + 1).padStart(2, '0')
      const day = String(d.getDate()).padStart(2, '0')
      return `${y}-${m}-${day}`
    }
    filters.value.dateFrom = fmt(range[0])
    filters.value.dateTo = fmt(range[1])
  }
  showDatePicker.value = false
}

function onCategoryFilterConfirm({ selectedOptions }: any) {
  const opt = selectedOptions[0]
  if (opt) {
    if (opt.value === 0) {
      filters.value.categoryName = ''
      filters.value.categoryId = null
    } else {
      filters.value.categoryName = opt.text
      filters.value.categoryId = opt.value
    }
  }
  showCategoryFilterPicker.value = false
}

function resetFilters() {
  filters.value = {
    dateFrom: '',
    dateTo: '',
    dateRange: [],
    keyword: '',
    categoryId: null,
    categoryName: '',
    minAmount: '',
    maxAmount: '',
  }
}

function clearFilters() {
  resetFilters()
  loadData()
}

function applyFilters() {
  showFilterDialog.value = false
  loadData()
}

async function loadData() {
  loading.value = true
  await Promise.all([loadTransactions(), loadTransfers(), loadCategories()])
  loading.value = false
}

async function loadTransactions() {
  try {
    const params: Record<string, string | number | boolean | undefined> = { page: 1, pageSize: 200 }
    if (filters.value.dateFrom) params.from = filters.value.dateFrom
    if (filters.value.dateTo) params.to = filters.value.dateTo
    if (filters.value.keyword) params.keyword = filters.value.keyword
    if (filters.value.categoryId) params.categoryId = filters.value.categoryId
    if (filters.value.minAmount) {
      params.minAmount = Math.round(parseFloat(filters.value.minAmount) * 100)
    }
    if (filters.value.maxAmount) {
      params.maxAmount = Math.round(parseFloat(filters.value.maxAmount) * 100)
    }
    const res = await api.get<ApiResponse<{ items: Transaction[]; total: number }>>('/transactions', params)
    transactions.value = res.data.items
  } catch {
    transactions.value = []
  }
}

async function loadTransfers() {
  try {
    const params: Record<string, string | number | boolean | undefined> = { page: 1, pageSize: 200 }
    if (filters.value.dateFrom) params.from = filters.value.dateFrom
    if (filters.value.dateTo) params.to = filters.value.dateTo
    if (filters.value.categoryId) params.categoryId = filters.value.categoryId
    const res = await api.get<ApiResponse<{ items: TransferItem[]; total: number }>>('/transfers', params)
    transfers.value = res.data.items
  } catch {
    transfers.value = []
  }
}

async function loadCategories() {
  try {
    const res = await api.get<ApiResponse<Category[]>>('/categories')
    categories.value = res.data
  } catch {
    categories.value = []
  }
}

// ---- 导出功能 ----
const showExportPicker = ref(false)

const exportActions = [
  { name: '导出 CSV', value: 'csv' },
  { name: '导出 Excel', value: 'xlsx' },
]

function onExportSelect(action: { value: string }) {
  showExportPicker.value = false
  handleExport(action.value as 'csv' | 'xlsx')
}

async function handleExport(format: 'csv' | 'xlsx') {
  try {
    const params: Record<string, string | number | boolean | undefined> = { page: 1, pageSize: 10000 }
    if (filters.value.dateFrom) params.from = filters.value.dateFrom
    if (filters.value.dateTo) params.to = filters.value.dateTo
    if (filters.value.keyword) params.keyword = filters.value.keyword
    if (filters.value.categoryId) params.categoryId = filters.value.categoryId
    if (filters.value.minAmount) {
      params.minAmount = Math.round(parseFloat(filters.value.minAmount) * 100)
    }
    if (filters.value.maxAmount) {
      params.maxAmount = Math.round(parseFloat(filters.value.maxAmount) * 100)
    }

    const res = await api.get<ApiResponse<{ items: Transaction[]; total: number }>>('/transactions', params)
    const items = res.data.items

    if (items.length === 0) {
      showToast('没有数据可导出')
      return
    }

    const rows = items.map(t => ({
      '日期': t.txnDate,
      '方向': t.direction === 'income' ? '收入' : '支出',
      '科目': categoryName(t.categoryId),
      '金额(元)': (t.amountCents / 100).toFixed(2),
      '摘要': t.note || '',
      '状态': t.status === 'normal' ? '正常' : '已作废',
    }))

    if (format === 'csv') {
      const header = '日期,方向,科目,金额(元),摘要,状态\n'
      const csvRows = rows.map(r =>
        `${r['日期']},${r['方向']},${r['科目']},${r['金额(元)']},"${r['摘要'].replace(/"/g, '""')}",${r['状态']}`
      ).join('\n')

      const blob = new Blob(['\ufeff' + header + csvRows], { type: 'text/csv;charset=utf-8;' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `流水导出_${new Date().toISOString().slice(0, 10)}.csv`
      a.click()
      URL.revokeObjectURL(url)
    } else {
      const ws = XLSX.utils.json_to_sheet(rows)
      const wb = XLSX.utils.book_new()
      XLSX.utils.book_append_sheet(wb, ws, '流水')
      XLSX.writeFile(wb, `流水导出_${new Date().toISOString().slice(0, 10)}.xlsx`)
    }

    showToast('导出成功')
  } catch (e: any) {
    showToast(e.message || '导出失败')
  }
}

// ---- 编辑弹窗 ----
const showEditDialog = ref(false)
const savingEdit = ref(false)
const showEditCategoryPicker = ref(false)

const editTransaction = ref<Transaction | null>(null)
const editTransfer = ref<TransferItem | null>(null)
const editChangelog = ref<ChangeLogItem[]>([])
const editTransferChangelog = ref<ChangeLogItem[]>([])

const editForm = ref({
  date: '',
  note: '',
  categoryName: '',
  categoryId: 0,
  amount: '',
  direction: 'expense' as 'income' | 'expense',
})

const categoryOptions = computed(() => {
  const options: { text: string; value: number }[] = []
  for (const l1 of categories.value) {
    if (l1.children) {
      for (const l2 of l1.children) {
        if (l2.status === 'active') {
          options.push({
            text: `${l1.name} / ${l2.name}`,
            value: l2.id,
          })
        }
      }
    }
  }
  return options
})

async function openEdit(item: MergedRow) {
  if (item._type === 'transaction') {
    const txn = transactions.value.find(t => `txn-${t.id}` === item._key)
    if (!txn) return
    editTransaction.value = txn
    editTransfer.value = null
    editForm.value = {
      date: txn.txnDate,
      note: txn.note || '',
      categoryName: categoryName(txn.categoryId),
      categoryId: txn.categoryId,
      amount: (txn.amountCents / 100).toFixed(2),
      direction: txn.direction,
    }
    await loadChangelog('transaction', txn.id)
  } else {
    const trf = transfers.value.find(t => `trf-${t.id}` === item._key)
    if (!trf) return
    editTransaction.value = null
    editTransfer.value = trf
    editChangelog.value = []
    await loadChangelog('transfer', trf.id)
  }
  showEditDialog.value = true
}

async function loadChangelog(entityType: string, entityId: number) {
  try {
    const res = await api.get<ApiResponse<ChangeLogItem[]>>('/changelog', { entityType, entityId })
    if (entityType === 'transaction') {
      editChangelog.value = res.data
    } else {
      editTransferChangelog.value = res.data
    }
  } catch {
    if (entityType === 'transaction') {
      editChangelog.value = []
    } else {
      editTransferChangelog.value = []
    }
  }
}

function onEditCategoryConfirm({ selectedOptions }: any) {
  const opt = selectedOptions[0]
  if (opt) {
    editForm.value.categoryName = opt.text
    editForm.value.categoryId = opt.value
  }
  showEditCategoryPicker.value = false
}

function editValidateAmount(val: string): boolean {
  const num = parseFloat(val)
  return !isNaN(num) && num > 0
}

async function handleSaveEdit() {
  if (!editTransaction.value) return
  const txn = editTransaction.value
  const amountFen = Math.round(parseFloat(editForm.value.amount) * 100)
  if (amountFen <= 0) {
    showToast('金额必须大于 0')
    return
  }

  savingEdit.value = true
  try {
    const body: Record<string, any> = {}
    if (editForm.value.date !== txn.txnDate) body.date = editForm.value.date
    if (editForm.value.direction !== txn.direction) body.direction = editForm.value.direction
    if (amountFen !== txn.amountCents) body.amountCents = amountFen
    if (editForm.value.categoryId !== txn.categoryId) body.categoryId = editForm.value.categoryId
    if (editForm.value.note !== (txn.note || '')) body.note = editForm.value.note

    if (Object.keys(body).length === 0) {
      showToast('没有修改')
      return
    }

    await api.put(`/transactions/${txn.id}`, body)
    showToast('修改成功')
    showEditDialog.value = false
    await loadData()
  } catch (e: any) {
    showToast(e.message || '修改失败')
  } finally {
    savingEdit.value = false
  }
}

async function handleVoidTransaction() {
  if (!editTransaction.value) return
  try {
    await api.put(`/transactions/${editTransaction.value.id}`, { status: 'voided' })
    showToast('已作废')
    showEditDialog.value = false
    await loadData()
  } catch (e: any) {
    showToast(e.message || '作废失败')
  }
}

async function handleUnvoidTransaction() {
  if (!editTransaction.value) return
  try {
    await api.put(`/transactions/${editTransaction.value.id}`, { status: 'normal' })
    showToast('已撤销作废')
    showEditDialog.value = false
    await loadData()
  } catch (e: any) {
    showToast(e.message || '撤销作废失败')
  }
}

async function handleVoidTransfer() {
  if (!editTransfer.value) return
  try {
    await api.put(`/transfers/${editTransfer.value.id}`, { status: 'voided' })
    showToast('已作废')
    showEditDialog.value = false
    await loadData()
  } catch (e: any) {
    showToast(e.message || '作废失败')
  }
}

async function handleUnvoidTransfer() {
  if (!editTransfer.value) return
  try {
    await api.put(`/transfers/${editTransfer.value.id}`, { status: 'normal' })
    showToast('已撤销作废')
    showEditDialog.value = false
    await loadData()
  } catch (e: any) {
    showToast(e.message || '撤销作废失败')
  }
}

function changelogActionText(action: string): string {
  const map: Record<string, string> = {
    create: '创建',
    update: '修改',
    void: '作废',
    unvoid: '撤销作废',
  }
  return map[action] || action
}

function changelogFieldText(field: string): string {
  const map: Record<string, string> = {
    txn_date: '日期',
    direction: '方向',
    amount_cents: '金额',
    category_id: '科目',
    note: '摘要',
    status: '状态',
  }
  return map[field] || field
}

function formatChangelogValue(field: string | null, value: string | null): string {
  if (value == null) return '-'
  if (field === 'amount_cents') {
    return formatFen(parseInt(value, 10))
  }
  if (field === 'direction') {
    return value === 'income' ? '收入' : '支出'
  }
  if (field === 'category_id') {
    return categoryName(parseInt(value, 10))
  }
  if (field === 'status') {
    return value === 'normal' ? '正常' : '已作废'
  }
  return value
}
</script>

<style scoped>
.list-page {
  padding: 16px;
  padding-bottom: 60px;
  min-height: 100vh;
  background: #f7f7f5;
}

.list-header {
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.list-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: #2c2c2a;
  margin: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.type-filter {
  display: flex;
  gap: 6px;
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
}

.empty-state p {
  color: #8f8e88;
  margin-bottom: 16px;
}

.list-content {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}

.txn-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #f0f0eb;
  cursor: pointer;
  transition: background 0.15s;
}

.txn-row:active {
  background: #f0f0eb;
}

.txn-row:last-child {
  border-bottom: none;
}

.txn-row.voided {
  opacity: 0.5;
  text-decoration: line-through;
}

.txn-main {
  flex: 1;
  min-width: 0;
}

.txn-date {
  font-size: 12px;
  color: #8f8e88;
}

.txn-category {
  font-size: 14px;
  color: #2c2c2a;
  margin: 2px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.transfer-tag {
  display: inline-block;
  font-size: 11px;
  color: #185fa5;
  background: #e6f1fb;
  border-radius: 4px;
  padding: 0 5px;
  margin-right: 4px;
  vertical-align: middle;
}

.transfer-arrow {
  margin: 0 2px;
  color: #8f8e88;
}

.txn-note {
  font-size: 12px;
  color: #8f8e88;
}

.txn-amount {
  font-size: 15px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  margin-left: 12px;
}

.txn-amount.income {
  color: #0f6e56;
}

.txn-amount.expense {
  color: #a32d2d;
}

.txn-amount.transfer {
  color: #185fa5;
}

.list-summary {
  display: flex;
  gap: 16px;
  justify-content: center;
  padding: 12px;
  margin-top: 12px;
  background: #fff;
  border-radius: 12px;
  font-size: 13px;
  color: #5f5e5a;
}

/* 筛选 */
.filter-hint {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  margin-bottom: 12px;
  background: #e6f1fb;
  border-radius: 8px;
  font-size: 12px;
  color: #185fa5;
}

.filter-popup {
  padding: 0 0 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.filter-title {
  font-size: 16px;
  font-weight: 500;
  color: #2c2c2a;
  padding: 16px;
  border-bottom: 1px solid #f0f0eb;
}

.amount-range {
  display: flex;
  align-items: center;
  padding: 0 12px;
}

.amount-range .van-field {
  flex: 1;
}

.amount-sep {
  margin: 0 8px;
  color: #8f8e88;
  font-size: 16px;
}

.filter-actions {
  display: flex;
  gap: 12px;
  padding: 16px;
  justify-content: flex-end;
}

/* 编辑弹窗 */
.edit-popup {
  padding: 0 0 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.edit-title {
  font-size: 16px;
  font-weight: 500;
  color: #2c2c2a;
  padding: 16px;
  border-bottom: 1px solid #f0f0eb;
}

.edit-direction-toggle {
  display: flex;
  gap: 8px;
  padding: 12px 16px 0;
}

.edit-actions {
  padding: 12px 16px;
}

.changelog-section {
  margin: 12px 16px;
  background: #f7f7f5;
  border-radius: 8px;
  padding: 12px;
}

.section-title {
  font-size: 13px;
  font-weight: 500;
  color: #5f5e5a;
  margin-bottom: 8px;
}

.changelog-item {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 8px;
  font-size: 12px;
  color: #5f5e5a;
  padding: 4px 0;
  border-bottom: 1px solid #ecebe5;
}

.changelog-item:last-child {
  border-bottom: none;
}

.changelog-action {
  font-weight: 500;
  color: #185fa5;
}

.changelog-field {
  color: #2c2c2a;
}

.changelog-diff {
  color: #8f8e88;
}

.changelog-time {
  color: #8f8e88;
  margin-left: auto;
}
</style>
