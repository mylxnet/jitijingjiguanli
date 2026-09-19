<template>
  <van-popup
    v-model:show="visible"
    :position="popupPos()"
    :round="true"
    closeable
    :style="isDesktop ? 'width:680px;border-radius:12px;max-height:88vh;overflow:auto' : 'max-height:92vh'"
  >
    <div class="bd-popup">
      <div class="popup-title">坏账核销</div>
      <div class="bd-tip">
        把收不回的应收款「剩余待收」全额清零。坏账不产生任何流水，
        <b>银行存款与科目余额不受影响</b>；操作可撤销。
        <br />仅可核销<b>本年度以前</b>的应收（含未填年度的单）。
      </div>

      <!-- 筛选 -->
      <van-cell-group inset>
        <van-field label="往来单位">
          <template #input>
            <select v-model.number="filterPartyId" class="bd-select">
              <option :value="0">全部单位</option>
              <option v-for="p in parties" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </template>
        </van-field>
        <van-field label="年度">
          <template #input>
            <select v-model.number="filterYear" class="bd-select">
              <option :value="0">全部年度</option>
              <option v-for="y in years" :key="y" :value="y">{{ y }}年</option>
            </select>
          </template>
        </van-field>
        <van-field label="应收类别">
          <template #input>
            <select v-model="filterKind" class="bd-select">
              <option value="">全部类别</option>
              <option v-for="k in kindOptions" :key="k" :value="k">{{ recvKindLabel[k] }}</option>
            </select>
          </template>
        </van-field>
      </van-cell-group>

      <!-- 未结清应收单列表 -->
      <div class="bd-list">
        <div v-if="loading" class="bd-empty">加载中…</div>
        <div v-else-if="filtered.length === 0" class="bd-empty">没有符合条件的未结清应收单</div>
        <template v-else>
          <div
            v-for="r in filtered"
            :key="r.id"
            class="bd-row"
            :class="{ active: selected && selected.id === r.id }"
            @click="selectRow(r)"
          >
            <div class="bd-row-main">
              <div class="bd-row-title">{{ r.partyName }} · {{ recvKindLabel[r.recvKind] }}</div>
              <div class="bd-row-sub">{{ r.title }}<template v-if="r.recvYear">（{{ r.recvYear }}年）</template></div>
            </div>
            <div class="bd-row-nums">
              <span>应收 {{ formatFen(r.amountCents) }}</span>
              <span>已收 {{ formatFen(cashPaid(r)) }}</span>
              <span class="bd-owe">待收 {{ formatFen(r.outstandingCents) }}</span>
            </div>
          </div>
        </template>
      </div>

      <!-- 选中详情 + 原因 -->
      <template v-if="selected">
        <van-cell-group inset style="margin-top:12px">
          <van-cell title="应收金额" :value="formatFen(selected.amountCents)" />
          <van-cell title="已收（现金/抵销）" :value="formatFen(cashPaid(selected))" />
          <van-cell title="待收（将转坏账）" :value="formatFen(selected.outstandingCents)" />
        </van-cell-group>
        <van-field
          v-model="reason"
          label="坏账原因"
          placeholder="必填，如：对方注销 / 无力偿还"
          maxlength="100"
          show-word-limit
        />
        <div v-if="error" class="bd-error">{{ error }}</div>
        <div class="bd-actions">
          <van-button
            round
            block
            type="danger"
            :loading="saving"
            @click="confirmWriteoff"
          >确认核销坏账 {{ formatFen(selected.outstandingCents) }}</van-button>
        </div>
      </template>
    </div>
  </van-popup>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { api } from '../../lib/http'
import { showToast, showConfirmDialog } from 'vant'
import { popupPos, useScreen } from '../../composables/useScreen'
import { formatFen, todayStr, recvKindLabel } from '../../types/api'
import type { ApiResponse, Party, Receivable, ReceivableListResponse, RecvKind } from '../../types/api'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'done'): void }>()

const { isDesktop } = useScreen()

const visible = ref(false)
watch(() => props.show, (v) => {
  visible.value = v
  if (v) openDialog()
})
watch(visible, (v) => emit('update:show', v))

const kindOptions: RecvKind[] = ['rent', 'dividend', 'service', 'reinvest_dividend', 'other']

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const parties = ref<Party[]>([])
const receivables = ref<Receivable[]>([])
const selected = ref<Receivable | null>(null)
const reason = ref('')

const filterPartyId = ref(0)
const filterYear = ref(0)
const filterKind = ref('')

