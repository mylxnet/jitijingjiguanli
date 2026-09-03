<template>
  <div class="categories-page">
    <div class="page-header">
      <h3>科目管理</h3>
      <div class="header-actions">
        <van-button type="primary" size="small" @click="showTransferDialog = true">转账</van-button>
        <van-button type="primary" plain size="small" icon="exchange" @click="openFundMovePopup">资金划转</van-button>
        <van-button type="primary" size="small" @click="openAddL1">+ 新增一级</van-button>
      </div>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="loading-state">
      <van-skeleton title :row="5" />
    </div>

    <!-- 空数据 -->
    <div v-else-if="categories.length === 0" class="empty-state">
      <p>还没有科目</p>
      <van-button type="primary" size="small" @click="openAddL1">新增一级科目</van-button>
    </div>

    <!-- 科目树 -->
    <div v-else class="category-tree">
      <div v-for="l1 in categories" :key="l1.id" class="l1-group">
        <div class="l1-row">
          <div class="l1-info">
            <span class="l1-name">{{ l1.name }}</span>
            <span class="l1-badge">一级</span>
            <span v-if="l1.preset" class="l1-chip preset">预置</span>
            <span class="l1-chip">分组</span>
          </div>
          <div class="l1-actions">
            <van-button size="mini" plain @click="openAddL2(l1)">+二级</van-button>
            <van-button size="mini" plain @click="renameCat(l1)">重命名</van-button>
            <van-button v-if="!l1.children?.length" size="mini" plain type="danger" @click="deleteCat(l1)">删除</van-button>
          </div>
        </div>

        <div class="l2-list" v-if="l1.children && l1.children.length > 0">
          <div v-for="l2 in l1.children" :key="l2.id" class="l2-row" :class="{ inactive: l2.status === 'inactive' }">
            <div class="l2-info">
              <span class="l2-name">{{ l2.name }}</span>
              <span v-if="l2.preset" class="l2-chip preset">预置</span>
              <span class="l2-chip" :class="l2.kind">{{ l2.kind === 'asset' ? '资产' : '权益' }}</span>
              <span v-if="l2.status === 'inactive'" class="l2-chip stopped">已停用</span>
              <span class="l2-balance">{{ l2.kind === 'asset' ? '在外' : '余额' }} {{ formatFen(l2.balanceCents ?? 0) }}</span>
              <span v-if="(l2.openingBalanceCents || 0) !== 0" class="l2-opening">期初 {{ formatFen(l2.openingBalanceCents || 0) }}</span>
              <span v-if="l2.kind !== 'asset' && l2.txnCount != null" class="l2-count">{{ l2.txnCount }}笔</span>
            </div>
            <div class="l2-actions">
              <van-button size="mini" plain @click="editL2(l2)">编辑</van-button>
              <van-button size="mini" plain @click="renameCat(l2)">重命名</van-button>
              <van-button
                size="mini"
                plain
                :type="l2.status === 'active' ? 'warning' : 'primary'"
                @click="toggleStatus(l2)"
              >{{ l2.status === 'active' ? '停用' : '启用' }}</van-button>
              <van-button size="mini" plain type="danger" @click="deleteCat(l2)">删除</van-button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="page-footer-note">
      <van-icon name="info-o" />
      已被引用的科目不能删除，只能停用
    </div>

    <!-- 新增一级科目对话框（一级是分组容器，只填名称） -->
    <van-dialog v-model:show="showAddDialog" title="新增一级科目" show-cancel-button @confirm="handleAddL1">
      <div class="dialog-tip">一级科目用于分组，二级科目才选择「资产/权益」类型</div>
      <van-field v-model="addForm.name" label="名称" placeholder="科目名称" :rules="[{ required: true }]" />
    </van-dialog>

    <!-- 新增/编辑二级科目对话框 -->
    <van-dialog v-model:show="showAddL2Dialog" :title="editL2Mode ? '编辑二级科目' : '新增二级科目 - ' + addL2ParentName" show-cancel-button @confirm="handleAddL2">
      <van-field v-model="addL2Form.name" label="名称" placeholder="科目名称" :rules="[{ required: true }]" class="only-mobile">
        <template v-if="!editL2Mode" #button>
          <span class="unit-quick" @click="openUnitPicker">选择往来单位 ▾</span>
        </template>
      </van-field>
      <NativeSelect
        v-if="!editL2Mode"
        label="选单位填名称"
        placeholder="从往来单位中选择"
        :model-value="null"
        :options="unitActions"
        @update:model-value="nativeUnitPick"
      />
      <div v-if="!editL2Mode && unitList.length === 0" class="field-hint">
        暂无往来单位；可手动输入科目名，或先到「往来」页新增单位
      </div>
      <van-field v-if="!editL2Mode" label="科目类型">
        <template #input>
          <van-radio-group v-model="addL2Form.kind" direction="horizontal">
            <van-radio name="equity">权益</van-radio>
            <van-radio name="asset">资产</van-radio>
          </van-radio-group>
        </template>
      </van-field>
      <div v-if="!editL2Mode && addL2Form.kind === 'asset'" class="field-hint">
        资产科目代表投到银行外的钱（如对外投资某公司）：期初填建账时已在外的金额；之后进出走「资金划转」
      </div>
      <div v-else-if="!editL2Mode" class="field-hint">
        权益科目记录收入/支出的去向与来源；期初填建账时该科目已有的存量
      </div>
      <van-field
        v-model="addL2Form.openingBalance"
        label="期初余额"
        type="number"
        placeholder="0"
        inputmode="decimal"
      />
    </van-dialog>

    <!-- 重命名对话框 -->
    <van-dialog v-model:show="showRenameDialog" title="重命名" show-cancel-button @confirm="handleRename">
      <van-field v-model="renameForm.name" label="名称" placeholder="新名称" :rules="[{ required: true }]" />
    </van-dialog>

    <!-- 转账对话框 -->
    <van-popup v-model:show="showTransferDialog" position="bottom" round closeable style="max-height: 90vh">
      <div class="transfer-popup">
        <div class="popup-title">科目间转账</div>

        <!-- 转出科目 -->
        <van-field
          v-model="transferSourceName"
          is-link
          readonly
          label="转出科目"
          placeholder="请选择转出科目"
          @click="showSourcePicker = true"
          class="only-mobile"
        />
        <NativeSelect
          label="转出科目"
          placeholder="请选择转出科目"
          v-model="transferSourceCategoryId"
          :options="sourceCategoryOptions"
        />
        <div v-if="selectedSourceBalance !== null" class="transfer-hint">
          可转出 {{ formatFen(selectedSourceBalance) }}
        </div>

        <!-- 转出金额 -->
        <van-field
          v-model="transferAmountYuan"
          label="转出金额"
          type="number"
          placeholder="0.00"
          inputmode="decimal"
        />

        <!-- 转入明细 -->
        <div class="transfer-legs-section">
          <div class="legs-header">
            <span class="legs-title">转入明细</span>
            <van-button size="mini" plain type="primary" @click="addLeg">+ 添加</van-button>
          </div>
          <div v-for="(leg, index) in transferLegs" :key="index" class="leg-row">
            <div class="leg-row-header">
              <span class="leg-label">转入 {{ index + 1 }}</span>
              <van-button v-if="transferLegs.length > 1" size="mini" plain type="danger" @click="removeLeg(index)">删除</van-button>
            </div>
            <van-field
              v-model="leg.categoryName"
              is-link
              readonly
              placeholder="请选择转入科目"
              @click="openLegPicker(index)"
              class="only-mobile"
            />
            <NativeSelect
              placeholder="请选择转入科目"
              v-model="leg.categoryId"
              :options="destCategoryOptions"
            />
            <van-field
              v-model="leg.amountYuan"
              label="金额"
              type="number"
              placeholder="0.00"
              inputmode="decimal"
            />
          </div>
        </div>

        <!-- 合计校验 -->
        <div class="transfer-total-check">
          <span>转出金额：{{ formatFen(transferAmountCents) }}</span>
          <span>已分配：{{ formatFen(allocatedTotalCents) }}</span>
          <span v-if="!isAmountValid" class="check-error">
            差额 {{ formatFen(Math.abs(transferAmountCents - allocatedTotalCents)) }}
          </span>
        </div>

        <!-- 摘要 -->
        <van-field
          v-model="transferNote"
          label="摘要"
          placeholder="转账说明"
        />
        <div class="transfer-tags">
          <van-tag
            v-for="tag in quickTags"
            :key="tag"
            :type="transferNote === tag ? 'primary' : 'default'"
            plain
            @click="transferNote = tag"
          >{{ tag }}</van-tag>
        </div>

        <!-- 保存 -->
        <div class="transfer-save">
          <van-button
            round
            block
            type="primary"
            :disabled="!canSubmitTransfer"
            :loading="savingTransfer"
            @click="handleTransfer"
          >保存转账</van-button>
        </div>
      </div>
    </van-popup>

    <!-- 转出科目选择器 -->
    <van-action-sheet
      v-model:show="showSourcePicker"
      title="选择转出科目"
      :actions="sourceCategoryOptions"
      @select="onSourceCategorySelect"
      @cancel="showSourcePicker = false"
    />

    <!-- 转入科目选择器 -->
    <van-action-sheet
      v-model:show="showLegPicker"
      title="选择转入科目"
      :actions="destCategoryOptions"
      @select="onLegCategorySelect"
      @cancel="showLegPicker = false"
    />

    <!-- 资金划转弹层（D10：投资=银行→资产；收回=资产→银行） -->
    <van-popup v-model:show="showFundMovePopup" position="bottom" round closeable style="max-height: 90vh">
      <div class="fundmove-popup">
        <div class="popup-title">资金划转</div>

        <van-field label="方向">
          <template #input>
            <van-radio-group v-model="fundMoveForm.kind" direction="horizontal">
              <van-radio name="invest">投资（银行→资产）</van-radio>
              <van-radio name="recover">收回（资产→银行）</van-radio>
            </van-radio-group>
          </template>
        </van-field>
        <van-field v-model="fundMoveForm.date" label="日期" placeholder="YYYY-MM-DD" :rules="[{ required: true }]" />
        <van-field
          v-model="fundMoveAssetName"
          is-link
          readonly
          label="资产科目"
          placeholder="请选择资产科目"
          @click="showAssetPicker = true"
          class="only-mobile"
        />
        <NativeSelect
          label="资产科目"
          placeholder="请选择资产科目"
          v-model="fundMoveAssetId"
          :options="assetCategoryOptions"
        />
        <div v-if="selectedAssetOutstanding !== null" class="transfer-hint">
          该资产目前在外 {{ formatFen(selectedAssetOutstanding) }}
        </div>
        <van-field
          v-model="fundMoveForm.amount"
          label="金额"
          type="number"
          placeholder="0.00"
          inputmode="decimal"
        />
        <van-field v-model="fundMoveForm.note" label="摘要" placeholder="项目/事由（可选）" />

        <div class="fundmove-save">
          <van-button
            round
            block
            type="primary"
            :disabled="!canSubmitFundMove"
            :loading="savingFundMove"
            @click="handleFundMove"
          >保存{{ fundMoveForm.kind === 'invest' ? '投资' : '收回' }}</van-button>
        </div>

        <!-- 划转记录 -->
        <div class="fundmove-records">
          <div class="legs-header">
            <span class="legs-title">划转记录</span>
            <van-button size="mini" plain @click="loadFundMoves">刷新</van-button>
          </div>
          <div v-if="fundMoves.length === 0" class="records-empty">暂无记录</div>
          <div v-for="mv in fundMoves" :key="mv.id" class="record-row" :class="{ voided: mv.status === 'voided' }">
            <div class="record-info">
              <span class="record-kind" :class="mv.kind">{{ mv.kind === 'invest' ? '投资' : '收回' }}</span>
              <span class="record-cat">{{ fundMoveAssetNameOf(mv.assetCategoryId) }}</span>
              <span class="record-amount" :class="mv.kind">{{ mv.kind === 'invest' ? '-' : '+' }}{{ formatFen(mv.amountCents) }}</span>
              <span class="record-date">{{ mv.moveDate }}</span>
              <span v-if="mv.status === 'voided'" class="l2-chip stopped">已作废</span>
            </div>
            <van-button
              v-if="mv.status === 'normal'"
              size="mini"
              plain
              type="warning"
              @click="voidFundMove(mv)"
            >作废</van-button>
            <van-button size="mini" plain @click="openMoveChangelog(mv)">留痕</van-button>
          </div>
        </div>
      </div>
    </van-popup>

    <!-- 资产科目选择器 -->
    <van-action-sheet
      v-model:show="showAssetPicker"
      title="选择资产科目"
      :actions="assetCategoryOptions"
      @select="onAssetCategorySelect"
      @cancel="showAssetPicker = false"
    />

    <!-- 划转留痕查看 -->
    <ChangeLogDialog v-model:show="chgShow" entity-type="fund_move" :entity-id="chgId" />

    <!-- 往来单位选择（填入科目名称） -->
    <van-action-sheet
      v-model:show="showUnitPicker"
      title="选择往来单位（自动填入科目名称）"
      :actions="unitActions"
      @select="onUnitSelect"
      @cancel="showUnitPicker = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../lib/http'
