<template>
  <van-popup
    v-model:show="visible"
    :position="popupPos()"
    :round="true"
    closeable
    :style="isDesktop ? 'width:680px;border-radius:12px;max-height:88vh;overflow:auto' : 'max-height:92vh'"
  >
    <div class="dp-popup">
      <div class="popup-title">删除往来单位</div>
      <div class="dp-tip">
        仅可删除<b>无欠款</b>（含坏账核销已结清）且<b>名下科目余额为 0</b>的单位。
        删除会一并清除该单位的应收单与核销记录、计提标准、合同、再投资去向；
        其历史流水不会丢失，将归档到「历史归档」科目。
      </div>

      <div class="dp-filter">
        <van-checkbox v-model="onlyDeletable" shape="square">只看可删除的单位</van-checkbox>
        <van-button size="mini" plain @click="load">刷新</van-button>
      </div>

      <div class="dp-list">
        <div v-if="loading" class="dp-empty">加载中…</div>
        <div v-else-if="rows.length === 0" class="dp-empty">没有符合条件的单位</div>
        <template v-else>
          <div
            v-for="p in rows"
            :key="p.id"
            class="dp-row"
            :class="{ active: selected && selected.id === p.id, blocked: !p.deletable }"
            @click="selectRow(p)"
          >
            <div class="dp-row-main">
              <div class="dp-row-title">
                {{ p.name }}
                <span class="dp-chip">{{ typeLabel(p.type) }}</span>
              </div>
              <div class="dp-row-sub">
                欠款 {{ formatFen(p.outstandingCents || 0) }}
                <template v-if="p.deletable">
                  <span class="dp-ok">· 可删除</span>
                </template>
                <template v-else>
                  <span class="dp-no">· {{ p.deleteBlockReason || '不可删除' }}</span>
                </template>
              </div>
            </div>
          </div>
        </template>
      </div>

      <template v-if="selected">
        <van-cell-group inset style="margin-top:12px">
          <van-cell title="单位" :value="selected.name" />
          <van-cell title="类型" :value="typeLabel(selected.type)" />
          <van-cell title="欠款" :value="formatFen(selected.outstandingCents || 0)" />
        </van-cell-group>
        <div class="dp-confirm">
          <van-checkbox v-model="confirmed" shape="square">
            我确认删除「{{ selected.name }}」及其应收/核销/合同等关联数据
          </van-checkbox>
        </div>
        <div v-if="error" class="dp-error">{{ error }}</div>
        <div class="dp-actions">
          <van-button
            round
            block
            type="danger"
            :disabled="!confirmed"
            :loading="deleting"
            @click="confirmDelete"
          >删除该单位</van-button>
        </div>
      </template>

      <!-- 删除结果 -->
      <van-dialog v-model:show="showResult" title="已删除" :show-confirm-button="true" confirm-button-text="知道了">
        <div class="dp-result">
          <p>单位「{{ result?.partyName }}」已删除。</p>
          <ul>
            <li>删除科目 {{ result?.categoriesDeleted || 0 }} 个</li>
            <li>归档历史流水 {{ result?.txnsMigrated || 0 }} 条</li>
            <li>删除应收单 {{ result?.receivablesDeleted || 0 }} 张</li>
            <li>删除核销记录 {{ result?.receiptsDeleted || 0 }} 条</li>
            <li>删除计提标准 {{ result?.standardsDeleted || 0 }} 条</li>
            <li>删除合同 {{ result?.contractsDeleted || 0 }} 份</li>
            <li>删除再投资去向 {{ result?.allocationsDeleted || 0 }} 条</li>
          </ul>
        </div>
      </van-dialog>
    </div>
  </van-popup>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { api } from '../../lib/http'
import { showToast, showConfirmDialog } from 'vant'
import { popupPos, useScreen } from '../../composables/useScreen'
import { formatFen } from '../../types/api'
import type { ApiResponse, Party } from '../../types/api'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'done'): void }>()

const { isDesktop } = useScreen()

