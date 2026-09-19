<template>
  <van-popup v-model:show="visible" :position="popupPos()" round closeable style="max-height: 90vh">
    <div class="oplog-popup">
      <div class="oplog-header-row">
        <div>
          <div class="popup-title">操作日志</div>
          <div class="oplog-hint">最近 48 小时内的操作记录</div>
        </div>
        <div class="oplog-actions">
          <van-button v-if="logs.length > 0" size="mini" plain @click="exportCsv">导出 CSV</van-button>
          <van-button v-if="logs.length > 0" size="mini" plain class="oplog-clear-btn" @click="clearLogs">清除日志</van-button>
        </div>
      </div>

      <!-- 加载中 -->
      <div v-if="loading" class="oplog-loading">
        <van-loading size="20" /> 加载中...
      </div>

      <!-- 空状态 -->
      <div v-else-if="logs.length === 0" class="oplog-empty">
        暂无操作记录
      </div>

      <!-- 日志列表 -->
      <div v-else class="oplog-list">
        <div
          v-for="log in logs"
          :key="log.id"
          class="oplog-item"
          :class="{ expanded: expandedId === log.id }"
        >
          <div class="oplog-header" @click="toggleExpand(log.id)">
            <div class="oplog-meta">
              <span class="oplog-op">{{ log.operation }}</span>
              <span class="oplog-time">{{ formatDateTime(log.time) }}</span>
            </div>
            <div class="oplog-summary">{{ log.summary }}</div>
            <div class="oplog-expand-icon">{{ expandedId === log.id ? '▲' : '▼' }}</div>
          </div>
          <div v-if="expandedId === log.id && log.effects.length > 0" class="oplog-effects">
            <div class="oplog-section-title">受影响的数据：</div>
            <div v-for="(eff, ei) in log.effects" :key="ei" class="oplog-effect">
              <span class="oplog-effect-entity">{{ eff.entity }}</span>
              <template v-if="eff.field">
                <span class="oplog-effect-field">{{ eff.field }}</span>
                <template v-if="eff.oldValue !== undefined && eff.oldValue !== null">
                  <span class="oplog-effect-arrow">→</span>
                  <span class="oplog-effect-new">{{ eff.newValue }}</span>
                </template>
                <template v-else-if="eff.newValue !== undefined && eff.newValue !== null">
                  <span class="oplog-effect-new">{{ eff.newValue }}</span>
                </template>
              </template>
              <div v-if="eff.desc" class="oplog-effect-desc">{{ eff.desc }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </van-popup>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { api } from '../../lib/http'
import { showConfirmDialog } from 'vant'
import { popupPos } from '../../composables/useScreen'

interface OpLogEffect {
  entity: string
  entityId?: number | null
  field?: string | null
  oldValue?: string | null
  newValue?: string | null
  desc: string
}

interface OpLog {
  id: number
  time: string
  operation: string
  summary: string
  effects: OpLogEffect[]
}

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const visible = ref(false)
watch(() => props.show, (v) => {
  visible.value = v
  if (v) fetchLogs()
})
watch(visible, (v) => emit('update:show', v))

const logs = ref<OpLog[]>([])
const loading = ref(false)
const expandedId = ref<number | null>(null)

function toggleExpand(id: number) {
  expandedId.value = expandedId.value === id ? null : id
}

async function fetchLogs() {
  loading.value = true
  try {
    const res = await api.get('/operation-logs')
    const data = res.data || res
    logs.value = data.items || []
  } catch {
    logs.value = []
  } finally {
    loading.value = false
  }
}

async function clearLogs() {
  try {
    await showConfirmDialog({
      title: '清除日志',
      message: '确定清空当前组织的全部操作日志吗？此操作不可恢复。',
      confirmButtonText: '确认清除',
    })
  } catch {
    return // 用户取消
  }
  try {
    await api.del('/operation-logs')
    logs.value = []
    expandedId.value = null
  } catch {
    logs.value = []
  }
}

function formatDateTime(iso: string) {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return d.getFullYear() + '-' + pad(d.getMonth()+1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds())
}

function escapeCsv(v: string) {
  if (v == null) return ''
  const s = String(v)
  if (s.includes(',') || s.includes('"') || s.includes('\n')) {
    return '"' + s.replace(/"/g, '""') + '"'
  }
  return s
}

function exportCsv() {
  const rows: string[] = []
  const header = ['时间', '操作', '摘要', '影响实体', '字段', '旧值', '新值', '影响描述']
  rows.push(header.map(escapeCsv).join(','))

  for (const log of logs.value) {
    if (log.effects.length === 0) {
      rows.push([formatDateTime(log.time), log.operation, log.summary, '', '', '', ''].map(escapeCsv).join(','))
    } else {
      for (const eff of log.effects) {
        rows.push([
          formatDateTime(log.time),
          log.operation,
          log.summary,
          eff.entity + (eff.entityId ? ' #' + eff.entityId : ''),
          eff.field || '',
          eff.oldValue != null ? String(eff.oldValue) : '',
          eff.newValue != null ? String(eff.newValue) : '',
          eff.desc || '',
        ].map(escapeCsv).join(','))
      }
    }
  }

  const bom = '\uFEFF'
  const blob = new Blob([bom + rows.join('\n')], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = '操作日志_' + new Date().toISOString().slice(0, 10) + '.csv'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.oplog-popup {
  width: 90vw;
  max-width: 520px;
  padding: 20px 16px 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.oplog-header-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
}

.popup-title {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 4px;
}

.oplog-hint {
  font-size: 12px;
  color: #999;
}

.oplog-clear-btn {
  margin-left: 8px;
}

.oplog-actions {
  display: flex;
  align-items: center;
  margin-right: 40px;
  padding-top: 22px;
}

.oplog-loading,
.oplog-empty {
  text-align: center;
  padding: 32px 0;
  color: #999;
  font-size: 14px;
}

.oplog-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.oplog-item {
  background: #f8f8f8;
  border-radius: 8px;
  overflow: hidden;
}

.oplog-header {
  padding: 10px 12px;
  cursor: pointer;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 8px;
  user-select: none;
}

.oplog-header:hover {
  background: #f0f0f0;
}

.oplog-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.oplog-op {
  font-weight: 600;
  font-size: 14px;
  color: #333;
  white-space: nowrap;
}

.oplog-time {
  font-size: 12px;
  color: #999;
  white-space: nowrap;
}

.oplog-summary {
  font-size: 12px;
  color: #666;
  width: 100%;
  padding-left: 0;
}

.oplog-expand-icon {
  font-size: 10px;
  color: #bbb;
  flex-shrink: 0;
}

.oplog-effects {
  padding: 0 12px 10px;
  border-top: 1px solid #eee;
  padding-top: 8px;
  margin-top: 0;
}

.oplog-section-title {
  font-size: 12px;
  color: #999;
  margin-bottom: 6px;
}

.oplog-effect {
  font-size: 13px;
  padding: 4px 0;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px;
}

.oplog-effect-entity {
  color: var(--info);
  font-weight: 500;
  margin-right: 4px;
}

.oplog-effect-field {
  color: #666;
  font-size: 12px;
}

.oplog-effect-arrow {
  color: #ccc;
  margin: 0 2px;
}

.oplog-effect-new {
  color: #333;
  font-weight: 500;
}

.oplog-effect-desc {
  width: 100%;
  font-size: 12px;
  color: #888;
  padding-left: 0;
  margin-top: 1px;
}
</style>