import { showToast, showDialog } from 'vant'
import ChangeLogDialog from '../../components/ChangeLogDialog.vue'
import NativeSelect from '../../components/NativeSelect.vue'
import type { Category, Party, FundMove, FundMoveListResponse, ApiResponse } from '../../types/api'
import { formatFen, todayStr } from '../../types/api'

const loading = ref(true)
const categories = ref<Category[]>([])

// ---- 往来单位（新增二级科目名称下拉，需求：可下拉自动填入） ----
const unitList = ref<Party[]>([])
const showUnitPicker = ref(false)
const unitActions = computed(() => unitList.value.map(p => ({ name: p.name, value: p.id })))

async function loadUnits() {
  try {
    const res = await api.get<ApiResponse<Party[]>>('/parties')
    unitList.value = res.data || []
  } catch {
    unitList.value = []
  }
}

// ---- 新增一级 ----
const showAddDialog = ref(false)
const addForm = ref({ name: '' })

function openAddL1() {
  addForm.value = { name: '' }
  showAddDialog.value = true
}

// ---- 新增/编辑二级 ----
const showAddL2Dialog = ref(false)
const editL2Mode = ref(false)
const editL2Id = ref(0)
const addL2Form = ref({
  name: '',
  kind: 'equity' as 'equity' | 'asset',
  openingBalance: '0',
})
const addL2ParentId = ref<number | null>(null)
const addL2ParentName = ref('')

