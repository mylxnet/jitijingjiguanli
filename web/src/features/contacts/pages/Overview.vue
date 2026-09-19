<template>
  <div class="ov-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">往来管理概览</h2>
        <p class="page-sub">一站式查看单位、应收、收缴与合同</p>
      </div>
      <van-button type="primary" size="small" @click="$router.push('/contacts/parties')">＋ 新增单位</van-button>
    </div>

    <div class="ov-stats">
      <div class="ov-stat">
        <span class="ov-fi"><van-icon name="shop" /></span>
        <div>
          <div class="ov-stat-label">单位总数</div>
          <div class="ov-stat-value">{{ parties.length }}</div>
        </div>
      </div>
      <div class="ov-stat">
        <span class="ov-fi"><van-icon name="balance-list" /></span>
        <div>
          <div class="ov-stat-label">本年应收合计</div>
          <div class="ov-stat-value">{{ fmtYuan(totalReceivable) }}</div>
        </div>
      </div>
      <div class="ov-stat">
        <span class="ov-fi"><van-icon name="passed" /></span>
        <div>
          <div class="ov-stat-label">本年已收</div>
          <div class="ov-stat-value">{{ fmtYuan(totalPaid) }}</div>
        </div>
      </div>
      <div class="ov-stat">
        <span class="ov-fi"><van-icon name="clock" /></span>
        <div>
          <div class="ov-stat-label">本年未收</div>
          <div class="ov-stat-value">{{ fmtYuan(totalReceivable - totalPaid) }}</div>
        </div>
      </div>
    </div>

    <div class="ov-hub-grid">
      <router-link to="/contacts/parties" class="ov-hub-card">
        <span class="ov-hub-ic blue">🏢</span>
        <div>
          <h3>单位列表</h3>
          <p>长投 / 再投 / 流转 / 其他，按类型筛选</p>
        </div>
        <div class="ov-hub-meta">
          <span class="count">{{ parties.length }}</span>
          <span class="label">个单位</span>
        </div>
      </router-link>
      <router-link to="/contacts/receivables/dividend" class="ov-hub-card">
        <span class="ov-hub-ic green">💹</span>
        <div>
          <h3>应收投资收益</h3>
          <p>长投 & 再投资单位的年度应收收益</p>
        </div>
        <div class="ov-hub-meta">
          <span class="count">{{ dividendCount }}</span>
          <span class="label">笔</span>
        </div>
      </router-link>
      <router-link to="/contacts/receivables/rent" class="ov-hub-card">
        <span class="ov-hub-ic orange">🌾</span>
        <div>
          <h3>应收土地流转费</h3>
          <p>流转企业按亩 × 每亩年流转费</p>
        </div>
        <div class="ov-hub-meta">
          <span class="count">{{ rentCount }}</span>
          <span class="label">笔</span>
        </div>
      </router-link>
      <router-link to="/contacts/receivables/service" class="ov-hub-card">
        <span class="ov-hub-ic purple">📋</span>
        <div>
          <h3>应收管理费</h3>
          <p>流转企业按亩 × 每亩年管理费</p>
        </div>
        <div class="ov-hub-meta">
          <span class="count">{{ serviceCount }}</span>
          <span class="label">笔</span>
        </div>
      </router-link>
      <router-link to="/contacts/contracts" class="ov-hub-card">
        <span class="ov-hub-ic blue">📄</span>
        <div>
          <h3>合同管理</h3>
          <p>所有单位的投资协议、流转合同、附件</p>
        </div>
        <div class="ov-hub-meta">
          <span class="count">{{ contracts.length }}</span>
          <span class="label">份文件</span>
        </div>
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../../lib/http'

// 统一从 API 响应里提取数组（兼容 data / data.items / 直接数组）
function extractList(r: any) {
  if (r == null) return [];
  if (Array.isArray(r)) return r;
  if (Array.isArray(r.data)) return r.data;
  if (r.data && Array.isArray(r.data.items)) return r.data.items;
  return [];
}

interface Party { id: number; name: string; type: string }
interface Receivable { kind: string; recvYear?: number; amountCents: number; paidCents: number }
interface Contract { id: number }

const parties = ref<Party[]>([])
const receivables = ref<Receivable[]>([])
const contracts = ref<Contract[]>([])

