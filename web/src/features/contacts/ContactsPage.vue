<template>
  <div class="contacts-page">
    <div class="page-header">
      <h3>往来</h3>
      <div class="header-actions">
        <van-button type="primary" plain size="small" @click="openAccrue">年度结转</van-button>
        <van-button type="primary" plain size="small" icon="down" @click="showExportMenu = true">导出</van-button>
        <van-button type="primary" size="small" icon="plus" @click="openAddParty">新增单位</van-button>
      </div>
    </div>

    <!-- 欠款分类 banner -->
    <div class="owe-banners">
      <div class="owe-banner total" :class="{ active: activeTypeFilter === 'all' }" @click="toggleBanner('all')">
        <div class="banner-label">欠款总计</div>
        <div class="banner-value">{{ formatFen(totalOwe) }}</div>
      </div>
      <div class="owe-banner rent" :class="{ active: activeTypeFilter === 'flow' }" @click="toggleBanner('flow')">
        <div class="banner-label">流转费欠款</div>
        <div class="banner-value">{{ formatFen(rentOweTotal) }}</div>
      </div>
      <div class="owe-banner invest" :class="{ active: activeTypeFilter === 'invest' }" @click="toggleBanner('invest')">
        <div class="banner-label">投资收益欠款</div>
        <div class="banner-value">{{ formatFen(dividendOweTotal) }}</div>
      </div>
    </div>

    <!-- 列表：往来单位 -->
    <div v-if="loading" class="loading-state"><van-skeleton title :row="4" /></div>
    <div v-else-if="parties.length === 0" class="empty-state">
      <p>还没有往来单位</p>
      <van-button size="small" type="primary" @click="openAddParty">新增单位</van-button>
    </div>
    <div v-else>
      <div v-if="displayParties.length === 0" class="empty-state">
        <p>{{ activeTypeFilter === 'flow' ? '暂无流转企业' : activeTypeFilter === 'invest' ? '暂无投资公司' : '还没有往来单位' }}</p>
      </div>
      <div v-else class="party-list">
        <div v-for="p in displayParties" :key="p.id" class="party-row" @click="openDetail(p)">
          <div class="party-main">
            <span class="party-name">{{ p.name }}</span>
            <span class="l2-chip" :class="p.type">{{ partyTypeLabel[p.type] || '流转企业' }}</span>
            <span v-if="p.type === 'invest'" class="l2-chip invest-chip">投资 {{ formatFen(p.investAmountCents || 0) }}</span>
          </div>
          <div class="party-owed">
            <div class="owed-value" :class="{ zero: p.outstandingCents <= 0 }">
              {{ formatFen(p.outstandingCents) }}
            </div>
            <div class="owed-label">欠款</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 单位详情弹窗 -->
    <van-popup
      v-model:show="showPartyDetail"
      position="bottom"
      round
      closeable
      class="party-detail-popup"
      @closed="onPartyDetailClosed"
    >
      <div v-if="currentParty">
        <div class="party-detail-head">
          <div class="pd-title">
            <h3 class="detail-title">{{ currentParty.name }}</h3>
            <span v-if="currentParty.type" class="pd-type-text">{{ partyTypeLabel[currentParty.type] || '' }}</span>
          </div>
          <div class="pd-actions">
            <van-button class="pd-btn" size="small" @click="openAddReceivable">登记应收</van-button>
            <van-button class="pd-btn ghost" size="small" @click="openAddPartyEdit">编辑单位</van-button>
          </div>
        </div>

        <div class="detail-block">
          <div class="section-title">基本情况</div>
          <div class="party-summary">
          <div class="summary-item">
            <div class="summary-value" :class="{ zero: currentParty.outstandingCents <= 0 }">
              {{ formatFen(currentParty.outstandingCents) }}
            </div>
            <div class="summary-label">合计欠款</div>
          </div>
          <div class="summary-item">
            <div class="summary-value">{{ receivableItems.length }}</div>
            <div class="summary-label">应收单</div>
          </div>
          <div class="summary-meta">
            <span class="meta-line">联系电话：{{ currentParty.contactPhone || '—' }}</span>
            <span v-if="currentParty.type === 'flow'" class="meta-line">流转面积：{{ currentParty.areaMu || 0 }} 亩</span>
            <span v-if="currentParty.note" class="meta-line">备注：{{ currentParty.note }}</span>
            <span v-if="currentParty.type === 'invest'" class="meta-line">投资金额（只读）{{ formatFen(currentParty.investAmountCents || 0) }} = 长期投资同名公司累计投出</span>
            <span v-if="currentParty.type === 'flow'" class="meta-line">
              年度流转费标准：{{ currentPartyStd ? formatFen(currentPartyStd.amountCents) : '未设置' }}
              <a class="std-edit-link" @click="openStdForParty">去修改</a>
            </span>
            <span v-if="currentParty.type === 'invest'" class="meta-line">
              年度应得分红：{{ currentPartyStd ? formatFen(currentPartyStd.amountCents) : '未设置' }}
              <a class="std-edit-link" @click="openStdForParty">去修改</a>
            </span>
          </div>
          </div>
        </div>

        <div class="detail-block">
          <div class="section-title">应收记录</div>
          <div v-if="detailLoading" class="loading-state"><van-skeleton title :row="4" /></div>
        <div v-else-if="receivableItems.length === 0" class="empty-state">
          <p>该单位暂无应收记录</p>
          <van-button size="small" type="primary" @click="openAddReceivable">登记应收</van-button>
        </div>

        <div v-else class="recv-list">
          <div v-for="r in receivableItems" :key="r.id" class="recv-card">
            <div class="recv-head" @click="toggleReceipts(r)">
              <div class="recv-title-wrap">
                <span class="recv-title">{{ r.title }}</span>
                <span class="l2-chip" :class="r.recvKind">{{ recvKindLabel[r.recvKind] }}</span>
                <span class="l2-chip" :class="r.status">{{ r.status === 'open' ? '未结清' : '已结清' }}</span>
              </div>
              <van-icon :name="expandedReceipts[r.id] ? 'arrow-up' : 'arrow-down'" />
            </div>
            <div class="recv-amounts">
              <div class="amount-cell"><span class="amount-label">应收</span>{{ formatFen(r.amountCents) }}</div>
              <div class="amount-cell"><span class="amount-label">已收</span>{{ formatFen(r.paidCents) }}</div>
              <div class="amount-cell strong"><span class="amount-label">未收</span>{{ formatFen(r.outstandingCents) }}</div>
            </div>
            <div class="recv-actions">
              <van-button v-if="r.status === 'open' && r.paidCents === 0" size="mini" plain type="danger" @click="voidReceivable(r)">作废</van-button>
              <van-button v-if="r.status === 'open'" size="mini" plain type="primary" @click="openReceipt(r)">收款 / 抵销</van-button>
            </div>

            <!-- 核销记录 -->
            <div v-if="expandedReceipts[r.id]" class="receipt-list">
              <div v-if="!receiptsByRec[r.id] || receiptsByRec[r.id].length === 0" class="receipt-empty">暂无核销记录</div>
              <div v-for="rc in receiptsByRec[r.id] || []" :key="rc.id" class="receipt-row" :class="{ voided: rc.status === 'voided' }">
                <span class="receipt-method" :class="rc.method">{{ rc.method === 'cash' ? '现金' : '抵销' }}</span>
                <span class="receipt-amount">{{ rc.method === 'cash' ? '+' : '抵' }}{{ formatFen(rc.amountCents) }}</span>
                <span class="receipt-date">{{ rc.receiptDate }}</span>
                <span v-if="rc.status === 'voided'" class="l2-chip stopped">已作废</span>
                <van-button
                  v-if="rc.status === 'normal'"
                  size="mini"
                  plain
                  type="warning"
                  @click="voidReceipt(rc)"
                >作废</van-button>
              </div>
            </div>
          </div>
        </div>
        </div>
      </div>
    </van-popup>

    <!-- 导出选择 -->
    <van-action-sheet
      v-model:show="showExportMenu"
      title="导出往来数据"
      :actions="exportActions"
      @select="onExportSelect"
      @cancel="showExportMenu = false"
    />

    <!-- 新增/编辑往来单位 -->
    <van-dialog v-model:show="showAddParty" :title="editParty ? '编辑单位' : '新增往来单位'" show-cancel-button @confirm="saveParty">
      <van-field v-model="partyForm.name" label="单位名称" placeholder="如：XX公司 / XX合作社" :rules="[{ required: true }]" />
      <van-field label="单位类型">
        <template #input>
          <select v-model="partyForm.type" class="party-type-select" @change="onPartyTypeChange">
            <option value="flow">流转企业</option>
            <option value="invest">投资公司</option>
            <option value="other">其它单位</option>
          </select>
        </template>
      </van-field>
      <van-field v-model="partyForm.contactPhone" label="联系电话" placeholder="联系电话（可选）" />
      <van-field v-if="partyForm.type === 'flow'" v-model="partyForm.areaYuan" type="number" label="流转面积" placeholder="如 120.5（亩）" inputmode="decimal" />
      <van-field v-if="partyForm.type === 'flow'" v-model="partyFlowStdYuan" type="number" label="年度流转费" placeholder="如 50000" inputmode="decimal" />
      <van-field v-if="partyForm.type === 'invest'" label="投资金额">
        <template #input>
          <span class="invest-readonly">{{ investAmountDisplay }}</span>
        </template>
      </van-field>
      <van-field v-if="partyForm.type === 'invest'" v-model="partyDividendYuan" type="number" label="年度投资收益" placeholder="如 80000" inputmode="decimal" />
      <div v-if="partyForm.type === 'other'" class="dialog-tip">其它单位不设年度标准，需要时手动登记应收</div>
      <div class="dialog-tip" v-if="!editParty && partyForm.type !== 'other'">
        投资金额自动取长期投资同名公司累计投出（不可手改）；这里的金额是年度标准，一键结转时按类型生成应收
      </div>
      <van-field v-model="partyForm.note" label="备注" placeholder="备注（可选）" />
    </van-dialog>

    <!-- 年度标准就地修改 -->
    <van-dialog v-model:show="showStdEditDialog" :title="stdEditTitle" show-cancel-button @confirm="saveStdFromDetail">
      <van-field v-model="stdEditYuan" type="number" :label="stdEditLabel" placeholder="0.00" inputmode="decimal" />
      <div class="dialog-tip">保存后作为该单位年度结转的标准金额</div>
    </van-dialog>

    <!-- 批量计提 / 流转费标准 -->
    <van-popup v-model:show="showAccrue" position="bottom" round closeable style="max-height: 92vh">
      <div class="accrue-popup">
        <van-tabs v-model:active="accrueTab">
          <!-- 年度结转：预览单位标准数据后再确认 -->
          <van-tab title="年度结转" name="accrue">
            <div class="accrue-body">
              <van-field v-model="accrueYear" type="digit" label="年度" placeholder="如 2026" />
              <div class="accrue-save">
                <van-button round block type="primary" plain :loading="previewLoading" @click="previewAccrue">生成预览（从各单位年度标准带数据）</van-button>
              </div>
              <div v-if="previewItems.length > 0" class="preview-list">
                <div v-for="it in previewItems" :key="it.kind + '-' + it.partyId" class="std-row">
                  <span class="std-name">{{ it.partyName }}（{{ it.kind === 'rent' ? '流转费' : '投资收益' }}）</span>
                  <span class="std-amount">{{ formatFen(it.amountCents) }}</span>
                  <span class="preview-state" :class="{ dup: it.exists }">{{ it.exists ? '已存在跳过' : '将新增' }}</span>
                </div>
                <div v-if="previewItems.length === 0" class="accrue-empty">该年度没有可结转的标准数据</div>
              </div>
              <div v-else-if="previewLoaded" class="accrue-empty">该年度没有可结转的标准数据（先到单位资料/年度标准里设好金额）</div>
              <div class="accrue-save">
                <van-button
                  round block type="primary" :loading="accruing" :disabled="previewNewCount === 0"
                  @click="accrueNow"
                >确认结转（将新增 {{ previewNewCount }} 条）</van-button>
              </div>
            </div>
          </van-tab>

          <!-- 年度标准 + 一键结转（按单位类型：流转费 / 投资收益） -->
          <van-tab title="年度标准" name="std">
            <div class="accrue-body">
              <div class="std-form">
                <van-field
                  :model-value="stdForm.partyName || '请选择单位'"
                  is-link readonly label="单位"
                  @click="openAccrueUnit('std', 0)"
                  class="only-mobile"
                />
                <NativeSelect
                  label="单位"
                  placeholder="请选择单位"
                  v-model="stdForm.partyId"
                  :options="accrueUnitActions"
                />
                <van-field v-model="stdForm.amountYuan" type="number" :label="stdAmountLabel" placeholder="0.00" inputmode="decimal" />
                <div v-if="stdKindError" class="dialog-tip">该单位是“其它单位”，不参与年度结转；需要时手动登记应收</div>
                <van-button size="small" round block type="primary" :loading="savingStd" @click="saveStandard">保存标准</van-button>
              </div>
              <div class="std-list">
                <div v-for="s in standards" :key="s.id" class="std-row">
                  <span class="std-name">{{ s.partyName }}（{{ s.recvKind === 'dividend' ? '年度分红' : '年度流转费' }}）</span>
                  <span class="std-amount">{{ formatFen(s.amountCents) }}</span>
                  <van-switch :model-value="s.active" size="20" @update:model-value="(v:boolean) => toggleStandard(s, v)" />
                </div>
                <div v-if="standards.length === 0" class="accrue-empty">还没有年度标准，先在上方添加</div>
              </div>
              <div class="accrue-save">
                <van-button round block type="primary" :loading="accruing" @click="accrueNow">
                  一键结转 {{ accrueYear }} 年度（流转费+投资收益）
                </van-button>
              </div>
            </div>
          </van-tab>
        </van-tabs>
      </div>
    </van-popup>

    <!-- 批量计提/标准：单位选择 -->
    <van-action-sheet
      v-model:show="showAccrueUnitPicker"
      title="选择单位"
      :actions="accrueUnitActions"
      @select="onAccrueUnitSelect"
      @cancel="showAccrueUnitPicker = false"
    />

    <!-- 登记应收 -->
    <van-dialog v-model:show="showAddReceivable" title="登记应收" show-cancel-button @confirm="saveReceivable">
      <van-field v-if="currentParty" :model-value="currentParty.name" label="单位" readonly />
      <van-field label="类别">
        <template #input>
          <van-radio-group v-model="recvForm.recvKind" direction="horizontal">
            <van-radio name="rent">流转费</van-radio>
            <van-radio name="dividend">投资收益</van-radio>
            <van-radio name="other">其他</van-radio>
          </van-radio-group>
        </template>
      </van-field>
      <van-field v-model="recvForm.title" label="事由" placeholder="如：2026年度土地流转费" :rules="[{ required: true }]" />
      <van-field v-model="recvForm.amount" label="应收金额" type="number" placeholder="0.00" inputmode="decimal" />
      <van-field v-model="recvForm.note" label="备注" placeholder="备注（可选）" />
    </van-dialog>

    <!-- 收款 / 抵销 -->
    <van-popup v-model:show="showReceipt" position="bottom" round closeable style="max-height: 92vh">
      <div class="receipt-popup">
        <div class="popup-title">收款 / 抵销</div>
        <van-field label="应收单">
          <template #input>
            <div class="static-text">
              {{ currentReceivable?.title }}（未收 {{ currentReceivable ? formatFen(currentReceivable.outstandingCents) : '' }}）
            </div>
          </template>
        </van-field>
        <van-field label="方式">
          <template #input>
            <van-radio-group v-model="receiptForm.method" direction="horizontal">
              <van-radio name="cash">现金入账</van-radio>
              <van-radio name="offset">抵销</van-radio>
            </van-radio-group>
          </template>
        </van-field>
        <van-field v-model="receiptForm.amount" label="金额" type="number" placeholder="0.00" inputmode="decimal" />
        <van-field v-model="receiptForm.date" label="日期" placeholder="YYYY-MM-DD" />
        <van-field v-if="receiptForm.method === 'offset'" class="only-mobile">
          <template #label>抵销流水</template>
          <template #input>
            <div class="static-text" v-if="receiptForm.txnLabel">{{ receiptForm.txnLabel }}</div>
            <van-button v-else size="mini" type="primary" plain @click="loadOffsetTxns">选择分红支出流水</van-button>
          </template>
        </van-field>
        <NativeSelect
          v-if="receiptForm.method === 'offset'"
          label="抵销流水"
          placeholder="选择分红支出流水"
          :model-value="receiptForm.txnId"
          :options="offsetTxnOptions"
          @update:model-value="(v:number|string|null) => { receiptForm.txnId = (v == null ? null : Number(v)); const o=offsetTxnOptions.find(x=>x.value===Number(v)); receiptForm.txnLabel = o ? o.name : ''; }"
        />
        <div v-if="receiptForm.method === 'cash'" class="dialog-tip">
          现金收款将自动记一笔银行收入
          <span v-if="cashCategoryResolved">{{ cashCategoryResolved }}</span>
          <template v-else>（需要选择入账科目）</template>
        </div>
        <van-field
          v-if="receiptForm.method === 'cash'"
          :model-value="receiptForm.categoryName || '请选择入账科目'"
          is-link
          readonly
          label="入账科目"
          @click="openIncomeCatPicker()"
          class="only-mobile"
        />
        <NativeSelect
          v-if="receiptForm.method === 'cash'"
          label="入账科目"
          placeholder="请选择入账科目"
          :model-value="receiptForm.categoryId"
          :options="incomeCatOptions"
          @update:model-value="(v:number|string|null) => { receiptForm.categoryId = (v == null ? null : Number(v)); const o=incomeCatOptions.find(x=>x.value===Number(v)); receiptForm.categoryName = o ? o.name : ''; }"
        />
        <van-field v-model="receiptForm.note" label="备注" placeholder="备注（可选）" />
        <div class="receipt-save">
          <van-button round block type="primary" :disabled="!canSubmitReceipt" :loading="savingReceipt" @click="saveReceipt">
            保存核销
          </van-button>
        </div>
      </div>
    </van-popup>

    <!-- 入账科目选择器 -->
    <van-action-sheet
      v-model:show="showIncomeCatPicker"
      title="选择入账科目"
      :actions="incomeCatOptions"
      @select="onIncomeCatSelect"
      @cancel="showIncomeCatPicker = false"
    />
    <!-- 抵销流水选择器 -->
    <van-action-sheet
      v-model:show="showTxnPicker"
      title="选择分红支出流水"
      :actions="offsetTxnOptions"
      @select="onTxnSelect"
      @cancel="showTxnPicker = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '../../lib/http'