const defaultAddL2Form = () => ({
  name: '',
  kind: 'equity' as 'equity' | 'asset',
  openingBalance: '0',
})

function openAddL2(l1: Category) {
  editL2Mode.value = false
  editL2Id.value = 0
  addL2ParentId.value = l1.id
  addL2ParentName.value = l1.name
  addL2Form.value = defaultAddL2Form()
  showAddL2Dialog.value = true
}

function editL2(l2: Category) {
  editL2Mode.value = true
  editL2Id.value = l2.id
  addL2ParentId.value = l2.parentId
  addL2ParentName.value = ''
  addL2Form.value = {
    name: l2.name,
    kind: l2.kind,
    openingBalance: String((l2.openingBalanceCents || 0) / 100),
  }
  showAddL2Dialog.value = true
}

function openUnitPicker() {
  showUnitPicker.value = true
}

function onUnitSelect(action: { name: string; value: number }) {
  addL2Form.value.name = action.name
  showUnitPicker.value = false
}

function nativeUnitPick(v: number | string | null) {
  const id = Number(v)
  const p = unitList.value.find(x => x.id === id)
  if (p) addL2Form.value.name = p.name
}

// ---- 重命名 ----
const showRenameDialog = ref(false)
const renameForm = ref({ id: 0, name: '' })