const visible = ref(false)
watch(() => props.show, (v) => {
  visible.value = v
  if (v) openDialog()
})
watch(visible, (v) => emit('update:show', v))

const loading = ref(false)
const deleting = ref(false)
const error = ref('')
const onlyDeletable = ref(true)
const parties = ref<Party[]>([])
const selected = ref<Party | null>(null)
const confirmed = ref(false)
const showResult = ref(false)
const result = ref<any>(null)

const typeLabel = (t?: string | null) => ({
  flow: '流转企业', invest: '投资公司', reinvest: '再投资', other: '其他',
}[t || ''] || '其他')

const rows = computed(() => parties.value.filter(p => (onlyDeletable.value ? p.deletable : true)))

async function openDialog() {
  error.value = ''
  selected.value = null
  confirmed.value = false
  onlyDeletable.value = true
  await load()
}

async function load() {
  loading.value = true
  try {
    const res = await api.get<ApiResponse<Party[]>>('/parties')
    parties.value = res.data || []
    selected.value = null
    confirmed.value = false
  } catch (e: any) {
    parties.value = []
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function selectRow(p: Party) {
  if (!p.deletable) {
    showToast(p.deleteBlockReason || '该单位不可删除')
    return
  }
  selected.value = p
  confirmed.value = false
  error.value = ''
}

async function confirmDelete() {
  const p = selected.value
  if (!p || !confirmed.value) return
  try {
    await showConfirmDialog({
      title: '确认删除',
      message: `确定删除「${p.name}」吗？\n\n该单位的应收单与核销记录（含坏账）、计提标准、合同、再投资去向将被清除，历史流水将归档，操作不可撤销。`,
      confirmButtonText: '确认删除',
      confirmButtonColor: 'var(--danger, #a33a2d)',
    })
  } catch {
    return
  }
  deleting.value = true
  error.value = ''
  try {
    const res: any = await api.del(`/parties/${p.id}`)
    result.value = res?.data || res
    showResult.value = true
    showToast('已删除')
    emit('done')
    await load()
  } catch (e: any) {
    error.value = e.message || '删除失败'
  } finally {
    deleting.value = false
  }
}
</script>

<style scoped>
.dp-popup { padding: 16px 0 24px; max-height: 88vh; overflow-y: auto; }
.popup-title { font-size: 16px; font-weight: 500; padding: 0 16px 6px; }
.dp-tip { padding: 0 16px 10px; font-size: 12px; color: var(--ink-muted); line-height: 1.5; }
.dp-filter { display: flex; align-items: center; justify-content: space-between; padding: 0 16px 8px; }
.dp-list { margin: 0 16px; border: 1px solid var(--line); border-radius: 10px; max-height: 34vh; overflow-y: auto; }
.dp-row { padding: 10px 12px; border-bottom: 1px solid var(--line-soft); cursor: pointer; }
.dp-row:last-child { border-bottom: none; }
.dp-row.active { background: var(--jade-light); }
.dp-row.blocked { cursor: not-allowed; opacity: 0.75; }
.dp-row-title { font-size: 14px; color: var(--ink); display: flex; align-items: center; gap: 8px; }
.dp-chip { font-size: 11px; padding: 1px 8px; border-radius: 999px; background: var(--paper-deep); color: var(--ink-muted); }
.dp-row-sub { margin-top: 3px; font-size: 12px; color: var(--ink-muted); }
.dp-ok { color: var(--success); }
.dp-no { color: var(--danger); }
.dp-empty { padding: 24px 12px; text-align: center; font-size: 13px; color: var(--ink-muted); }
.dp-confirm { padding: 10px 16px 0; font-size: 13px; }
.dp-error { color: var(--danger); font-size: 13px; padding: 8px 16px 0; }
.dp-actions { margin: 12px 16px 0; }
.dp-result { padding: 8px 16px 16px; font-size: 13px; color: var(--ink); }
.dp-result ul { margin: 8px 0 0; padding-left: 18px; }
.dp-result li { line-height: 1.9; }
</style>
