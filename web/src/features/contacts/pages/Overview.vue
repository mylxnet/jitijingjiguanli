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
        <span class="ov-stat-ic blue">🏢</span>
        <div>
          <div class="ov-stat-label">单位总数</div>
          <div class="ov-stat-value">{{ parties.length }}</div>
        </div>
      </div>
      <div class="ov-stat">
        <span class="ov-stat-ic orange">📥</span>
        <div>
          <div class="ov-stat-label">本年应收合计</div>
          <div class="ov-stat-value">{{ fmtYuan(totalReceivable) }}</div>
        </div>
      </div>
      <div class="ov-stat">
        <span class="ov-stat-ic green">✅</span>
        <div>
          <div class="ov-stat-label">本年已收</div>
          <div class="ov-stat-value">{{ fmtYuan(totalPaid) }}</div>
        </div>
      </div>
      <div class="ov-stat">
        <span class="ov-stat-ic red">⏳</span>
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
interface Receivable { kind: string; amountCents: number; paidCents: number }
interface Contract { id: number }

const parties = ref<Party[]>([])
const receivables = ref<Receivable[]>([])
const contracts = ref<Contract[]>([])

onMounted(async () => {
  try {
    const [p, r, c] = await Promise.all([
      api.get<any>('/parties'),
      api.get<any>('/receivables'),
      api.get<any>('/contracts'),
    ])
    parties.value = extractList(p)
    receivables.value = extractList(r)
    contracts.value = extractList(c)
  } catch {}
})

const fmtYuan = (cents: number) => '¥' + (cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })

const totalReceivable = computed(() => receivables.value.reduce((s, r) => s + r.amountCents, 0))
const totalPaid       = computed(() => receivables.value.reduce((s, r) => s + r.paidCents, 0))
const dividendCount   = computed(() => receivables.value.filter(r => r.kind === 'dividend' || r.kind === 'reinvest_dividend').length)
const rentCount       = computed(() => receivables.value.filter(r => r.kind === 'rent').length)
const serviceCount    = computed(() => receivables.value.filter(r => r.kind === 'service').length)
</script>

<style scoped>
.page-header { margin-bottom: 18px; display: flex; align-items: center; justify-content: space-between; }
.page-title { font-size: 18px; font-weight: 600; color: var(--ink-900, #1f2329); margin: 0; }
.page-sub { font-size: 12px; color: var(--ink-300, #969799); margin: 4px 0 0; }

.ov-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 10px;
  margin-bottom: 18px;
}
.ov-stat {
  background: #fff;
  border: 1px solid var(--line-soft, #eaeaea);
  border-radius: 8px;
  padding: 12px 14px;
  display: flex; align-items: center; gap: 12px;
}
.ov-stat-ic {
  width: 36px; height: 36px; border-radius: 8px;
  display: inline-flex; align-items: center; justify-content: center;
  font-size: 16px; color: #fff; flex-shrink: 0;
}
.ov-stat-ic.blue   { background: #1989fa; }
.ov-stat-ic.orange { background: #ff6034; }
.ov-stat-ic.green  { background: #07c160; }
.ov-stat-ic.red    { background: #ee0a24; }
.ov-stat-label { font-size: 12px; color: var(--ink-300, #969799); }
.ov-stat-value { font-size: 17px; font-weight: 600; color: var(--ink-900, #1f2329); margin-top: 2px; font-variant-numeric: tabular-nums; }

.ov-hub-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 14px;
}
.ov-hub-card {
  background: #fff;
  border: 1px solid var(--line-soft, #eaeaea);
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
.ov-hub-ic.blue   { background: linear-gradient(135deg, #1989fa, #0074d9); }
.ov-hub-ic.green  { background: linear-gradient(135deg, #07c160, #06ad56); }
.ov-hub-ic.orange { background: linear-gradient(135deg, #ff6034, #ee3f12); }
.ov-hub-ic.purple { background: linear-gradient(135deg, #764ba2, #667eea); }
.ov-hub-card h3 { font-size: 14px; font-weight: 600; color: var(--ink-900, #1f2329); margin: 0; }
.ov-hub-card p  { font-size: 12px; color: var(--ink-300, #969799); margin: 0; line-height: 1.6; }
.ov-hub-meta { display: flex; align-items: center; justify-content: space-between; }
.ov-hub-meta .count { font-weight: 600; color: var(--ink-900, #1f2329); font-size: 20px; font-variant-numeric: tabular-nums; }
.ov-hub-meta .label { color: var(--ink-300, #969799); font-size: 12px; }
</style>