// ---- 科目间转账 ----
const showTransferDialog = ref(false)
const showSourcePicker = ref(false)
const showLegPicker = ref(false)
const currentLegIndex = ref(0)
const savingTransfer = ref(false)

const transferSourceCategoryId = ref<number | null>(null)
const transferSourceName = ref('')
const transferAmountYuan = ref('')
const transferNote = ref('')
const quickTags = ['年末结转', '收益分配', '专款调剂']

interface TransferLeg {
  categoryId: number | null
  categoryName: string
  amountYuan: string
}

const transferLegs = ref<TransferLeg[]>([{ categoryId: null, categoryName: '', amountYuan: '' }])

// 所有启用中的普通二级科目（用于科目间转账；资产科目走资金划转，不在此列）
const activeL2Categories = computed(() => {
  const result: Category[] = []
  for (const l1 of categories.value) {
    if (l1.children) {
      for (const l2 of l1.children) {
        if (l2.status === 'active' && l2.kind === 'equity') {
          result.push(l2)
        }
      }
    }
  }
  return result
})

// 启用中的资产二级科目（资金划转可挂，D10）
const assetL2Categories = computed(() => {
  const result: Category[] = []
  for (const l1 of categories.value) {
    if (l1.children) {
      for (const l2 of l1.children) {
        if (l2.status === 'active' && l2.kind === 'asset') {
          result.push(l2)
        }
      }
    }
  }
  return result
})