import { showToast, showDialog } from 'vant'
import type {
  Category, Party, Receivable, Receipt, ReceivableDetail,
  ReceivableListResponse, Transaction, ApiResponse, RecvKind, AccrualStandard, AccrueResult, AccruePreview, AccruePreviewItem,
} from '../../types/api'
import { formatFen, todayStr, recvKindLabel } from '../../types/api'
import NativeSelect from '../../components/NativeSelect.vue'
import { downloadExport } from '../../lib/download'

const loading = ref(false)
const detailLoading = ref(false)
const parties = ref<Party[]>([])

// ---- 往来导出 ----
const showExportMenu = ref(false)
const exportActions = [
  { name: '导出单位基本情况（Excel）', value: 'parties' },
  { name: '导出欠款明细（Excel）', value: 'receivables' },
]
async function onExportSelect(action: { name: string; value: 'parties' | 'receivables' }) {
  showExportMenu.value = false
  try {
    await downloadExport({}, action.value, 'xlsx')
    showToast('导出成功')
  } catch (e: any) {
    showToast(e.message || '导出失败')
  }
}

// ---- 往来单位 ----
const currentParty = ref<Party | null>(null)
const receivableItems = ref<Receivable[]>([])
const receiptsByRec = ref<Record<number, Receipt[]>>({})
const expandedReceipts = ref<Record<number, boolean>>({})

