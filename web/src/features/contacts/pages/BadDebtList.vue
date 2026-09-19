<template>
  <div class="bd-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">坏账清单</h2>
        <p class="page-sub">已核销为坏账的应收单，共 {{ rows.length }} 笔 · 坏账合计 {{ fmt(totalWriteoff) }}</p>
      </div>
      <van-button size="small" plain @click="goBack">返回</van-button>
    </div>

    <div class="bd-filter-bar">
      <label class="bd-filter">
        <span>往来单位</span>
        <select v-model.number="filterPartyId" class="bd-select">
          <option :value="0">全部单位</option>
          <option v-for="p in parties" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </label>
      <label class="bd-filter">
        <span>年度</span>
        <select v-model="filterYear" class="bd-select">
          <option value="">全部年度</option>
          <option v-for="y in years" :key="y" :value="String(y)">{{ y }}年</option>
          <option value="none">无年度</option>
        </select>
      </label>
      <label class="bd-filter">
        <span>应收类别</span>
        <select v-model="filterKind" class="bd-select">
          <option value="">全部类别</option>
          <option v-for="k in kindOptions" :key="k" :value="k">{{ recvKindLabel[k] }}</option>
        </select>
      </label>
    </div>

    <van-loading v-if="loading && rows.length === 0" />
    <van-empty v-else-if="rows.length === 0" description="暂无坏账记录" />

    <div v-else class="bd-list">
      <div v-for="r in rows" :key="r.id" class="bd-card">
        <div class="bd-card-head">
          <span class="bd-card-title">{{ r.partyName }}</span>
          <span class="bd-chip">{{ recvKindLabel[r.recvKind] }}</span>
          <span class="bd-chip">{{ r.recvYear ? r.recvYear + '年' : '无年度' }}</span>
          <span class="bd-chip bad">坏账</span>
        </div>
        <div class="bd-card-body">
          <div class="bd-line">
            <span class="bd-k">事由</span><span class="bd-v">{{ r.title }}</span>
          </div>
          <div class="bd-line">
            <span class="bd-k">应收</span><span class="bd-v">{{ fmt(r.amountCents) }}</span>
            <span class="bd-k">已收</span><span class="bd-v">{{ fmt(cashPaid(r)) }}</span>
            <span class="bd-k">坏账</span><span class="bd-v bd-bad">{{ fmt(r.writeoffCents || 0) }}</span>
          </div>
          <div class="bd-line">
            <span class="bd-k">核销日期</span><span class="bd-v">{{ r.writeoffDate || '—' }}</span>
          </div>
          <div class="bd-line">
            <span class="bd-k">坏账原因</span><span class="bd-v">{{ r.writeoffNote || '—' }}</span>
          </div>
        </div>
        <div class="bd-card-actions">
          <van-button
            size="mini"
            plain
            type="warning"
            :loading="voidingId === r.id"
            @click="revoke(r)"
          >撤销坏账</van-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../../lib/http'
import { showToast, showConfirmDialog } from 'vant'
import { formatFen, recvKindLabel } from '../../../types/api'
import type { ApiResponse, Party, Receivable, ReceivableListResponse, RecvKind } from '../../../types/api'

const router = useRouter()
const kindOptions: RecvKind[] = ['rent', 'dividend', 'service', 'reinvest_dividend', 'other']

const loading = ref(false)
const voidingId = ref<number | null>(null)
const parties = ref<Party[]>([])
const allBadDebts = ref<Receivable[]>([])

const filterPartyId = ref(0)
const filterYear = ref('')
const filterKind = ref('')

function extractItems(res: any): any[] {
  return res?.data?.items || res?.items || (Array.isArray(res?.data) ? res.data : []) || []
}

const cashPaid = (r: Receivable) => Math.max(0, (r.paidCents || 0) - (r.writeoffCents || 0))