// 转出科目选择器选项（权益二级，带余额）
const sourceCategoryOptions = computed(() => {
  return activeL2Categories.value.map(c => ({
    name: `${c.name} (余额 ${formatFen(c.balanceCents ?? 0)})`,
    value: c.id,
  }))
})

// 转入科目选择：所有启用权益二级（v0.4 无花费型限制）
const destCategoryOptions = computed(() => {
  return activeL2Categories.value.map(c => ({
    name: `${c.name} (${formatFen(c.balanceCents ?? 0)})`,
    value: c.id,
  }))
})

// 选中转出科目的余额
const selectedSourceBalance = computed(() => {
  if (!transferSourceCategoryId.value) return null
  const cat = activeL2Categories.value.find(c => c.id === transferSourceCategoryId.value)
  return cat?.balanceCents ?? null
})

// 转出金额（分）
const transferAmountCents = computed(() => {
  return Math.round(parseFloat(transferAmountYuan.value || '0') * 100)
})

// 已分配总额（分）
const allocatedTotalCents = computed(() => {
  return transferLegs.value.reduce((sum, leg) => {
    return sum + Math.round(parseFloat(leg.amountYuan || '0') * 100)
  }, 0)
})

// 金额是否匹配
const isAmountValid = computed(() => {
  return transferAmountCents.value > 0 && transferAmountCents.value === allocatedTotalCents.value
})

// 是否可以提交转账
const canSubmitTransfer = computed(() => {
  return (
    transferSourceCategoryId.value !== null &&
    transferAmountCents.value > 0 &&
    isAmountValid.value &&
    transferLegs.value.every(l => l.categoryId !== null && l.amountYuan !== '')
  )
})

function onSourceCategorySelect(action: { name: string; value: number }) {
  transferSourceCategoryId.value = action.value
  transferSourceName.value = action.name
  showSourcePicker.value = false
}

function openLegPicker(index: number) {
  currentLegIndex.value = index
  showLegPicker.value = true
}

function onLegCategorySelect(action: { name: string; value: number }) {
  const index = currentLegIndex.value
  transferLegs.value[index].categoryId = action.value
  transferLegs.value[index].categoryName = action.name
  showLegPicker.value = false
}

function addLeg() {
  transferLegs.value.push({ categoryId: null, categoryName: '', amountYuan: '' })
}

function removeLeg(index: number) {
  transferLegs.value.splice(index, 1)
}

function resetTransferForm() {
  transferSourceCategoryId.value = null
  transferSourceName.value = ''
  transferAmountYuan.value = ''
  transferNote.value = ''
  transferLegs.value = [{ categoryId: null, categoryName: '', amountYuan: '' }]
}