const showAddParty = ref(false)
const editParty = ref(false)
const partyForm = ref({ id: 0, name: '', type: 'flow' as 'flow' | 'invest' | 'other', contactPhone: '', areaYuan: '', note: '' })

const partyTypeLabel: Record<string, string> = { flow: '流转企业', invest: '投资公司', other: '其它单位' }

// 新增/编辑单位弹窗里的年度标准输入
const partyFlowStdYuan = ref('')
const partyDividendYuan = ref('')
const investAmountDisplay = computed(() => {
  if (editParty.value && currentParty.value?.type === 'invest') {
    return formatFen(currentParty.value.investAmountCents || 0)
  }
  return formatFen(0)
})
function onPartyTypeChange() {
  partyFlowStdYuan.value = ''
  partyDividendYuan.value = ''
}

const showAddReceivable = ref(false)
const recvForm = ref({
  recvKind: 'rent' as RecvKind,
  title: '',
  amount: '',
  note: '',
})

// ---- 收款/抵销 ----
const showReceipt = ref(false)
const currentReceivable = ref<Receivable | null>(null)
const receiptForm = ref({
  method: 'cash' as 'cash' | 'offset',
  amount: '',
  date: todayStr(),
  categoryId: null as number | null,
  categoryName: '',
  txnId: null as number | null,
  txnLabel: '',
  note: '',
})
const savingReceipt = ref(false)