/** 已收 = 全部核销 − 坏账（坏账单会结清，此处通常为 0，仍按口径展示）。 */
function cashPaid(r: Receivable): number {
  return Math.max(0, (r.paidCents || 0) - (r.writeoffCents || 0))
}

// 坏账核销仅限「本年度以前」：往年（recvYear < 当前年）或未填年度（recvYear=0）。
const curYear = new Date().getFullYear()
const eligibleYear = (r: Receivable) => !r.recvYear || r.recvYear < curYear

const years = computed(() => {
  const set = new Set<number>()
  for (const r of receivables.value) if (r.recvYear) set.add(r.recvYear)
  return Array.from(set).sort((a, b) => b - a)
})

const filtered = computed(() => receivables.value.filter((r) => {
  if (!eligibleYear(r)) return false
  if (filterPartyId.value && r.partyId !== filterPartyId.value) return false
  if (filterYear.value && r.recvYear !== filterYear.value) return false
  if (filterKind.value && r.recvKind !== filterKind.value) return false
  return (r.outstandingCents || 0) > 0
}))

async function openDialog() {
  error.value = ''
  reason.value = ''
  selected.value = null
  filterPartyId.value = 0
  filterYear.value = 0
  filterKind.value = ''
  loading.value = true
  try {
    const [pr, rr] = await Promise.all([
      api.get<ApiResponse<Party[]>>('/parties'),
      api.get<ApiResponse<ReceivableListResponse>>('/receivables', { status: 'open', pageSize: 500 }),
    ])
    parties.value = pr.data || []
    // 仅列可核销的：未结清 + 本年度以前（含未填年度）
    receivables.value = (rr.data?.items || []).filter((r) => (r.outstandingCents || 0) > 0 && eligibleYear(r))
  } catch (e: any) {
    receivables.value = []
    parties.value = []
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function selectRow(r: Receivable) {
  selected.value = r
  error.value = ''
}

async function confirmWriteoff() {
  error.value = ''
  const r = selected.value
  if (!r) return
  const note = reason.value.trim()
  if (!note) {
    error.value = '请填写坏账原因'
    return
  }
  try {
    await showConfirmDialog({
      title: '确认坏账核销',
      message: `将把「${r.partyName} · ${r.title}」的剩余待收 ${formatFen(r.outstandingCents)} 全额核销为坏账。\n\n此操作不影响银行存款与科目余额，可在核销记录中撤销。`,
      confirmButtonText: '确认核销',
      confirmButtonColor: 'var(--danger, #a33a2d)',
    })
  } catch {
    return
  }
  saving.value = true
  try {
    await api.post(`/receivables/${r.id}/receipts`, {
      method: 'writeoff',
      receiptDate: todayStr(),
      note,
    })
    showToast('坏账核销成功')
    emit('done')
    visible.value = false
  } catch (e: any) {
    error.value = e.message || '核销失败'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.bd-popup {
  padding: 16px 0 24px;
  max-height: 88vh;
  overflow-y: auto;
}

.popup-title {
  font-size: 16px;
  font-weight: 500;
  padding: 0 16px 6px;
}

.bd-tip {
  padding: 0 16px 12px;
  font-size: 12px;
  color: var(--ink-muted);
  line-height: 1.5;
}

.bd-select {
  flex: 1;
  width: 100%;
  height: 38px;
  border: 1px solid var(--line);
  border-radius: 8px;
  font-size: 15px;
  padding: 0 10px;
  background: #fff;
  color: var(--ink);
}

.bd-list {
  margin: 12px 16px 0;
  border: 1px solid var(--line);
  border-radius: 10px;
  max-height: 34vh;
  overflow-y: auto;
}

.bd-row {
  padding: 10px 12px;
  border-bottom: 1px solid var(--line-soft);
  cursor: pointer;
}

.bd-row:last-child {
  border-bottom: none;
}

.bd-row.active {
  background: var(--jade-light);
}

.bd-row-title {
  font-size: 14px;
  color: var(--ink);
}

.bd-row-sub {
  margin-top: 2px;
  font-size: 12px;
  color: var(--ink-muted);
}

.bd-row-nums {
  margin-top: 6px;
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--ink-soft);
}

.bd-owe {
  color: var(--danger);
}

.bd-empty {
  padding: 24px 12px;
  text-align: center;
  font-size: 13px;
  color: var(--ink-muted);
}

.bd-error {
  color: var(--danger);
  font-size: 13px;
  padding: 8px 16px 0;
}

.bd-actions {
  margin: 12px 16px 0;
}
</style>