async function handleTransfer() {
  if (!canSubmitTransfer.value) return
  savingTransfer.value = true
  try {
    const legs = transferLegs.value.map(l => ({
      categoryId: l.categoryId!,
      amountCents: Math.round(parseFloat(l.amountYuan || '0') * 100),
    }))
    await api.post('/transfers', {
      txnDate: new Date().toISOString().slice(0, 10),
      sourceCategoryId: transferSourceCategoryId.value,
      sourceAmountCents: transferAmountCents.value,
      note: transferNote.value || '',
      legs,
    })
    showToast('转账成功')
    showTransferDialog.value = false
    resetTransferForm()
  } catch (e: any) {
    showToast(e.message || '转账失败')
  } finally {
    savingTransfer.value = false
  }
}

// ---- 资金划转（D10：投资 = 银行→资产；收回 = 资产→银行） ----
const showFundMovePopup = ref(false)
const showAssetPicker = ref(false)
const savingFundMove = ref(false)
const fundMoves = ref<FundMove[]>([])

const fundMoveForm = ref({
  kind: 'invest' as 'invest' | 'recover',
  date: todayStr(),
  amount: '',
  note: '',
})
const fundMoveAssetId = ref<number | null>(null)
const fundMoveAssetName = ref('')

const defaultFundMoveForm = () => ({
  kind: 'invest' as 'invest' | 'recover',
  date: todayStr(),
  amount: '',
  note: '',
})

// 资产科目选择器选项（带一级前缀与在外余额）
const assetCategoryOptions = computed(() => {
  const options: { name: string; value: number }[] = []
  for (const l1 of categories.value) {
    if (l1.children) {
      for (const l2 of l1.children) {
        if (l2.status === 'active' && l2.kind === 'asset') {
          options.push({
            name: `${l1.name} / ${l2.name}（在外 ${formatFen(l2.balanceCents ?? 0)}）`,
            value: l2.id,
          })
        }
      }
    }
  }
  return options
})

// 当前选中资产科目的在外余额（用于提示）
const selectedAssetOutstanding = computed(() => {
  if (!fundMoveAssetId.value) return null
  const cat = assetL2Categories.value.find(c => c.id === fundMoveAssetId.value)
  return cat?.balanceCents ?? null
})

const fundMoveAmountCents = computed(() => {
  return Math.round(parseFloat(fundMoveForm.value.amount || '0') * 100)
})

const canSubmitFundMove = computed(() => {
  return (
    fundMoveAssetId.value !== null &&
    fundMoveAmountCents.value > 0 &&
    fundMoveForm.value.date !== ''
  )
})

function openFundMovePopup() {
  fundMoveForm.value = defaultFundMoveForm()
  fundMoveAssetId.value = null
  fundMoveAssetName.value = ''
  showFundMovePopup.value = true
  void loadFundMoves()
}

function onAssetCategorySelect(action: { name: string; value: number }) {
  fundMoveAssetId.value = action.value
  fundMoveAssetName.value = action.name
  showAssetPicker.value = false
}

function fundMoveAssetNameOf(categoryId: number): string {
  const cat = assetL2Categories.value.find(c => c.id === categoryId)
  if (cat) return cat.name
  // 停用/删除后兜底显示 id
  return `科目 #${categoryId}`
}

async function loadFundMoves() {
  try {
    const res = await api.get<ApiResponse<FundMoveListResponse>>('/fund-moves', { pageSize: 50 })
    fundMoves.value = res.data.items || []
  } catch {
    fundMoves.value = []
  }
}

async function handleFundMove() {
  if (!canSubmitFundMove.value) return
  savingFundMove.value = true
  try {
    await api.post('/fund-moves', {
      moveDate: fundMoveForm.value.date,
      kind: fundMoveForm.value.kind,
      assetCategoryId: fundMoveAssetId.value,
      amountCents: fundMoveAmountCents.value,
      note: fundMoveForm.value.note || '',
    })
    showToast('保存成功')
    await loadCategories()
    await loadFundMoves()
    fundMoveForm.value.amount = ''
    fundMoveForm.value.note = ''
  } catch (e: any) {
    showToast(e.message || '保存失败')
  } finally {
    savingFundMove.value = false
  }
}