const cats = ref<Category[]>([])
const incomeCatOptions = ref<{ name: string; value: number }[]>([])
const showIncomeCatPicker = ref(false)

const showTxnPicker = ref(false)
const offsetTxnOptions = ref<{ name: string; value: number }[]>([])
const catNameById = computed<Record<number, string>>(() => {
  const m: Record<number, string> = {}
  for (const l1 of cats.value) {
    if (l1.children) {
      for (const l2 of l1.children) m[l2.id] = `${l1.name} / ${l2.name}`
    }
  }
  return m
})

const cashCategoryResolved = computed(() => {
  if (receiptForm.value.method !== 'cash') return ''
  const id = receiptForm.value.categoryId
  if (id == null) return ''
  return '（入账科目：' + (catNameById.value[id] || `#${id}`) + '）'
})

onMounted(async () => {
  await Promise.all([loadParties(), loadCategories()])
})

// ---- 单位列表 ----
async function loadParties() {
  loading.value = true
  try {
    const res = await api.get<ApiResponse<Party[]>>('/parties')
    parties.value = res.data
    void loadKindOwes()
  } catch {
    parties.value = []
  } finally {
    loading.value = false
  }
}

// ---- 分类欠款 banner：总计 / 流转费 / 投资收益 ----
const rentOweTotal = ref(0)
const dividendOweTotal = ref(0)
const activeTypeFilter = ref<'all' | 'flow' | 'invest'>('all')

const totalOwe = computed(() => parties.value.reduce((s, p) => s + (p.outstandingCents || 0), 0))

const displayParties = computed(() => {
  if (activeTypeFilter.value === 'flow') return parties.value.filter(p => p.type === 'flow')
  if (activeTypeFilter.value === 'invest') return parties.value.filter(p => p.type === 'invest')
  return parties.value
})

function toggleBanner(kind: 'all' | 'flow' | 'invest') {
  activeTypeFilter.value = activeTypeFilter.value === kind ? 'all' : kind
}

async function loadKindOwes() {
  try {
    const res = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', { status: 'open', pageSize: 1000 })
    let rentTotal = 0
    let dividendTotal = 0
    for (const it of res.data.items || []) {
      if (it.outstandingCents <= 0) continue
      if (it.recvKind === 'rent') rentTotal += it.outstandingCents
      else if (it.recvKind === 'dividend') dividendTotal += it.outstandingCents
    }
    rentOweTotal.value = rentTotal
    dividendOweTotal.value = dividendTotal
  } catch {
    rentOweTotal.value = 0
    dividendOweTotal.value = 0
  }
}

function openAddParty() {
  editParty.value = false
  partyForm.value = { id: 0, name: '', type: 'flow', contactPhone: '', areaYuan: '', note: '' }
  partyFlowStdYuan.value = ''
  partyDividendYuan.value = ''
  showAddParty.value = true
}

function openAddPartyEdit() {
  if (!currentParty.value) return
  editParty.value = true
  partyForm.value = {
    id: currentParty.value.id,
    name: currentParty.value.name,
    type: currentParty.value.type || 'flow',
    contactPhone: currentParty.value.contactPhone || '',
    areaYuan: currentParty.value.areaMu ? String(currentParty.value.areaMu) : '',
    note: currentParty.value.note || '',
  }
  partyFlowStdYuan.value = ''
  partyDividendYuan.value = ''
  const kind = currentParty.value.type === 'invest' ? 'dividend' : currentParty.value.type === 'other' ? null : 'rent'
  if (kind) {
    const s = standards.value.find(x => x.partyId === currentParty.value!.id && x.recvKind === kind)
    if (s) {
      const yuan = (s.amountCents / 100).toFixed(2)
      if (kind === 'dividend') partyDividendYuan.value = yuan
      else partyFlowStdYuan.value = yuan
    }
  }
  showAddParty.value = true
}

// 保存标准金额（无输入不覆盖旧值）
async function syncPartyStandard(partyId: number, kind: 'rent' | 'dividend', yuan: string) {
  const amount = Math.round(parseFloat(yuan || '0') * 100)
  if (amount <= 0) return
  await api.post('/recv-standards', { partyId, recvKind: kind, amountCents: amount })
}

