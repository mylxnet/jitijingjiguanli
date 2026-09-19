<template>
  <van-popup v-model:show="innerShow" :position="popupPos()" round closeable style="max-height: 70vh">
    <div class="changelog-popup">
      <div class="popup-title">变更历史</div>
      <div v-if="loading" class="changelog-empty">加载中…</div>
      <div v-else-if="items.length === 0" class="changelog-empty">暂无变更记录</div>
      <div v-else class="changelog-list">
        <div v-for="log in items" :key="log.id" class="changelog-item">
          <div class="changelog-main">
            <span class="changelog-action">{{ actionText(log.action) }}</span>
            <span v-if="log.field" class="changelog-field">{{ fieldText(log.field) }}</span>
          </div>
          <div v-if="log.oldValue != null && log.newValue != null" class="changelog-diff">
            {{ oldText(log) }} → {{ newText(log) }}
          </div>
          <div class="changelog-time">{{ formatTime(log.changedAt) }}</div>
        </div>
      </div>
    </div>
  </van-popup>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { api } from '../lib/http'
import type { ApiResponse } from '../types/api'

import { popupPos } from '../composables/useScreen';
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

const props = defineProps<{
  show: boolean
  entityType: string
  entityId: number
}>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const innerShow = ref(props.show)
watch(
  () => props.show,
  v => {
    innerShow.value = v
    if (v && props.entityId > 0) load()
  },
)
watch(innerShow, v => emit('update:show', v))

const items = ref<ChangeLogItem[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  items.value = []
  try {
    const res = await api.get<ApiResponse<ChangeLogItem[]>>('/changelog', {
      entityType: props.entityType,
      entityId: props.entityId,
    })
    items.value = res.data || []
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

const actionMap: Record<string, string> = {
  create: '创建',
  update: '修改',
  void: '作废',
  unvoid: '撤销作废',
  status: '状态变更',
}

const fieldMap: Record<string, string> = {
  name: '名称',
  status: '状态',
  balance_type: '余额类型',
  opening_balance_cents: '期初余额',
  include_in_reconciliation: '参与勾稽',
  txn_date: '日期',
  direction: '方向',
  amount_cents: '金额',
  category_id: '科目',
  note: '摘要',
  kind: '类别',
  recv_kind: '应收类别',
  title: '事由',
  method: '核销方式',
}

function actionText(a: string): string {
  return actionMap[a] || a
}

function fieldText(f: string): string {
  return fieldMap[f] || f
}

function oldText(log: ChangeLogItem): string {
  if (log.field === 'direction') return log.oldValue === 'income' ? '收入' : '支出'
  if (log.field === 'status') return statusText(log.oldValue)
  return log.oldValue || ''
}

function newText(log: ChangeLogItem): string {
  if (log.field === 'direction') return log.newValue === 'income' ? '收入' : '支出'
  if (log.field === 'status') return statusText(log.newValue)
  return log.newValue || ''
}

function statusText(s: string | null): string {
  const map: Record<string, string> = {
    active: '启用',
    inactive: '停用',
    normal: '正常',
    voided: '已作废',
    open: '未结清',
    closed: '已结清',
    residual: '余粮型',
    spending: '花费型',
    household: '农户',
    unit: '单位',
    cash: '现金',
    offset: '抵销',
    writeoff: '坏账',
    invest: '投资',
    recover: '收回',
    income: '收入',
    expense: '支出',
  }
  return (s && map[s]) || s || ''
}

function formatTime(s: string): string {
  return s ? s.replace('T', ' ').slice(0, 19) : s
}
</script>

<style scoped>
.changelog-popup {
  padding: 16px 0 24px;
max-height: 70vh;
  overflow-y: auto;
}

.popup-title {
  font-size: 16px;
  font-weight: 500;
  color: var(--ink);
  padding: 0 16px 12px;
}

.changelog-empty {
  text-align: center;
  color: var(--ink-muted);
  font-size: 13px;
  padding: 20px 0;
}

.changelog-list {
  padding: 0 16px;
}

.changelog-item {
  padding: 8px 0;
  border-bottom: 1px solid var(--line-soft);
}

.changelog-item:last-child {
  border-bottom: none;
}

.changelog-main {
  display: flex;
  gap: 8px;
  align-items: center;
}

.changelog-action {
  font-size: 13px;
  color: var(--info-deep);
  background: var(--info-bg);
  border-radius: 4px;
  padding: 1px 6px;
}

.changelog-field {
  font-size: 12px;
  color: var(--ink-muted);
}

.changelog-diff {
  font-size: 13px;
  color: var(--ink);
  margin-top: 2px;
  word-break: break-all;
}

.changelog-time {
  font-size: 11px;
  color: var(--ink-muted);
  margin-top: 2px;
}
</style>