async function voidFundMove(mv: FundMove) {
  try {
    await showDialog({
      title: '确认作废',
      message: `作废这笔${mv.kind === 'invest' ? '投资' : '收回'}（${formatFen(mv.amountCents)}）？银行与资产余额将回滚。`,
      showCancelButton: true,
    })
  } catch {
    return // 用户取消
  }
  try {
    await api.put(`/fund-moves/${mv.id}`, { status: 'voided' })
    showToast('已作废')
    await loadCategories()
    await loadFundMoves()
  } catch (e: any) {
    showToast(e.message || '操作失败')
  }
}

// ---- 划转留痕（F6） ----
const chgShow = ref(false)
const chgId = ref(0)

function openMoveChangelog(mv: FundMove) {
  chgId.value = mv.id
  chgShow.value = true
}

// ---- 生命周期 ----
onMounted(async () => {
  await Promise.all([loadCategories(), loadUnits()])
  loading.value = false
})

async function loadCategories() {
  try {
    const res = await api.get<ApiResponse<Category[]>>('/categories')
    categories.value = res.data
  } catch {
    categories.value = []
  }
}

async function handleAddL1() {
  if (!addForm.value.name) {
    showToast('请填写名称')
    return
  }
  try {
    await api.post('/categories', {
      name: addForm.value.name,
      level: 1,
    })
    showToast('创建成功')
    addForm.value = { name: '' }
    await loadCategories()
  } catch (e: any) {
    showDialog({ title: '创建失败', message: e.message || '创建失败，请重试' })
  }
}

async function handleAddL2() {
  if (!addL2Form.value.name || !addL2ParentId.value) {
    showToast('请填写名称')
    return
  }
  try {
    if (editL2Mode.value) {
      // 编辑：改名/停用/期初（kind 不可变）
      const updPayload: Record<string, unknown> = { name: addL2Form.value.name }
      updPayload.openingBalanceCents = Math.round(parseFloat(addL2Form.value.openingBalance || '0') * 100)
      await api.put(`/categories/${editL2Id.value}`, updPayload)
      showToast('更新成功')
    } else {
      await api.post('/categories', {
        name: addL2Form.value.name,
        level: 2,
        parentId: addL2ParentId.value,
        kind: addL2Form.value.kind,
        openingBalanceCents: Math.round(parseFloat(addL2Form.value.openingBalance || '0') * 100),
      })
      showToast('创建成功')
    }
    showAddL2Dialog.value = false
    await loadCategories()
  } catch (e: any) {
    showDialog({ title: editL2Mode.value ? '保存失败' : '创建失败', message: e.message || (editL2Mode.value ? '保存失败，请重试' : '创建失败，请重试') })
  }
}

function renameCat(cat: Category) {
  renameForm.value = { id: cat.id, name: cat.name }
  showRenameDialog.value = true
}

async function handleRename() {
  if (!renameForm.value.name) {
    showToast('请填写名称')
    return
  }
  try {
    await api.put(`/categories/${renameForm.value.id}`, { name: renameForm.value.name })
    showToast('重命名成功')
    await loadCategories()
  } catch (e: any) {
    showToast(e.message || '重命名失败')
  }
}

async function toggleStatus(cat: Category) {
  const newStatus = cat.status === 'active' ? 'inactive' : 'active'
  try {
    await api.put(`/categories/${cat.id}`, { status: newStatus })
    showToast(newStatus === 'active' ? '已启用' : '已停用')
    await loadCategories()
  } catch (e: any) {
    showToast(e.message || '操作失败')
  }
}

async function deleteCat(cat: Category) {
  try {
    await showDialog({
      title: '确认删除',
      message: `确定删除科目「${cat.name}」吗？此操作不可撤销。`,
      showCancelButton: true,
    })
  } catch {
    return // 用户取消
  }
  try {
    await api.del(`/categories/${cat.id}`)
    showToast('删除成功')
    await loadCategories()
  } catch (e: any) {
    showToast(e.message || '删除失败')
  }
}
</script>

<style scoped>
.dialog-tip {
  margin: 12px 16px 0;
  font-size: 12px;
  color: #8f8e88;
  line-height: 1.5;
}