async function saveParty() {
  if (!partyForm.value.name) {
    showToast('请填写单位名称')
    return
  }
  try {
    const payload: Record<string, unknown> = { name: partyForm.value.name, type: partyForm.value.type }
    payload.contactPhone = partyForm.value.contactPhone.trim()
    const area = parseFloat(partyForm.value.areaYuan || '0')
    payload.areaMu = isNaN(area) ? 0 : area
    if (partyForm.value.note) payload.note = partyForm.value.note
    let id = partyForm.value.id
    if (editParty.value) {
      await api.put(`/parties/${id}`, payload)
      showToast('更新成功')
    } else {
      const res = await api.post<ApiResponse<Party>>('/parties', payload)
      id = res.data?.id ?? 0
      showToast('创建成功')
    }
    if (partyForm.value.type === 'flow') {
      await syncPartyStandard(id, 'rent', partyFlowStdYuan.value)
    } else if (partyForm.value.type === 'invest') {
      await syncPartyStandard(id, 'dividend', partyDividendYuan.value)
    }
    await loadStandards()
    await loadParties()
    if (currentParty.value) {
      const updated = parties.value.find(p => p.id === currentParty.value!.id)
      if (updated) currentParty.value = updated
    }
  } catch (e: any) {
    showDialog({ title: '保存失败', message: e.message || '保存失败，请重试' })
  }
}

// ---- 详情（弹窗） ----
const showPartyDetail = ref(false)
function onPartyDetailClosed() {
  currentParty.value = null
  receivableItems.value = []
}

async function openDetail(p: Party) {
  currentParty.value = p
  showPartyDetail.value = true
  detailLoading.value = true
  receivableItems.value = []
  receiptsByRec.value = {}
  expandedReceipts.value = {}
  void loadStandards()
  try {
    const res = await api.get<ApiResponse<ReceivableListResponse>>('/receivables', { partyId: p.id, pageSize: 200 })
    receivableItems.value = res.data.items || []
    for (const r of receivableItems.value) {
      if (r.status === 'open' || r.paidCents > 0) {
        await loadReceipts(r)
      }
    }
    // 刷新对象欠款合计（应收单核销会改变欠款）
    await loadParties()
    const fresh = parties.value.find(x => x.id === p.id)
    if (fresh) currentParty.value = fresh
  } catch {
    receivableItems.value = []
  } finally {
    detailLoading.value = false
  }
}

function closeDetail() {
  showPartyDetail.value = false
  currentParty.value = null
}

async function loadReceipts(r: Receivable) {
  try {
    const res = await api.get<ApiResponse<ReceivableDetail>>(`/receivables/${r.id}`)
    receiptsByRec.value[r.id] = res.data.receipts || []
  } catch {
    receiptsByRec.value[r.id] = []
  }
}

function toggleReceipts(r: Receivable) {
  expandedReceipts.value[r.id] = !expandedReceipts.value[r.id]
}

// ---- 登记应收 ----
function openAddReceivable() {
  recvForm.value = { recvKind: 'rent', title: '', amount: '', note: '' }
  showAddReceivable.value = true
}

async function saveReceivable() {
  if (!currentParty.value) return
  const amountCents = parseFen(recvForm.value.amount)
  if (amountCents <= 0) {
    showToast('请填写正确的金额')
    return
  }
  if (!recvForm.value.title) {
    showToast('请填写事由')
    return
  }
  try {
    const payload: Record<string, unknown> = {
      partyId: currentParty.value.id,
      recvKind: recvForm.value.recvKind,
      title: recvForm.value.title,
      amountCents,
    }
    if (recvForm.value.note) payload.note = recvForm.value.note
    await api.post('/receivables', payload)
    showToast('登记成功')
    showAddReceivable.value = false
    await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '登记失败', message: e.message || '登记失败，请重试' })
  }
}

// ---- 收款 / 抵销 ----
function openReceipt(r: Receivable) {
  currentReceivable.value = r
  receiptForm.value = {
    method: 'cash',
    amount: String((r.outstandingCents / 100).toFixed(2)),
    date: todayStr(),
    categoryId: null,
    categoryName: '',
    txnId: null,
    txnLabel: '',
    note: '',
  }
  showReceipt.value = true
  void fetchOffsetOptions()
}

const canSubmitReceipt = computed(() => {
  const amount = parseFen(receiptForm.value.amount)
  if (amount <= 0) return false
  if (receiptForm.value.method === 'offset' && !receiptForm.value.txnId) return false
  if (receiptForm.value.method === 'cash' && !receiptForm.value.categoryId) return false
  return true
})

async function saveReceipt() {
  const amountCents = parseFen(receiptForm.value.amount)
  if (!currentReceivable.value || amountCents <= 0) {
    showToast('请填写正确的金额')
    return
  }
  savingReceipt.value = true
  try {
    const payload: Record<string, unknown> = {
      amountCents,
      receiptDate: receiptForm.value.date,
      method: receiptForm.value.method,
    }
    if (receiptForm.value.method === 'cash') {
      payload.categoryId = receiptForm.value.categoryId
    }
    if (receiptForm.value.method === 'offset') {
      payload.txnId = receiptForm.value.txnId
    }
    if (receiptForm.value.note) payload.note = receiptForm.value.note
    await api.post(`/receivables/${currentReceivable.value.id}/receipts`, payload)
    showToast('核销成功')
    showReceipt.value = false
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '核销失败', message: e.message || '核销失败，请重试' })
  } finally {
    savingReceipt.value = false
  }
}

// 预载发放支出流水选项（桌面下拉用；不主动弹层）
async function fetchOffsetOptions() {
  offsetTxnOptions.value = []
  try {
    const res = await api.get<ApiResponse<{ items: Transaction[] }>>('/transactions', { pageSize: 50 })
    const options: { name: string; value: number }[] = []
    for (const t of res.data.items || []) {
      if (t.direction === 'expense' && t.status === 'normal') {
        const cat = catNameById.value[t.categoryId] || `#${t.categoryId}`
        const note = t.note ? ` ${t.note}` : ''
        options.push({ name: `${t.txnDate} ${cat}${note} ${formatFen(t.amountCents)}`, value: t.id })
      }
    }
    offsetTxnOptions.value = options
  } catch {
    offsetTxnOptions.value = []
  }
}

// 抵销需要选择发放支出流水（移动端弹 action sheet）
async function loadOffsetTxns() {
  await fetchOffsetOptions()
  showTxnPicker.value = true
  if (offsetTxnOptions.value.length === 0) showToast('近期没有可抵销的支出流水（如分红发放）')
}

function onTxnSelect(action: { name: string; value: number }) {
  receiptForm.value.txnId = action.value
  receiptForm.value.txnLabel = action.name
  showTxnPicker.value = false
}