const years = computed(() => {
  const set = new Set<number>()
  for (const r of allBadDebts.value) if (r.recvYear) set.add(r.recvYear)
  return Array.from(set).sort((a, b) => b - a)
})

const rows = computed(() => allBadDebts.value.filter((r) => {
  if (filterPartyId.value && r.partyId !== filterPartyId.value) return false
  if (filterYear.value === 'none') { if (r.recvYear) return false }
  else if (filterYear.value && r.recvYear !== Number(filterYear.value)) return false
  if (filterKind.value && r.recvKind !== filterKind.value) return false
  return true
}))

const totalWriteoff = computed(() => rows.value.reduce((s, r) => s + (r.writeoffCents || 0), 0))

const fmt = (c: number) => '¥' + (c / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })

function goBack() {
  if (window.history.length > 1) router.back()
  else router.push('/contacts/receivables/rent')
}

async function load() {
  loading.value = true
  try {
    const [pr, rr] = await Promise.all([
      api.get<ApiResponse<Party[]>>('/parties'),
      api.get<ApiResponse<ReceivableListResponse>>('/receivables', { pageSize: 500 }),
    ])
    parties.value = pr.data || []
    allBadDebts.value = extractItems(rr).filter((r: any) => (r.writeoffCents || 0) > 0)
  } catch (e: any) {
    showToast(e.message || '加载失败')
    allBadDebts.value = []
  } finally {
    loading.value = false
  }
}

async function revoke(r: Receivable) {
  if (!r.writeoffReceiptId) {
    showToast('未找到对应核销记录，无法撤销')
    return
  }
  try {
    await showConfirmDialog({
      title: '撤销坏账核销',
      message: `将撤销「${r.partyName} · ${r.title}」的坏账核销，该应收单退回未结清、待收恢复 ${fmt(r.writeoffCents || 0)}。`,
      confirmButtonText: '确认撤销',
    })
  } catch {
    return
  }
  voidingId.value = r.id
  try {
    await api.put(`/receipts/${r.writeoffReceiptId}`, { status: 'voided' })
    showToast('已撤销，该应收已回到正常列表')
    await load()
  } catch (e: any) {
    showToast(e.message || '撤销失败')
  } finally {
    voidingId.value = null
  }
}

onMounted(load)
</script>

<style scoped>
.bd-page { padding-bottom: 24px; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 14px; }
.page-title { font-size: 18px; font-weight: 600; margin: 0; }
.page-sub { font-size: 12px; color: #969799; margin: 4px 0 0; }

.bd-filter-bar { display: flex; flex-wrap: wrap; gap: 14px; margin-bottom: 14px; }
.bd-filter { display: inline-flex; align-items: center; gap: 8px; font-size: 13px; color: #646566; }
.bd-select {
  height: 34px; border: 1px solid #dcdee0; border-radius: 8px; font-size: 14px;
  padding: 0 8px; background: #fff; color: #323233; min-width: 120px;
}

.bd-list { display: flex; flex-direction: column; gap: 12px; }
.bd-card { background: #fff; border: 1px solid var(--line-soft, #eaeaea); border-radius: 10px; padding: 12px 14px; }
.bd-card-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.bd-card-title { font-size: 15px; font-weight: 600; color: #1f2329; }
.bd-chip {
  font-size: 11px; padding: 1px 8px; border-radius: 999px;
  background: #f2f3f5; color: #646566;
}
.bd-chip.bad { background: #fdf3e3; color: var(--warn); }

.bd-card-body { display: flex; flex-direction: column; gap: 4px; }
.bd-line { display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px; font-size: 13px; color: #323233; }
.bd-k { font-size: 12px; color: #969799; }
.bd-v { margin-right: 10px; font-variant-numeric: tabular-nums; }
.bd-bad { color: var(--warn); font-weight: 600; }

.bd-card-actions { display: flex; justify-content: flex-end; margin-top: 10px; }

@media (max-width: 800px) {
  .bd-filter { width: 100%; }
  .bd-select { flex: 1; }
}
</style>