.categories-page {
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

.category-tree {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.l1-group {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}

.l1-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #f0f0eb;
}

.l1-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.l1-name {
  font-size: 14px;
  font-weight: 500;
}

.l1-badge {
  font-size: 11px;
  color: #8f8e88;
  border: 1px solid #e3e2dd;
  border-radius: 4px;
  padding: 0 6px;
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
.l1-chip.preset { border-color: #8a6d1f; color: #8a6d1f; background: #fbf3df; }

.l1-actions {
  display: flex;
  gap: 4px;
}

.l2-list {
  padding: 8px 16px;
}

.l2-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid #f0f0eb;
}

.l2-row:last-child {
  border-bottom: none;
}

.l2-row.inactive {
  opacity: 0.6;
}

.l2-info {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.l2-name {
  font-size: 14px;
}

.l2-chip {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #e3e2dd;
  color: #8f8e88;
}

.l2-chip.residual { border-color: #0f6e56; color: #0f6e56; }
.l2-chip.spending { border-color: #185fa5; color: #185fa5; }
.l2-chip.preset { border-color: #8a6d1f; color: #8a6d1f; background: #fbf3df; }
.l2-chip.equity { border-color: #0f6e56; color: #0f6e56; background: #eaf5ed; }
.l2-chip.asset { border-color: #7a4f0f; color: #7a4f0f; background: #fdf3e3; }
.l2-chip.reconcile { border-color: #185fa5; color: #185fa5; background: #e6f1fb; }
.l2-chip.stopped { background: #fcebeb; border-color: #a32d2d; color: #a32d2d; }

.l2-balance {
  font-size: 12px;
  color: #2c2c2a;
  font-weight: 500;
}

.l2-opening {
  font-size: 11px;
  color: #8f8e88;
}

.l2-count {
  font-size: 11px;
  color: #8f8e88;
}

.l2-actions {
  display: flex;
  gap: 4px;
}

.page-footer-note {
  text-align: center;
  font-size: 12px;
  color: #8f8e88;
  padding: 16px;
}

.field-hint {
  font-size: 12px;
  color: #8f8e88;
  padding: 8px 16px;
}

.unit-quick {
  color: #0f6e56;
  font-size: 13px;
  white-space: nowrap;
}

/* 转账弹出表单 */
.transfer-popup {
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

.transfer-hint {
  font-size: 12px;
  color: #0f6e56;
  padding: 0 16px 8px;
}

.transfer-legs-section {
  margin: 8px 0;
}

.legs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
}

.legs-title {
  font-size: 14px;
  font-weight: 500;
  color: #2c2c2a;
}

.leg-row {
  background: #f7f7f5;
  margin: 0 12px 8px;
  border-radius: 8px;
  padding: 4px 0;
}

.leg-row-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 12px 0;
}

.leg-label {
  font-size: 12px;
  color: #8f8e88;
}

.transfer-total-check {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 8px 16px;
  font-size: 13px;
  color: #5f5e5a;
}

.check-error {
  color: #a32d2d;
  font-weight: 500;
}

.transfer-tags {
  padding: 4px 16px 12px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.transfer-save {
  margin: 8px 16px 0;
}

.header-actions {
  display: flex;
  gap: 4px;
}

/* 资金划转弹层 */
.fundmove-popup {
  padding: 16px 0 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.fundmove-save {
  margin: 8px 16px 0;
}

.fundmove-records {
  margin-top: 16px;
  border-top: 1px solid #f0f0eb;
  padding-top: 4px;
}

.records-empty {
  text-align: center;
  color: #8f8e88;
  font-size: 12px;
  padding: 16px 0;
}

.record-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  border-bottom: 1px solid #f0f0eb;
}

.record-row.voided {
  opacity: 0.6;
}

.record-info {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.record-kind {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #e3e2dd;
}

.record-kind.invest { border-color: #7a4f0f; color: #7a4f0f; background: #fdf3e3; }
.record-kind.recover { border-color: #0f6e56; color: #0f6e56; background: #eaf5ed; }

.record-cat {
  font-size: 13px;
  color: #2c2c2a;
}

.record-amount {
  font-size: 13px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.record-amount.invest { color: #7a4f0f; }
.record-amount.recover { color: #0f6e56; }

.record-date {
  font-size: 11px;
  color: #8f8e88;
}
</style>