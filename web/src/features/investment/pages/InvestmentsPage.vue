<template>
  <div class="ip-page">
    <!-- ========== 长期投资 ========== -->
    <template v-if="activeTab === 'longterm'">
      <div class="page-header">
        <div>
          <h2 class="page-title">长期投资</h2>
          <p class="page-sub">共 {{ ltItems.length }} 笔投资</p>
        </div>
        <van-button size="small" @click="exportCSV('longterm')">导出</van-button>
      </div>
      <div class="ip-stats">
        <div class="ip-stat">
          <div class="ip-stat-label">投资笔数</div>
          <div class="ip-stat-value">{{ ltItems.length }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">投资总额</div>
          <div class="ip-stat-value">{{ fmt(ltTotal) }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">年预期收益</div>
          <div class="ip-stat-value">{{ fmt(ltReturnTotal) }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">平均收益率</div>
          <div class="ip-stat-value">{{ ltAvgRate }}%</div>
        </div>
      </div>
      <div class="ip-filter">
        <van-search v-model="ltKeyword" placeholder="搜索单位名称" shape="round" background="transparent" class="ip-search" />
      </div>
      <div class="ip-table-wrap">
        <table class="ip-table">
          <thead><tr>
            <th>单位名称</th><th class="num">投资金额</th><th class="num">年收益率</th><th class="num">年收益</th>
          </tr></thead>
          <tbody>
            <tr v-for="item in ltFiltered" :key="item.id">
              <td>{{ item.name }}</td>
              <td class="num">{{ fmt(item.balanceCents) }}</td>
              <td class="num">{{ item.ratePct }}%</td>
              <td class="num">{{ fmt(item.expectedReturnCents) }}</td>
            </tr>
            <tr v-if="ltFiltered.length === 0"><td colspan="4" class="empty-cell">暂无数据</td></tr>
          </tbody>
        </table>
      </div>
    </template>

    <!-- ========== 再投资 ========== -->
    <template v-if="activeTab === 'reinvest'">
      <div class="page-header">
        <div>
          <h2 class="page-title">再投资</h2>
          <p class="page-sub">共 {{ riItems.length }} 笔再投资</p>
        </div>
        <van-button size="small" @click="exportCSV('reinvest')">导出</van-button>
      </div>
      <div class="ip-stats">
        <div class="ip-stat">
          <div class="ip-stat-label">再投资笔数</div>
          <div class="ip-stat-value">{{ riItems.length }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">再投资总额</div>
          <div class="ip-stat-value">{{ fmt(riTotal) }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">年预期收益</div>
          <div class="ip-stat-value">{{ fmt(riReturnTotal) }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">平均收益率</div>
          <div class="ip-stat-value">{{ riAvgRate }}%</div>
        </div>
      </div>
      <div class="ip-filter">
        <van-search v-model="riKeyword" placeholder="搜索单位名称" shape="round" background="transparent" class="ip-search" />
      </div>
      <div class="ip-table-wrap">
        <table class="ip-table">
          <thead><tr>
            <th>单位名称</th><th class="num">再投资金额</th><th class="num">年收益率</th><th class="num">年收益</th>
          </tr></thead>
          <tbody>
            <tr v-for="item in riFiltered" :key="item.id">
              <td>{{ item.name }}</td>
              <td class="num">{{ fmt(item.balanceCents) }}</td>
              <td class="num">{{ item.ratePct }}%</td>
              <td class="num">{{ fmt(item.expectedReturnCents) }}</td>
            </tr>
            <tr v-if="riFiltered.length === 0"><td colspan="4" class="empty-cell">暂无数据</td></tr>
          </tbody>
        </table>
      </div>
    </template>

    <!-- ========== 投资收益 ========== -->
    <template v-if="activeTab === 'returns'">
      <div class="page-header">
        <div>
          <h2 class="page-title">投资收益</h2>
          <p class="page-sub">共 {{ retItems.length }} 笔收益</p>
        </div>
        <van-button size="small" @click="exportCSV('returns')">导出</van-button>
      </div>
      <div class="ip-stats">
        <div class="ip-stat">
          <div class="ip-stat-label">收益笔数</div>
          <div class="ip-stat-value">{{ retItems.length }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">收益总额</div>
          <div class="ip-stat-value">{{ fmt(retTotal) }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">最高单笔</div>
          <div class="ip-stat-value">{{ fmt(retMax) }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">平均单笔</div>
          <div class="ip-stat-value">{{ fmt(retAvg) }}</div>
        </div>
      </div>
      <div class="ip-filter">
        <van-search v-model="retKeyword" placeholder="搜索收益项目" shape="round" background="transparent" class="ip-search" />
      </div>
      <div class="ip-table-wrap">
        <table class="ip-table">
          <thead><tr>
            <th>收益项目</th><th class="num">金额</th>
          </tr></thead>
          <tbody>
            <tr v-for="item in retFiltered" :key="item.id">
              <td>{{ item.name }}</td>
              <td class="num">{{ fmt(item.balanceCents) }}</td>
            </tr>
            <tr v-if="retFiltered.length === 0"><td colspan="2" class="empty-cell">暂无数据</td></tr>
          </tbody>
        </table>
      </div>
    </template>

    <!-- ========== 532分配 ========== -->
    <template v-if="activeTab === 'dist532'">
      <div class="page-header">
        <div>
          <h2 class="page-title">532分配</h2>
          <p class="page-sub">年度收益按 50% 再投资 · 30% 分红福利 · 20% 管理公益 分配</p>
        </div>
        <div class="ip-header-actions">
          <van-button size="small" type="primary" plain @click="openDistDialog">分配数据</van-button>
          <van-button size="small" @click="exportCSV('dist532')">导出</van-button>
        </div>
      </div>

      <!-- 年份选择 -->
      <div class="dist-year-bar">
        <label class="dist-year-label">选择年度：</label>
        <select v-model="distYear" class="dist-year-select" @change="onDistYearChange">
          <option v-for="y in yearOptions" :key="y" :value="y">{{ y }} 年</option>
        </select>
        <span class="dist-year-status" :class="{ allocated: distData !== null }">
          {{ distData ? '已分配' : '未分配' }}
        </span>
      </div>

      <div class="ip-stats">
        <div class="ip-stat">
          <div class="ip-stat-label">年度总收益</div>
          <div class="ip-stat-value">{{ fmt(distTotalIncome) }}</div>
          <div class="ip-stat-sub" v-if="totalAllocated > 0">已分配 {{ fmt(totalAllocated) }} · 剩余 {{ fmt(remainingTotal) }}</div>
          <div class="ip-stat-sub na" v-else>未分配 · 可分配 {{ fmt(remainingTotal) }}</div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">50% 再投资</div>
          <div class="ip-stat-value" :style="{color:'#07c160'}" v-if="distData">{{ fmt(distData.reinvestCents) }}</div>
          <div class="ip-stat-value na" v-else>未分配</div>
          <div class="ip-stat-sub" v-if="distData">总计 {{ fmt(reinvestStat.total) }} · 已支 {{ fmt(reinvestStat.invested) }} · <span class="rd">余 {{ fmt(reinvestStat.remain) }}</span></div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">30% 分红福利</div>
          <div class="ip-stat-value" :style="{color:'#1989fa'}" v-if="distData">{{ fmt(distData.dividendCents) }}</div>
          <div class="ip-stat-value na" v-else>未分配</div>
          <div class="ip-stat-sub" v-if="distData">总计 {{ fmt(dividendStat.total) }} · 已支 {{ fmt(dividendStat.invested) }} · <span class="rd">余 {{ fmt(dividendStat.remain) }}</span></div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-label">20% 管理公益</div>
          <div class="ip-stat-value" :style="{color:'#ff6034'}" v-if="distData">{{ fmt(distData.welfareCents) }}</div>
          <div class="ip-stat-value na" v-else>未分配</div>
          <div class="ip-stat-sub" v-if="distData">总计 {{ fmt(welfareStat.total) }} · 已支 {{ fmt(welfareStat.invested) }} · <span class="rd">余 {{ fmt(welfareStat.remain) }}</span></div>
        </div>
      </div>

      <div class="ip-section-title">{{ distYear }} 年度分配明细</div>
      <div class="ip-table-wrap">
        <table class="ip-table">
          <thead><tr>
            <th>分配项目</th><th>比例</th><th class="num">金额</th><th class="num">实际占比</th><th>操作</th>
          </tr></thead>
          <tbody>
            <tr v-if="!distData">
              <td colspan="5" class="empty-cell">未分配 — 请点击"分配数据"按钮进行年度532分配</td>
            </tr>
            <template v-if="distData">
              <tr>
                <td>再投资（扩大再生产）</td>
                <td>50%</td>
                <td class="num">{{ fmt(distData.reinvestCents) }}</td>
                <td class="num">{{ distPct(distData.reinvestCents) }}%</td>
                <td>
                  <van-button size="mini" plain :disabled="reinvestStat.remain <= 0"
                    @click="recordDistExpense('再投资')">
                    {{ reinvestStat.remain <= 0 ? '已分配完' : '记账' }}
                  </van-button>
                </td>
              </tr>
              <tr>
                <td>成员分红/福利</td>
                <td>30%</td>
                <td class="num">{{ fmt(distData.dividendCents) }}</td>
                <td class="num">{{ distPct(distData.dividendCents) }}%</td>
                <td>
                  <van-button size="mini" plain :disabled="dividendStat.remain <= 0"
                    @click="recordDistExpense('成员分红')">
                    {{ dividendStat.remain <= 0 ? '已分配完' : '记账' }}
                  </van-button>
                </td>
              </tr>
              <tr>
                <td>管理公益支出</td>
                <td>20%</td>
                <td class="num">{{ fmt(distData.welfareCents) }}</td>
                <td class="num">{{ distPct(distData.welfareCents) }}%</td>
                <td>
                  <van-button size="mini" plain :disabled="welfareStat.remain <= 0"
                    @click="recordDistExpense('公益支出')">
                    {{ welfareStat.remain <= 0 ? '已分配完' : '记账' }}
                  </van-button>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <div class="ip-section-title">{{ distYear }} 年度支出记录</div>
      <div class="ip-table-wrap">
        <table class="ip-table">
          <thead><tr>
            <th>时间</th><th>支出项目</th><th>类别</th><th class="num">金额</th>
          </tr></thead>
          <tbody>
            <tr v-if="!distData">
              <td colspan="4" class="empty-cell">暂无支出记录</td>
            </tr>
            <template v-if="distData">
              <tr v-for="e in distExpenses" :key="e.id">
                <td>{{ e.date }}</td>
                <td>{{ e.itemName }}</td>
                <td>{{ e.category }}</td>
                <td class="num">{{ fmt(e.amountCents) }}</td>
              </tr>
              <tr v-if="distExpenses.length === 0">
                <td colspan="4" class="empty-cell">暂无支出记录</td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </template>
  <!-- 532分配弹窗 -->
    <van-dialog
      v-model:show="showDistDialog"
      :title="distLocked ? '532分配数据（已锁定）' : '532分配数据'"
      :show-cancel-button="!distLocked"
      :confirm-button-text="distLocked ? '关闭' : '确认分配'"
      @confirm="distLocked ? (showDistDialog = false) : confirmDist()"
      @cancel="showDistDialog = false"
      class="dist-dialog"
    >
      <div class="dist-dialog-body">
        <div class="dist-dialog-year">{{ distYear }} 年度</div>
        <div v-if="distLocked" class="dist-dialog-lock">
          该方案已有支出记录，无法修改
        </div>
        <div class="dist-dialog-hint">{{ distLocked ? '查看已分配的方案数据' : ('投资收益总余额 ' + fmt(distTotalIncome) + '，已分配 ' + fmt(totalAllocated) + '，剩余可分配 ' + fmt(remainingTotal) + '。按剩余可分配的 50%/30%/20% 生成初始方案，可手动修改') }}</div>
        <div class="dist-dialog-fields">
          <div class="dist-dialog-field">
            <label>再投资（50%）</label>
            <div class="dist-input-wrap">
              <input type="number" class="dist-input" :class="{ readonly: distLocked }" v-model.number="distForm.reinvest" :readonly="distLocked" />
              <span class="dist-input-unit">元</span>
            </div>
          </div>
          <div class="dist-dialog-field">
            <label>分红福利（30%）</label>
            <div class="dist-input-wrap">
              <input type="number" class="dist-input" :class="{ readonly: distLocked }" v-model.number="distForm.dividend" :readonly="distLocked" />
              <span class="dist-input-unit">元</span>
            </div>
          </div>
          <div class="dist-dialog-field">
            <label>管理公益（20%）</label>
            <div class="dist-input-wrap">
              <input type="number" class="dist-input" :class="{ readonly: distLocked }" v-model.number="distForm.welfare" :readonly="distLocked" />
              <span class="dist-input-unit">元</span>
            </div>
          </div>
        </div>
        <div class="dist-dialog-total">
          合计：¥{{ distFormTotal.toLocaleString('zh-CN', { minimumFractionDigits: 2 }) }}
          <span v-if="distFormTotal !== Math.round(remainingTotal / 100)" class="dist-dialog-diff">
            （与剩余可分配差 ¥{{ Math.abs(distFormTotal - Math.round(remainingTotal / 100)).toLocaleString('zh-CN', { minimumFractionDigits: 2 }) }}）
          </span>
        </div>
      </div>
    </van-dialog>

    <!-- 支出记账弹窗 -->
    <van-dialog
      v-model:show="showRecordDialog"
      :title="recordTitle"
      show-cancel-button
      :confirm-button-text="recordSaving ? '保存中...' : '确认记账'"
      :confirm-disabled="recordSaving"
      @confirm="confirmRecord"
      @cancel="showRecordDialog = false"
    >
      <div class="dist-dialog-body">
        <div class="dist-dialog-field">
          <label>日期</label>
          <input type="date" class="dist-input" v-model="recordForm.date" style="width:160px;" />
        </div>
        <div class="dist-dialog-field" v-if="recordForm.category === '再投资'" style="margin-top:10px;">
          <label>往来单位</label>
          <select v-model.number="recordForm.partyId" class="dist-year-select" style="min-width:200px;" @change="onReinvestPartyChange">
            <option :value="null" disabled>请选择单位</option>
            <option v-for="opt in reinvestPartyOptions" :key="opt.value" :value="opt.value">{{ opt.text }}</option>
          </select>
        </div>
        <div class="dist-dialog-field" style="margin-top:10px;">
          <label>金额</label>
          <div class="dist-input-wrap">
            <input type="number" class="dist-input" v-model.number="recordForm.amount" placeholder="输入金额" />
            <span class="dist-input-unit">元</span>
          </div>
        </div>
        <div class="dist-dialog-field" style="margin-top:10px;">
          <label>备注</label>
          <input type="text" class="dist-input" v-model="recordForm.note" placeholder="支出备注" style="width:auto;flex:1;" />
        </div>
      </div>
    </van-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { api } from '../../../lib/http'
import { todayStr } from '../../../types/api'

interface Category {
  id: number; name: string; level: number; parentId?: number;
  children?: Category[]; balanceCents?: number;
}
interface Party {
  id: number; name: string; type: string; types: string[];
  investAmountCents: number; returnRateBps: number; expectedReturnCents: number;
  contactPhone: string;
}
interface Distribution {
  id: number; itemName: string; year: number; category: string; amountCents: number;
}

const props = defineProps<{ activeTab: string }>()

const categories = ref<Category[]>([])
const parties = ref<Party[]>([])
const distributions = ref<Distribution[]>([])

// 搜索关键词
const ltKeyword = ref('')
const riKeyword = ref('')
const retKeyword = ref('')

const fmt = (c: number) => '¥' + (c / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })

function findCat(name: string) {
  return categories.value.find(c => c.name === name)
}

function matchParty(name: string) {
  return parties.value.find(p => p.name === name)
}

function buildItems(catName: string) {
  const cat = findCat(catName)
  if (!cat || !cat.children) return []
  return cat.children.map(c => {
    const p = matchParty(c.name)
    const rateBps = p?.returnRateBps || 0
    // 长期投资科目按"支出"记账，余额为负；展示统一取投出口径（绝对值）
    const balanceCents = Math.abs(c.balanceCents || 0)
    const expectedReturn = p?.expectedReturnCents || Math.round(balanceCents * rateBps / 10000)
    return {
      id: c.id, name: c.name,
      balanceCents,
      ratePct: (rateBps / 100).toFixed(2), rateBps,
      expectedReturnCents: expectedReturn,
    }
  })
}

// ====== 长期投资 ======
const ltItems = computed(() => buildItems('长期投资'))
const ltFiltered = computed(() => ltItems.value.filter(i => !ltKeyword.value || i.name.includes(ltKeyword.value)))
const ltTotal = computed(() => ltItems.value.reduce((s, i) => s + i.balanceCents, 0))
const ltReturnTotal = computed(() => ltItems.value.reduce((s, i) => s + i.expectedReturnCents, 0))
const ltAvgRate = computed(() => ltTotal.value ? (ltReturnTotal.value / ltTotal.value * 100).toFixed(2) : '0.00')

// ====== 再投资 ======
const riItems = computed(() => buildItems('再投资'))
const riFiltered = computed(() => riItems.value.filter(i => !riKeyword.value || i.name.includes(riKeyword.value)))
const riTotal = computed(() => riItems.value.reduce((s, i) => s + i.balanceCents, 0))
const riReturnTotal = computed(() => riItems.value.reduce((s, i) => s + i.expectedReturnCents, 0))
const riAvgRate = computed(() => riTotal.value ? (riReturnTotal.value / riTotal.value * 100).toFixed(2) : '0.00')

// ====== 投资收益 ======
const retItems = computed(() => {
  const cat = findCat('投资收益')
  if (!cat || !cat.children) return []
  return cat.children.map(c => ({ id: c.id, name: c.name, balanceCents: c.balanceCents || 0 }))
})
const retFiltered = computed(() => retItems.value.filter(i => !retKeyword.value || i.name.includes(retKeyword.value)))
const retTotal = computed(() => retItems.value.reduce((s, i) => s + i.balanceCents, 0))
const retMax = computed(() => retItems.value.length ? Math.max(...retItems.value.map(i => i.balanceCents)) : 0)
const retAvg = computed(() => retItems.value.length ? Math.round(retTotal.value / retItems.value.length) : 0)

// ====== 532分配 ======
interface DistRecord {
  id: number; year: number; totalIncomeCents: number;
  reinvestCents: number; dividendCents: number; welfareCents: number;
  createdAt: string; updatedAt: string;
}
interface DistExpense {
  id: number; itemName: string; category: string; amountCents: number; date: string;
}

// 年份选项：从2026年到当前年
const START_YEAR = 2026
const yearOptions = computed(() => {
  const cur = new Date().getFullYear()
  const years: number[] = []
  for (let y = cur; y >= START_YEAR; y--) years.push(y)
  return years
})

const distYear = ref(new Date().getFullYear())
const distData = ref<DistRecord | null>(null)
const allDistributions = ref<DistRecord[]>([])
const distExpenses = ref<DistExpense[]>([])

// 分配弹窗
const showDistDialog = ref(false)
const distLocked = ref(false)
const distForm = ref({ reinvest: 0, dividend: 0, welfare: 0 })

// 投资收益总余额（累计的，所有可分配资金）
const distTotalIncome = computed(() => {
  const cat = findCat('投资收益')
  return cat?.children?.reduce((s, c) => s + (c.balanceCents || 0), 0) || 0
})

const distFormTotal = computed(() => distForm.value.reinvest + distForm.value.dividend + distForm.value.welfare)

const distPct = (cents: number) => {
  if (!distData.value) return '0.0'
  const yearTotal = distData.value.reinvestCents + distData.value.dividendCents + distData.value.welfareCents
  if (!yearTotal) return '0.0'
  return ((cents / yearTotal) * 100).toFixed(1)
}

// 所有年份已分配总和
const totalAllocated = computed(() => {
  return allDistributions.value.reduce((s, d) => s + d.reinvestCents + d.dividendCents + d.welfareCents, 0)
})

// 当前已选年份的分配合计
const allocatedTotal = computed(() => {
  if (!distData.value) return 0
  return distData.value.reinvestCents + distData.value.dividendCents + distData.value.welfareCents
})

// 剩余可分配 = 投资收益余额 - 所有年份已分配
const remainingTotal = computed(() => {
  return Math.max(0, distTotalIncome.value - totalAllocated.value)
})

// 532 各类目投入统计：已投入 = 该类目已记账的支出流水合计（不限定年度）
const distInvested = reactive({ reinvest: 0, dividend: 0, welfare: 0 })

async function loadDistInvested() {
  distInvested.reinvest = 0
  distInvested.dividend = 0
  // welfare 复用已按年度加载的支出记录（仅公益支出类别）
  distInvested.welfare = distExpenses.value
    .filter(e => e.category === '公益支出')
    .reduce((s, e) => s + e.amountCents, 0)
  const reinvestL1 = categories.value.find(c => c.name === '再投资' && c.level === 1)
  const divCat = findEquityCat('成员分红')
  await Promise.all([
    reinvestL1 ? loadCatSpent(reinvestL1.id, 'reinvest', distYear.value) : Promise.resolve(),
    divCat ? loadCatSpent(divCat.id, 'dividend', distYear.value) : Promise.resolve(),
  ])
}
async function loadCatSpent(catId: number, key: 'reinvest' | 'dividend', year: number) {
  try {
    const from = `${year}-01-01`
    const to = `${year}-12-31`
    const r = await api.get<any>(`/transactions?categoryId=${catId}&direction=expense&from=${from}&to=${to}&pageSize=10000`)
    const items = r?.data?.items || r?.items || (Array.isArray(r?.data) ? r?.data : []) || []
    distInvested[key] = (items as any[]).reduce((s, t) => s + (t.amountCents || 0), 0)
  } catch { /* 忽略单个类别统计失败 */ }
}

// 三张类目卡的「总计 · 已投入 · 余」
const reinvestStat = computed(() => ({
  total: distData.value?.reinvestCents || 0,
  invested: distInvested.reinvest,
  remain: Math.max(0, (distData.value?.reinvestCents || 0) - distInvested.reinvest),
}))
const dividendStat = computed(() => ({
  total: distData.value?.dividendCents || 0,
  invested: distInvested.dividend,
  remain: Math.max(0, (distData.value?.dividendCents || 0) - distInvested.dividend),
}))
const welfareStat = computed(() => ({
  total: distData.value?.welfareCents || 0,
  invested: distInvested.welfare,
  remain: Math.max(0, (distData.value?.welfareCents || 0) - distInvested.welfare),
}))

async function loadDist(year: number) {
  try {
    const from = `${year}-01-01`
    const to = `${year}-12-31`
    const reinvestL1 = findCat('再投资')
    const divCat = findEquityCat('成员分红')
    const welCat = findEquityCat('公益支出')
    const sources = [
      { id: reinvestL1?.id as number | undefined, label: '再投资' },
      { id: divCat?.id as number | undefined, label: '成员分红' },
      { id: welCat?.id as number | undefined, label: '公益支出' },
    ].filter(s => !!s.id)

    const [distR, allR, ...expR] = await Promise.all([
      api.get<any>('/distributions-532?year=' + year),
      api.get<any>('/distributions-532'),
      ...sources.map(s => api.get<any>(
        `/transactions?categoryId=${s.id}&direction=expense&from=${from}&to=${to}&pageSize=10000`
      )),
    ])
    const payload = distR?.data !== undefined ? distR.data : distR
    distData.value = payload || null
    // 加载所有年份分配记录，用于计算已分配总和
    const allRaw = Array.isArray(allR) ? allR : (allR?.data || allR?.items || [])
    allDistributions.value = Array.isArray(allRaw) ? allRaw : []
    // 年度支出记录 = 532 三类（再投资 / 成员分红 / 公益支出）在本年度的全部支出
    const rows: DistExpense[] = []
    sources.forEach((s, i) => {
      const items = (expR[i]?.data?.items || expR[i]?.items || [])
      items.forEach((t: any) => rows.push({
        id: t.id,
        itemName: t.note || s.label,
        category: s.label,
        amountCents: t.amountCents || 0,
        date: t.txnDate || '',
      }))
    })
    rows.sort((a, b) => (a.date === b.date ? b.id - a.id : (a.date < b.date ? 1 : -1)))
    distExpenses.value = rows
    await loadDistInvested()
  } catch (e) { distData.value = null; allDistributions.value = []; distExpenses.value = [] }
}

async function onDistYearChange() {
  await loadDist(distYear.value)
}

function openDistDialog() {
  // 如果已存在支出记录，方案锁定不可编辑
  if (distExpenses.value.length > 0) {
    // 填充现有方案数据用于展示
    if (distData.value) {
      distForm.value = {
        reinvest: Math.round(distData.value.reinvestCents / 100),
        dividend: Math.round(distData.value.dividendCents / 100),
        welfare: Math.round(distData.value.welfareCents / 100),
      }
    }
    distLocked.value = true
  } else {
    distLocked.value = false
    const remaining = remainingTotal.value
    const remainingYuan = Math.round(remaining / 100)
    distForm.value = {
      reinvest: Math.round(remainingYuan * 0.5),
      dividend: Math.round(remainingYuan * 0.3),
      welfare: remainingYuan - Math.round(remainingYuan * 0.5) - Math.round(remainingYuan * 0.3),
    }
  }
  showDistDialog.value = true
}

async function confirmDist() {
  if (distLocked.value) { showDistDialog.value = false; return }
  const totalYuan = distFormTotal.value
  const remainingYuan = Math.round(remainingTotal.value / 100)
  if (totalYuan > remainingYuan) {
    alert(`分配合计 ¥${totalYuan.toLocaleString('zh-CN')} 超过剩余可分配 ¥${remainingYuan.toLocaleString('zh-CN')}，请修改`)
    return
  }
  try {
    const body = {
      year: distYear.value,
      totalIncomeCents: distTotalIncome.value,
      reinvestCents: Math.round(distForm.value.reinvest * 100),
      dividendCents: Math.round(distForm.value.dividend * 100),
      welfareCents: Math.round(distForm.value.welfare * 100),
    }
    const r = await api.post<any>('/distributions-532', body)
    distData.value = r?.data || r
    showDistDialog.value = false
    alert('532分配数据已保存')
  } catch (e: any) {
    alert('保存失败：' + (e?.message || '未知错误'))
  }
}

// 532分配支出记账（统一走快速记账模板）
const CATEGORY_MAP: Record<string, string> = { '再投资': '再投资', '成员分红': '成员分红', '公益支出': '公益支出' }
const showRecordDialog = ref(false)
const recordForm = ref({ category: '', amount: 0, date: todayStr(), note: '', partyId: null as number | null })
const recordSaving = ref(false)

const RECORD_TITLES: Record<string, string> = {
  '再投资': '532-再投资记账',
  '成员分红': '532-成员分配发放',
  '公益支出': '532-公益支出',
}
const RECORD_DEFAULT_NOTES: Record<string, string> = {
  '再投资': '',
  '成员分红': '532-成员分配发放',
  '公益支出': '532-公益支出',
}
const recordTitle = computed(() => RECORD_TITLES[recordForm.value.category] || '532支出记账')

const reinvestPartyOptions = computed(() => {
  return parties.value
    .filter(p => {
      const types = p.types?.length ? p.types : (p.type ? [p.type] : [])
      return types.some(t => t === 'invest' || t === 'reinvest')
    })
    .map(p => ({ text: p.name, value: p.id }))
})

function partyNameById(id: number | null | undefined): string {
  if (!id) return ''
  return parties.value.find(p => p.id === id)?.name || ''
}

function findEquityCat(name: string): any {
  for (const l1 of categories.value) {
    if (l1.children) {
      for (const l2 of l1.children) {
        if (l2.kind === 'equity' && l2.name === name) return l2
      }
    }
  }
  return null
}

async function ensureL2InL1(l1Name: string, l2Name: string) {
  const l1 = categories.value.find(c => c.name === l1Name && c.level === 1)
  if (!l1) throw new Error(`缺少一级科目：${l1Name}`)
  const existing = (l1.children || []).find(c => c.name === l2Name)
  if (existing) return existing
  const res = await api.post<any>('/categories', {
    name: l2Name, level: 2, parentId: l1.id, kind: 'equity',
  })
  const newL2 = res.data || res
  if (l1.children) l1.children.push(newL2)
  else l1.children = [newL2]
  return newL2
}

function recordDistExpense(cat: string) {
  // 已全部支出完毕的项目不允许再记账
  const stat = cat === '再投资' ? reinvestStat.value
    : cat === '成员分红' ? dividendStat.value
    : welfareStat.value
  if (stat.remain <= 0) {
    alert('该项目本年度已全部支出完毕，不能再记账')
    return
  }
  recordForm.value = { category: cat, amount: 0, date: todayStr(), note: RECORD_DEFAULT_NOTES[cat] || cat, partyId: null }
  showRecordDialog.value = true
}
function onReinvestPartyChange() {
  const name = partyNameById(recordForm.value.partyId)
  recordForm.value.note = name ? `${name}—再投资` : '532-再投资'
}
async function confirmRecord() {
  if (!recordForm.value.amount || recordForm.value.amount <= 0) { alert('请输入有效金额'); return }

  const catStat = recordForm.value.category === '再投资' ? reinvestStat.value
    : recordForm.value.category === '成员分红' ? dividendStat.value
    : welfareStat.value
  if (catStat.remain <= 0) { alert('该项目本年度已全部支出完毕，不能再记账'); return }

  const isReinvest = recordForm.value.category === '再投资'
  if (isReinvest && !recordForm.value.partyId) { alert('请选择往来单位'); return }

  recordSaving.value = true
  try {
    let categoryId: number
    let note = recordForm.value.note || recordForm.value.category

    if (isReinvest) {
      // 走快速记账 532-再投资 模板逻辑：在再投资L1下建单位名L2
      const partyName = partyNameById(recordForm.value.partyId)
      const l2 = await ensureL2InL1('再投资', partyName)
      categoryId = l2.id
      note = recordForm.value.note || `${partyName}—再投资`
    } else {
      // 走快速记账 成员分红/公益支出 模板逻辑：按名称查找科目
      const catName = CATEGORY_MAP[recordForm.value.category]
      if (!catName) { alert('未知支出类别'); return }
      const cat = findEquityCat(catName)
      if (!cat) { alert(`缺少科目：${catName}`); return }
      categoryId = cat.id
    }

    await api.post('/transactions', {
      direction: 'expense',
      amountCents: Math.round(recordForm.value.amount * 100),
      categoryId,
      txnDate: recordForm.value.date,
      note,
      partyId: isReinvest ? recordForm.value.partyId : null,
      status: 'normal',
    })
    showRecordDialog.value = false
    alert('记账成功（快速记账模板）')
    await loadDist(distYear.value)
  } catch (e: any) {
    alert('记账失败：' + (e?.message || '未知错误'))
  } finally {
    recordSaving.value = false
  }
}

// 导出
function downloadCSV(filename: string, headers: string[], rows: string[][]) {
  const csv = [headers.join(','), ...rows.map(r => r.map(c => '"' + (c || '').replace(/"/g, '""') + '"').join(','))].join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = filename; a.click()
  URL.revokeObjectURL(url)
}

function exportCSV(tab: string) {
  if (tab === 'longterm') {
    downloadCSV('长期投资.csv', ['单位名称', '投资金额', '年收益率', '年收益'],
      ltFiltered.value.map(i => [i.name, fmt(i.balanceCents), i.ratePct + '%', fmt(i.expectedReturnCents)]))
  } else if (tab === 'reinvest') {
    downloadCSV('再投资.csv', ['单位名称', '再投资金额', '年收益率', '年收益'],
      riFiltered.value.map(i => [i.name, fmt(i.balanceCents), i.ratePct + '%', fmt(i.expectedReturnCents)]))
  } else if (tab === 'returns') {
    downloadCSV('投资收益.csv', ['收益项目', '金额'],
      retFiltered.value.map(i => [i.name, fmt(i.balanceCents)]))
  } else if (tab === 'dist532') {
    if (!distData.value) { downloadCSV('532分配.csv', ['提示'], [['该年度尚未分配']]); return }
    downloadCSV('532分配.csv', ['分配项目', '比例', '金额', '实际占比'],
      [['再投资（扩大再生产）', '50%', fmt(distData.value.reinvestCents), distPct(distData.value.reinvestCents) + '%'],
       ['成员分红/福利', '30%', fmt(distData.value.dividendCents), distPct(distData.value.dividendCents) + '%'],
       ['管理公益支出', '20%', fmt(distData.value.welfareCents), distPct(distData.value.welfareCents) + '%'],
       ...distExpenses.value.map(e => [e.date, e.itemName, e.category, fmt(e.amountCents)])])
  }
}

async function load() {
  try {
    const [cats, p] = await Promise.all([
      api.get('/categories'),
      api.get<{ data: Party[] } | Party[]>('/parties'),
    ])
    categories.value = Array.isArray(cats) ? cats : (cats?.data || [])
    const raw = Array.isArray(p) ? p : (p as any)?.data || []
    parties.value = raw
    // 加载当前年度的532分配数据
    await loadDist(distYear.value)
  } catch (e) { console.error('load error', e) }
}

onMounted(load)
</script>

<style scoped>
.page-header { margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; }
.page-title { font-size: 18px; font-weight: 600; margin: 0; }
.page-sub   { font-size: 12px; color: #969799; margin: 4px 0 0; }

.ip-stats {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-bottom: 14px;
}
.ip-stat {
  background: #fff; border: 1px solid var(--line-soft, #eaeaea); border-radius: 8px; padding: 10px 12px;
}
.ip-stat-label { font-size: 12px; color: #969799; }
.ip-stat-value { font-size: 16px; font-weight: 600; color: var(--ink-900, #1f2329); margin-top: 2px; font-variant-numeric: tabular-nums; }
.ip-stat-sub { font-size: 11px; color: #969799; margin-top: 4px; }
.ip-stat-sub.na { color: #969799; }
.ip-stat-sub .rd { color: #ee0a24; }
.ip-stats .ip-stat { border-color: transparent; }
.ip-stats .ip-stat:nth-child(1) { background: #e8f0fb; }
.ip-stats .ip-stat:nth-child(2) { background: #e6f5f4; }
.ip-stats .ip-stat:nth-child(3) { background: #f1edfc; }
.ip-stats .ip-stat:nth-child(4) { background: #fff1e0; }

.ip-filter { margin-bottom: 8px; }
.ip-search { padding: 0; }

.ip-table-wrap { overflow-x: auto; border: 1px solid var(--line-soft, #eaeaea); border-radius: 8px; background: #fff; }
.ip-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.ip-table th {
  position: sticky; top: 0; z-index: 1;
  background: #f7f8fa; padding: 10px 12px; text-align: left; font-weight: 600; color: #646566;
  border-bottom: 1px solid #ebedf0; white-space: nowrap;
}
.ip-table td { padding: 10px 12px; border-bottom: 1px solid #f0f1f2; color: #1f2329; }
.ip-table tr:last-child td { border-bottom: none; }
.ip-table td.num { text-align: right; font-variant-numeric: tabular-nums; }
.ip-table th.num { text-align: right; }
.ip-table .empty-cell { text-align: center; color: #969799; padding: 24px; }

.ip-section-title {
  font-size: 14px; font-weight: 600; color: #1f2329;
  margin: 20px 0 8px; padding: 0 2px;
}

/* 532分配样式 */
.dist-year-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; }
.dist-year-label { font-size: 13px; color: #646566; white-space: nowrap; }
.dist-year-select {
  padding: 6px 10px; border: 1px solid #dcdee0; border-radius: 6px; font-size: 14px;
  background: #fff; outline: none; min-width: 120px;
}
.dist-year-select:focus { border-color: #1989fa; }
.dist-year-status {
  font-size: 12px; padding: 2px 10px; border-radius: 10px; background: #f7f8fa; color: #969799;
}
.dist-year-status.allocated { background: #e8f8e8; color: #07c160; font-weight: 500; }

.ip-stat-value.na { color: #969799; font-weight: 400; font-size: 13px; }

/* 分配弹窗 */
.dist-dialog { width: 90vw; max-width: 480px; }
.dist-dialog-body { padding: 0 16px 16px; }
.dist-dialog-year { font-size: 15px; font-weight: 600; color: #1f2329; margin-bottom: 4px; }
.dist-dialog-hint { font-size: 12px; color: #969799; margin-bottom: 16px; line-height: 1.5; }
.dist-dialog-lock { font-size: 13px; color: #ee0a24; margin-bottom: 12px; padding: 8px 12px; background: #fff2f0; border-radius: 4px; }
.dist-dialog-fields { display: flex; flex-direction: column; gap: 10px; }
.dist-dialog-field {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 12px; background: #f7f8fa; border-radius: 6px;
}
.dist-dialog-field label { font-size: 13px; font-weight: 500; color: #1f2329; white-space: nowrap; }
.dist-input-wrap { display: flex; align-items: center; gap: 4px; }
.dist-input {
  width: 140px; padding: 6px 8px; border: 1px solid #dcdee0; border-radius: 4px;
  font-size: 14px; text-align: right; outline: none;
}
.dist-input:focus { border-color: #1989fa; }
.dist-input.readonly { background: #f0f0f0; cursor: not-allowed; }
.dist-input-unit { font-size: 12px; color: #969799; }
.dist-dialog-total {
  margin-top: 12px; text-align: right; font-size: 13px; font-weight: 500; color: #1f2329;
}
.dist-dialog-diff { font-size: 11px; color: #ee0a24; font-weight: 400; }

@media (max-width: 600px) {
  .ip-stats { grid-template-columns: repeat(2, 1fr); }
  .ip-table th, .ip-table td { padding: 8px 8px; font-size: 12px; }
}
</style>