const curYear = new Date().getFullYear()

onMounted(async () => {
  try {
    const [p, r, c] = await Promise.all([
      api.get<any>('/parties'),
      api.get<any>('/receivables'),
      api.get<any>('/contracts'),
    ])
    parties.value = extractList(p)
    // 后端字段为 recvKind，前端统一归一化为 kind 使用
    receivables.value = (extractList(r) || []).map((it: any) => ({ ...it, kind: it.recvKind }))
    contracts.value = extractList(c)
  } catch {}
})

const fmtYuan = (cents: number) => '¥' + (cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })

// 概览金额与笔数一律按「本年度」计提/应收口径，不计历史年度欠款
const yearReceivables = computed(() =>
  receivables.value.filter(r => (r.recvYear || 0) === curYear)
)

const totalReceivable = computed(() => yearReceivables.value.reduce((s, r) => s + r.amountCents, 0))
const totalPaid       = computed(() => yearReceivables.value.reduce((s, r) => s + r.paidCents, 0))
const dividendCount   = computed(() => yearReceivables.value.filter(r => r.kind === 'dividend' || r.kind === 'reinvest_dividend').length)
const rentCount       = computed(() => yearReceivables.value.filter(r => r.kind === 'rent').length)
const serviceCount    = computed(() => yearReceivables.value.filter(r => r.kind === 'service').length)
</script>

<style scoped>
.page-header { margin-bottom: 18px; display: flex; align-items: center; justify-content: space-between; }
.page-title { font-size: 18px; font-weight: 600; color: var(--ink-900); margin: 0; }
.page-sub { font-size: 12px; color: var(--ink-300); margin: 4px 0 0; }

.ov-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 10px;
  margin-bottom: 18px;
}
.ov-stat {
  background: #ffffff;
  border: 1px solid var(--line-soft);
  border-radius: 8px;
  padding: 12px 14px;
  display: flex; align-items: center; gap: 12px;
}
.ov-stats .ov-stat { border-color: transparent; }
.ov-stats .ov-stat:nth-child(1) { background: var(--asset-bg); }
.ov-stats .ov-stat:nth-child(2) { background: var(--warn-bg); }
.ov-stats .ov-stat:nth-child(3) { background: var(--success-bg); }
.ov-stats .ov-stat:nth-child(4) { background: var(--danger-bg); }
.ov-fi {
  width: 40px; height: 40px; border-radius: 10px;
  display: inline-flex; align-items: center; justify-content: center;
  font-size: 22px; color: var(--ink-muted); background: rgba(255, 255, 255, 0.65);
  flex-shrink: 0;
}
.ov-stat-label { font-size: 12px; color: var(--ink-300); }
.ov-stat-value { font-size: 17px; font-weight: 600; color: var(--ink-900); margin-top: 2px; font-variant-numeric: tabular-nums; }

.ov-hub-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 14px;
}
.ov-hub-card {
  background: #fff;
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  padding: 18px;
  transition: all .2s;
  display: flex; flex-direction: column; gap: 12px;
  color: inherit; text-decoration: none;
}
.ov-hub-card:hover { transform: translateY(-2px); box-shadow: 0 6px 18px rgba(0,0,0,.08); }
.ov-hub-ic {
  width: 40px; height: 40px; border-radius: 10px;
  display: inline-flex; align-items: center; justify-content: center;
  font-size: 18px; color: #fff;
}
.ov-hub-ic.blue   { background: var(--info); }
.ov-hub-ic.green  { background: var(--success); }
.ov-hub-ic.orange { background: var(--terracotta); color: var(--jade-deep); }
.ov-hub-ic.purple { background: var(--accent); }
.ov-hub-card h3 { font-size: 14px; font-weight: 600; color: var(--ink-900); margin: 0; }
.ov-hub-card p  { font-size: 12px; color: var(--ink-300); margin: 0; line-height: 1.6; }
.ov-hub-meta { display: flex; align-items: center; justify-content: space-between; }
.ov-hub-meta .count { font-weight: 600; color: var(--ink-900); font-size: 20px; font-variant-numeric: tabular-nums; }
.ov-hub-meta .label { color: var(--ink-300); font-size: 12px; }
</style>
