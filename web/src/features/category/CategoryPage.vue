<template>
  <div class="categories-page">
    <div class="page-header">
      <h3>科目管理</h3>
      <div class="header-actions">
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
            <van-button v-if="!l1.preset" size="mini" plain @click="renameCat(l1)">重命名</van-button>
            <van-button v-if="!l1.preset && !l1.children?.length" size="mini" plain type="danger" @click="deleteCat(l1)">删除</van-button>
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
              <van-button v-if="!l2.preset" size="mini" plain @click="renameCat(l2)">重命名</van-button>
              <van-button
                size="mini"
                plain
                :type="l2.status === 'active' ? 'warning' : 'primary'"
                @click="toggleStatus(l2)"
              >{{ l2.status === 'active' ? '停用' : '启用' }}</van-button>
              <van-button v-if="!l2.preset" size="mini" plain type="danger" @click="deleteCat(l2)">删除</van-button>
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
      <div class="dialog-tip">一级科目用于分组，二级科目记录收支去向</div>
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
      <div v-if="!editL2Mode" class="field-hint">
        普通收支科目：收入/支出都记在此；对外投资给公司的，把公司建在「长期投资」分组下
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
import NativeSelect from '../../components/NativeSelect.vue'
import type { Category, Party, ApiResponse } from '../../types/api'
import { formatFen } from '../../types/api'

import { popupPos } from '../../composables/useScreen';
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

function onUnitSelect(action: { name: string;
value: number }) {
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
  color: var(--ink-muted);
  line-height: 1.5;
}

.categories-page {
  padding: 16px;
  padding-bottom: 60px;
  min-height: 100vh;
  background: var(--paper);
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
  color: var(--ink);
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
  color: var(--ink-muted);
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
  border-bottom: 1px solid var(--line-soft);
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
  color: var(--ink-muted);
  border: 1px solid #e3e2dd;
  border-radius: 4px;
  padding: 0 6px;
}

.l1-chip {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #e3e2dd;
  color: var(--ink-muted);
}

.l1-chip.residual { border-color: var(--jade); color: var(--jade); }
.l1-chip.spending { border-color: var(--indigo); color: var(--indigo); }
.l1-chip.preset { border-color: var(--warn); color: var(--warn); background: #fbf3df; }

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
  border-bottom: 1px solid var(--line-soft);
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
  color: var(--ink-muted);
}

.l2-chip.residual { border-color: var(--jade); color: var(--jade); }
.l2-chip.spending { border-color: var(--indigo); color: var(--indigo); }
.l2-chip.preset { border-color: var(--warn); color: var(--warn); background: #fbf3df; }
.l2-chip.equity { border-color: var(--jade); color: var(--jade); background: var(--jade-light); }
.l2-chip.asset { border-color: var(--warn); color: var(--warn); background: #fdf3e3; }
.l2-chip.reconcile { border-color: var(--indigo); color: var(--indigo); background: var(--indigo-light); }
.l2-chip.stopped { background: #fcebeb; border-color: var(--expense); color: var(--expense); }

.l2-balance {
  font-size: 12px;
  color: var(--ink);
  font-weight: 500;
}

.l2-opening {
  font-size: 11px;
  color: var(--ink-muted);
}

.l2-count {
  font-size: 11px;
  color: var(--ink-muted);
}

.l2-actions {
  display: flex;
  gap: 4px;
}

.page-footer-note {
  text-align: center;
  font-size: 12px;
  color: var(--ink-muted);
  padding: 16px;
}

.field-hint {
  font-size: 12px;
  color: var(--ink-muted);
  padding: 8px 16px;
}

.unit-quick {
  color: var(--jade);
  font-size: 13px;
  white-space: nowrap;
}

.header-actions {
  display: flex;
  gap: 4px;
}

/* 桌面端（≥800px）按钮放大，改善可点击性 */
@media (min-width: 800px) {
  .categories-page {
    padding: 20px 28px;
    padding-bottom: 80px;
  }

  .page-header h3 {
    font-size: 20px;
    font-weight: 700;
    font-family: 'Noto Serif SC', serif;
    letter-spacing: -.01em;
  }

  .header-actions {
    gap: 10px;
  }
  .header-actions .van-button--small {
    height: 36px !important;
    padding: 0 20px !important;
    font-size: 14px !important;
  }

  .l1-row {
    padding: 16px 24px;
  }
  .l1-info { gap: 12px; }
  .l1-name { font-size: 16px; font-weight: 600; }
  .l1-chip, .l1-badge { font-size: 12px; padding: 2px 10px; }

  .l1-actions { gap: 8px; }
  .l1-actions .van-button {
    height: 34px !important;
    padding: 0 16px !important;
    font-size: 13px !important;
    border-radius: var(--r-sm) !important;
  }

  .l2-list { padding: 8px 24px; }
  .l2-row { padding: 12px 0; }
  .l2-info { gap: 8px; }
  .l2-name { font-size: 15px; font-weight: 500; }
  .l2-chip { font-size: 12px; padding: 2px 10px; }
  .l2-balance, .l2-opening, .l2-count { font-size: 13px; }

  .l2-actions { gap: 8px; }
  .l2-actions .van-button {
    height: 32px !important;
    padding: 0 14px !important;
    font-size: 12px !important;
    border-radius: var(--r-sm) !important;
  }
}
</style>