async function voidReceipt(rc: Receipt) {
  try {
    await showDialog({
      title: '确认作废',
      message: `作废这笔${rc.method === 'cash' ? '现金核销' : '抵销'}（${formatFen(rc.amountCents)}）？若为现金核销，对应的银行收入流水将同步作废。`,
      showCancelButton: true,
    })
  } catch {
    return
  }
  try {
    await api.put(`/receipts/${rc.id}`, { status: 'voided' })
    showToast('已作废')
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showToast(e.message || '操作失败')
  }
}

// ---- 年度结转 / 标准 ----
const showAccrue = ref(false)
const accrueTab = ref('accrue')
const accrueYear = ref(String(new Date().getFullYear()))
const accrueTitle = ref('')
const savingAccrue = ref(false)
const accrueRows = ref<{ partyId: number | null; partyName: string; recvKind: RecvKind; amountYuan: string }[]>([
  { partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' },
])

// 年度结转预览（从单位年度标准带数据）
const previewItems = ref<AccruePreviewItem[]>([])
const previewLoading = ref(false)
const previewLoaded = ref(false)
const previewNewCount = computed(() => previewItems.value.filter(it => !it.exists).length)

const showAccrueUnitPicker = ref(false)
let accruePickFor = 'row' as 'row' | 'std'
let accruePickRow = 0

const accrueUnitActions = computed(() =>
  parties.value.map(p => ({ name: p.name, value: p.id })),
)

const standards = ref<AccrualStandard[]>([])
const stdForm = ref<{ partyId: number | null; partyName: string; amountYuan: string }>({ partyId: null, partyName: '', amountYuan: '' })
const stdSelectedType = computed(() => {
  const p = parties.value.find(x => x.id === stdForm.value.partyId)
  return p?.type || 'flow'
})
const stdAmountLabel = computed(() => (stdSelectedType.value === 'invest' ? '年度应得分红' : '年度流转费'))
const stdKindError = computed(() => {
  return stdForm.value.partyId != null && stdSelectedType.value === 'other'
})
const savingStd = ref(false)
const accruing = ref(false)

async function previewAccrue() {
  const year = parseInt(accrueYear.value || '0', 10)
  if (!year || year < 2000 || year > 2100) {
    showToast('请填写正确年度')
    return
  }
  previewLoading.value = true
  try {
    const res = await api.get<ApiResponse<AccruePreview>>('/recv-standards/preview', { year })
    previewItems.value = res.data?.items || []
    previewLoaded.value = true
    if (previewItems.value.length === 0) showToast('没有可结转的标准数据')
  } catch (e: any) {
    showToast(e.message || '生成预览失败')
  } finally {
    previewLoading.value = false
  }
}

async function openAccrue() {
  accrueTab.value = 'accrue'
  accrueYear.value = String(new Date().getFullYear())
  accrueTitle.value = ''
  accrueRows.value = [{ partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' }]
  stdForm.value = { partyId: null, partyName: '', amountYuan: '' }
  previewItems.value = []
  previewLoaded.value = false
  showAccrue.value = true
  await loadStandards()
  await loadParties()
}

// 当前单位按类型的年度标准（flow→流转费 / invest→应得分红）
const currentPartyStd = computed(() => {
  const p = currentParty.value
  if (!p || p.type === 'other') return null
  const kind = p.type === 'invest' ? 'dividend' : 'rent'
  return standards.value.find(s => s.partyId === p.id && s.recvKind === kind) || null
})

// 年度标准就地修改（单位详情“去修改”直接弹窗）
const showStdEditDialog = ref(false)
const stdEditYuan = ref('')
const stdEditLabel = computed(() => (currentParty.value?.type === 'invest' ? '年度应得分红' : '年度流转费'))
const stdEditTitle = computed(() => (currentParty.value?.type === 'invest' ? '修改年度应得分红' : '修改年度流转费'))

function openStdForParty() {
  stdEditYuan.value = currentPartyStd.value ? (currentPartyStd.value.amountCents / 100).toFixed(2) : ''
  showStdEditDialog.value = true
}

async function saveStdFromDetail() {
  const amount = Math.round(parseFloat(stdEditYuan.value || '0') * 100)
  const p = currentParty.value
  if (!p || p.type === 'other') return
  if (amount <= 0) {
    showToast('请填写正确的金额')
    return
  }
  try {
    await api.post('/recv-standards', {
      partyId: p.id,
      recvKind: p.type === 'invest' ? 'dividend' : 'rent',
      amountCents: amount,
    })
    showStdEditDialog.value = false
    showToast('已保存')
    await loadStandards()
    if (currentParty.value) {
      const updated = parties.value.find(x => x.id === currentParty.value!.id)
      if (updated) currentParty.value = updated
    }
  } catch (e: any) {
    showToast(e.message || '保存失败')
  }
}

function addAccrueRow() {
  accrueRows.value.push({ partyId: null, partyName: '', recvKind: 'rent', amountYuan: '' })
}

function openAccrueUnit(kind: 'row' | 'std', rowIndex: number) {
  accruePickFor = kind
  accruePickRow = rowIndex
  showAccrueUnitPicker.value = true
}

function onAccrueUnitSelect(action: { name: string; value: number }) {
  if (accruePickFor === 'row') {
    const row = accrueRows.value[accruePickRow]
    if (row) {
      row.partyId = action.value
      row.partyName = action.name
    }
  } else {
    stdForm.value.partyId = action.value
    stdForm.value.partyName = action.name
  }
  showAccrueUnitPicker.value = false
}

async function saveBatch() {
  const year = parseInt(accrueYear.value || '0', 10)
  if (!year || year < 2000 || year > 2100) {
    showToast('请填写正确年度')
    return
  }
  if (!accrueTitle.value.trim()) {
    showToast('请填写事由')
    return
  }
  const items: { partyId: number; recvKind: RecvKind; amountCents: number }[] = []
  for (const row of accrueRows.value) {
    const amount = parseFen(row.amountYuan)
    if (!row.partyId || amount <= 0) {
      showToast('请补全各单位与金额')
      return
    }
    items.push({ partyId: row.partyId, recvKind: row.recvKind, amountCents: amount })
  }
  savingAccrue.value = true
  try {
    const res = await api.post<ApiResponse<AccrueResult>>('/receivables/batch', {
      recvYear: year,
      title: accrueTitle.value.trim(),
      items,
    })
    showToast(`新增 ${res.data.created} 条${res.data.skipped ? `，跳过 ${res.data.skipped} 条` : ''}`)
    showAccrue.value = false
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showDialog({ title: '计提失败', message: e.message || '批量计提失败' })
  } finally {
    savingAccrue.value = false
  }
}

async function loadStandards() {
  try {
    const res = await api.get<ApiResponse<AccrualStandard[]>>('/recv-standards')
    standards.value = res.data || []
  } catch {
    standards.value = []
  }
}

async function saveStandard() {
  const amount = parseFen(stdForm.value.amountYuan)
  if (!stdForm.value.partyId || amount <= 0) {
    showToast('请选择单位并填写金额')
    return
  }
  if (stdKindError.value) {
    showToast('其它单位不设年度标准，需要时手动登记应收')
    return
  }
  savingStd.value = true
  try {
    const kind = stdSelectedType.value === 'invest' ? 'dividend' : 'rent'
    await api.post('/recv-standards', {
      partyId: stdForm.value.partyId,
      recvKind: kind,
      amountCents: amount,
    })
    showToast('已保存')
    stdForm.value = { partyId: null, partyName: '', amountYuan: '' }
    await loadStandards()
  } catch (e: any) {
    showToast(e.message || '保存失败')
  } finally {
    savingStd.value = false
  }
}

async function toggleStandard(s: AccrualStandard, v: boolean) {
  try {
    await api.put(`/recv-standards/${s.id}`, { active: v })
    s.active = v
  } catch (e: any) {
    showToast(e.message || '操作失败')
  }
}

async function accrueNow() {
  const year = parseInt(accrueYear.value || '0', 10)
  if (!year) {
    showToast('请填写年度')
    return
  }
  accruing.value = true
  try {
    let created = 0
    let skipped = 0
    for (const kind of ['rent', 'dividend'] as const) {
      const res = await api.post<ApiResponse<AccrueResult>>('/recv-standards/accrue', {
        year,
        kind,
        title: kind === 'rent' ? `${year}年度土地流转费` : `${year}年度投资收益`,
      })
      created += res.data.created
      skipped += res.data.skipped
    }
    showToast(`结转完成：新增 ${created} 条${skipped ? `，跳过 ${skipped} 条` : ''}`)
    previewItems.value = []
    previewLoaded.value = false
    await loadParties()
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showToast(e.message || '结转失败')
  } finally {
    accruing.value = false
  }
}

// ---- 应收作废（仅未收款，作废后可按标准重结） ----
async function voidReceivable(r: Receivable) {
  try {
    await showDialog({
      title: '确认作废',
      message: `作废应收「${r.title}」（${formatFen(r.amountCents)}）？仅未收款的应收可作废，作废后可在年度标准里重新结转。`,
      showCancelButton: true,
    })
  } catch {
    return // 用户取消
  }
  try {
    await api.put(`/receivables/${r.id}/void`)
    showToast('已作废')
    await loadParties()
    if (currentParty.value) await openDetail(currentParty.value)
  } catch (e: any) {
    showToast(e.message || '作废失败')
  }
}

// ---- 科目与选择器 ----
async function loadCategories() {
  try {
    const res = await api.get<ApiResponse<Category[]>>('/categories')
    cats.value = res.data
    const options: { name: string; value: number }[] = []
    for (const l1 of res.data) {
      if (l1.children) {
        for (const l2 of l1.children) {
          if (l2.status === 'active' && l2.kind === 'equity') {
            options.push({ name: `${l1.name} / ${l2.name}`, value: l2.id })
          }
        }
      }
    }
    incomeCatOptions.value = options
  } catch {
    cats.value = []
  }
}

function openIncomeCatPicker() {
  showIncomeCatPicker.value = true
}

function onIncomeCatSelect(action: { name: string; value: number }) {
  receiptForm.value.categoryId = action.value
  receiptForm.value.categoryName = action.name
  showIncomeCatPicker.value = false
}

function parseFen(yuan: string): number {
  const cleaned = yuan.replace(/[^0-9.]/g, '')
  const num = parseFloat(cleaned)
  if (isNaN(num)) return 0
  return Math.round(num * 100)
}
</script>

<style scoped>
.contacts-page {
  padding: 16px;
  padding-bottom: 60px;
  min-height: 100vh;
  background: #f7f7f5;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.page-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: #2c2c2a;
  margin: 0;
}

.detail-title {
  font-size: 16px;
  font-weight: 500;
  flex: 1;
  text-align: center;
}

.back-icon {
  font-size: 18px;
  color: #5f5e5a;
  cursor: pointer;
}

.header-actions {
  display: flex;
  gap: 4px;
}

.kind-tabs {
  margin-bottom: 8px;
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
  color: #8f8e88;
}

.empty-state p {
  margin-bottom: 16px;
}

.party-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.party-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-radius: 12px;
  padding: 14px 16px;
  cursor: pointer;
}

.party-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.party-name {
  font-size: 15px;
  font-weight: 500;
}

.owed-value {
  font-size: 16px;
  font-weight: 600;
  color: #a32d2d;
  font-variant-numeric: tabular-nums;
}

.owed-value.zero {
  color: #185fa5;
}

.owed-label {
  font-size: 11px;
  color: #8f8e88;
  text-align: right;
}

.party-summary {
  display: flex;
  align-items: center;
  gap: 24px;
  background: #fff;
  border-radius: 12px;
  padding: 14px 16px;
  margin-bottom: 12px;
}

.summary-value {
  font-size: 18px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.summary-value.zero {
  color: #185fa5;
}

.summary-label {
  font-size: 11px;
  color: #8f8e88;
}

.recv-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.recv-card {
  background: #fff;
  border-radius: 12px;
  padding: 12px 16px;
}

.recv-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
}

.recv-title-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.recv-title {
  font-size: 14px;
  font-weight: 500;
}

.recv-amounts {
  display: flex;
  gap: 24px;
  padding: 8px 0;
}

.amount-cell {
  font-size: 14px;
  font-variant-numeric: tabular-nums;
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.amount-cell.strong {
  font-weight: 600;
  color: #a32d2d;
}

.amount-label {
  font-size: 11px;
  color: #8f8e88;
}

.recv-actions {
  display: flex;
  gap: 4px;
  justify-content: flex-end;
}

.receipt-list {
  border-top: 1px solid #f0f0eb;
  margin-top: 8px;
  padding-top: 4px;
}

.receipt-empty {
  text-align: center;
  color: #8f8e88;
  font-size: 12px;
  padding: 8px 0;
}

.receipt-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
}

.receipt-row.voided {
  opacity: 0.6;
}

.receipt-method {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #185fa5;
  color: #185fa5;
}

.receipt-method.offset {
  border-color: #185fa5;
  color: #185fa5;
}

.receipt-amount {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.receipt-date {
  font-size: 12px;
  color: #8f8e88;
}

.l2-chip {
  font-size: 11px;
  border-radius: 99px;
  padding: 1px 8px;
  border: 1px solid #e3e2dd;
  color: #8f8e88;
}

.l2-chip.household { border-color: #185fa5; color: #185fa5; }
.l2-chip.unit { border-color: #185fa5; color: #185fa5; }
.l2-chip.rent { border-color: #7a4f0f; color: #7a4f0f; background: #fdf3e3; }
.l2-chip.dividend { border-color: #185fa5; color: #185fa5; background: #e6f1fb; }
.l2-chip.other { border-color: #8f8e88; color: #5f5e5a; }
.l2-chip.open { border-color: #e88a3a; color: #a8601a; background: #fef3e8; }
.l2-chip.closed { border-color: #185fa5; color: #185fa5; background: #e6f1fb; }
.l2-chip.stopped { background: #fcebeb; border-color: #a32d2d; color: #a32d2d; }

.receipt-popup {
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

.static-text {
  font-size: 13px;
  color: #2c2c2a;
}

.dialog-tip {
  font-size: 12px;
  color: #8f8e88;
  padding: 0 16px 8px;
  line-height: 1.5;
}

.receipt-save {
  margin: 8px 16px 0;
}

.accrue-popup {
  padding-bottom: 24px;
}

.accrue-body {
  padding: 8px 16px;
}

.accrue-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.accrue-row {
  background: #f7f7f5;
  border-radius: 8px;
  padding: 4px 8px;
}

.accrue-row-foot {
  display: flex;
  align-items: center;
  gap: 8px;
}

.accrue-add {
  padding: 8px 0;
}

.accrue-save {
  margin-top: 8px;
}

.accrue-empty {
  text-align: center;
  color: #8f8e88;
  font-size: 13px;
  padding: 16px 0;
}

.std-form {
  padding-bottom: 4px;
}

.std-list {
  margin-top: 8px;
  border-top: 1px solid #f0f0eb;
}

.std-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  border-bottom: 1px solid #f0f0eb;
}

.std-name {
  font-size: 14px;
}

.std-amount {
  font-size: 13px;
  color: #5f5e5a;
  font-variant-numeric: tabular-nums;
}

.summary-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 4px 0;
}

.summary-meta .meta-line {
  font-size: 12px;
  color: #8f8e88;
}

.l2-chip.flow { border-color: #185fa5; color: #185fa5; background: #e6f1fb; }
.l2-chip.invest { border-color: #7a4f0f; color: #7a4f0f; background: #fdf3e3; }
.l2-chip.other { border-color: #8f8e88; color: #5f5e5a; }
.l2-chip.invest-chip { border-color: #185fa5; color: #185fa5; background: #e6f1fb; }

.std-edit-link {
  color: #185fa5;
  margin-left: 6px;
  cursor: pointer;
}

.preview-list {
  margin: 4px 0;
}

.preview-state {
  font-size: 12px;
  color: #185fa5;
  white-space: nowrap;
}

.preview-state.dup {
  color: #8f8e88;
}

.party-detail-popup {
  padding-bottom: 24px;
}

.party-detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 16px 16px 0;
}

.party-detail-head .detail-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pd-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.party-type-select {
  flex: 1;
  width: 100%;
  height: 40px;
  border: 1px solid #dcdee0;
  border-radius: 8px;
  font-size: 15px;
  padding: 0 10px;
  background: #fff;
  color: #323233;
}

.owe-banners {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
}

.owe-banner {
  flex: 1 1 0%;
  border-radius: 12px;
  padding: 12px 14px;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.owe-banner.total {
  background: #185fa5;
  color: #fff;
}

.owe-banner.rent {
  background: #e6f1fb;
  color: #185fa5;
}

.owe-banner.invest {
  background: #fdf3e3;
  color: #7a4f0f;
}

.owe-banner.active {
  box-shadow: 0 0 0 2px #185fa5 inset;
}

.owe-banner .banner-label {
  font-size: 12px;
  opacity: 0.85;
}

.owe-banner .banner-value {
  font-size: 18px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  margin-top: 2px;
}

/* ---- 详情弹窗站点风格美化 ---- */
.party-detail-popup {
  background: #f7f7f5;
}

.party-detail-head {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 12px;
  padding: 20px 16px 16px;
  background: linear-gradient(135deg, #185fa5 0%, #12569b 100%);
  color: #fff;
}

.pd-title {
  display: flex;
  align-items: baseline;
  gap: 0;
  min-width: 0;
}

.party-detail-head .detail-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #fff;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pd-type-text {
  font-size: 12px;
  font-weight: 400;
  color: rgba(255, 255, 255, 0.85);
  white-space: nowrap;
  margin-left: 0;
}

.pd-type-text::before {
  content: '--- ';
}

.party-detail-head .pd-actions {
  display: flex;
  gap: 10px;
}

.pd-btn.van-button {
  flex: 1 1 0%;
  height: 36px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.9);
  background: rgba(255, 255, 255, 0.2);
  color: #fff;
  font-size: 13px;
  font-weight: 500;
}

.pd-btn.ghost.van-button {
  background: transparent;
}

.detail-block {
  margin: 10px 12px 0;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #2c2c2a;
  padding: 2px 0 8px;
}

.section-title::before {
  content: '';
  width: 4px;
  height: 14px;
  border-radius: 2px;
  background: #185fa5;
}

.party-detail-popup .party-summary {
  margin: 0;
  flex-wrap: wrap;
  gap: 10px 24px;
  box-shadow: 0 1px 4px rgba(44, 44, 42, 0.06);
}

.party-detail-popup .party-summary .summary-item {
  flex: 1 1 30%;
  min-width: 120px;
}

.party-detail-popup .party-summary .summary-meta {
  width: 100%;
  border-top: 1px dashed #e3e2dd;
  padding-top: 10px;
  margin-top: 4px;
}

.party-detail-popup .recv-list {
  padding: 0 0 16px;
  gap: 10px;
}

.party-detail-popup .recv-card {
  box-shadow: 0 1px 4px rgba(44, 44, 42, 0.06);
}

.party-detail-popup .recv-amounts {
  justify-content: space-between;
  gap: 8px;
  background: #f7f7f5;
  border-radius: 8px;
  padding: 8px 10px;
  margin-top: 8px;
}

.party-detail-popup .amount-cell {
  flex-direction: column;
  gap: 0;
  text-align: center;
}

.party-detail-popup .recv-actions {
  margin-top: 4px;
}
</style>
