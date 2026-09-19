<!--
  引导页（Onboarding）— 对齐 demo 原型（docs/onboarding-demo/index.html）
  模式：导入 Excel 快速建账 / 手工逐条录入（7 步：流转企业 → 流转企业余额 → 投资 → 余额 → 再投资 → 余额 → 其他设置+预览）
  完成后写服务端 org.onboarded=1（POST /api/onboarding/complete）
-->
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import { useAuthStore } from '../auth/store'
import type { ApiResponse, Category, Party } from '../../types/api'

const router = useRouter()
const auth = useAuthStore()

// ========== 步骤定义 ==========
type StepType = 'add-party' | 'fill-balance' | 'final'
interface StepDef {
  label: string
  type: StepType
  partyType?: 'flow' | 'invest' | 'reinvest' // add-party 步骤的默认类型
  l1Names?: string[]                           // fill-balance 步骤涉及的 L1
}
const STEPS: StepDef[] = [
  { label: '流转企业',   type: 'add-party',    partyType: 'flow' },
  { label: '流转企业余额', type: 'fill-balance', l1Names: ['土地流转费收入', '流转管理费'] },
  { label: '投资公司',   type: 'add-party',    partyType: 'invest' },
  { label: '投资公司余额', type: 'fill-balance', l1Names: ['长期投资'] },
  { label: '再投资',     type: 'add-party',    partyType: 'reinvest' },
  { label: '再投资余额',  type: 'fill-balance', l1Names: ['再投资'] },
  { label: '其他设置',   type: 'final' },
]
const step = ref(0)
const currentStep = computed(() => STEPS[step.value])
const isLast = computed(() => step.value === STEPS.length - 1)
// fill-balance 步骤绑定其前一个 add-party 的类型（用于按单位分组）
const balancePartyType = computed<'flow' | 'invest' | 'reinvest' | ''>(() => {
  const s = currentStep.value
  if (s.type !== 'fill-balance') return ''
  return STEPS[step.value - 1]?.partyType || ''
})
const canNext = computed(() => {
  const s = currentStep.value
  if (s.type === 'add-party') return getPartiesByType(s.partyType!).length > 0 || skipThisStep.value
  if (s.type === 'fill-balance') return true // 余额可选，随时跳过
  if (s.type === 'final') return true
  return false
})

// ========== 页面数据 ==========
const cats = ref<Category[]>([])
const parties = ref<Party[]>([])
const bankOpening = ref('')
const skipThisStep = ref(false)

// 建账方式：手工逐条 / 导入 Excel
const mode = ref<'manual' | 'import'>('import')
const importing = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const importResult = ref<ImportResult | null>(null)

interface ImportResult {
  summary: Record<string, { type: string; created: number; failed: number }>
  errors: { type: string; row: number; name: string; message: string }[]
  created: number
  failed: number
}
const importSummaryRows = computed(() => (importResult.value?.summary ? Object.values(importResult.value.summary) : []))

// 本地表单
const showPartyForm = ref(false)
const partyForm = ref({ name: '' })

// 本地期初输入：key = 二级科目 id（number），值 = 元字符串
const openingInputs = ref<Record<number, string>>({})

// 类型 → 目标 L1 映射（用于 autoBuild 和 fill-balance）
const TYPE_TO_L1S: Record<string, string[]> = {
  flow: ['土地流转费收入', '流转管理费'],
  invest: ['长期投资'],
  reinvest: ['再投资'],
}

// ========== 加载 ==========
async function loadCats() {
  try { cats.value = (await api.get<ApiResponse<Category[]>>('/categories')).data || [] } catch { cats.value = [] }
}
async function loadParties() {
  try { parties.value = (await api.get<ApiResponse<Party[]>>('/parties')).data || [] } catch { parties.value = [] }
}
onMounted(async () => { await loadCats(); await loadParties() })

// ========== 辅助 ==========
function findL1(name: string): Category | undefined {
  return cats.value.find(c => c.name === name && c.level === 1)
}
function getPartiesByType(t: string): Party[] {
  return parties.value.filter(p => {
    const types = p.types || ((p as any).type ? [(p as any).type] : [])
    return (types as string[]).includes(t)
  })
}

// ========== Step: add-party ==========
function openNewParty() {
  partyForm.value = { name: '' }
  showPartyForm.value = true
}
async function saveParty() {
  const name = partyForm.value.name.trim()
  if (!name) return
  const t = currentStep.value.partyType!
  try {
    // 后端：type 为单值落库，types 数组为多选兼容层（引导页单类型，两者都传以兼容新旧后端）
    await api.post('/parties', { name, type: t, types: [t] })
    // 服务器端自动在关联 L1 下建同名 L2，无需前端重复创建
    showPartyForm.value = false
    await loadParties()
    await loadCats()
  } catch (e: any) {
    if (e?.status === 409 && e?.response?.code === 'DUPLICATE_NAME') {
      const dupes: any[] = e?.response?.details || []
      if (dupes.length > 0) {
        const names = dupes.map((d: any) => (typeof d === 'string' ? d : d.name)).filter(Boolean).join('、')
        alert('以下近似重名单位与您输入高度相近：' + names + '\n\n请修改单位名称后重试。')
      } else {
        alert('该类型下已存在同名单位，请修改单位名称后重试。')
      }
    } else {
      alert('保存失败：' + (e.message || e))
    }
  }
}
async function deletePartyByType(p: Party, _t?: string) {
  if (!confirm(`确定删除单位「${p.name}」吗？将同时移除其同名科目。`)) return
  try {
    await api.del(`/parties/${p.id}`)
    await loadParties()
    await loadCats()
  } catch (e: any) {
    alert('删除失败：' + (e.message || e))
  }
}

// ========== Step: fill-balance ==========
// 本步涉及的单位同名科目（L2），剔除了系统预置/普通科目
const stepL2s = computed(() => {
  const s = currentStep.value
  if (s.type !== 'fill-balance') return []
  const t = balancePartyType.value
  const unitNames = new Set(getPartiesByType(t).map(p => p.name))
  const result: { l1Name: string; l2: Category }[] = []
  for (const l1Name of (s.l1Names || [])) {
    const l1 = findL1(l1Name)
    if (!l1) continue
    for (const l2 of l1.children || []) {
      if (unitNames.has(l2.name)) result.push({ l1Name, l2 })
    }
  }
  return result
})

// 按单位分组：每个单位一组，组内含其全部同名科目（行标签用 L1 名区分）
const unitBalances = computed(() => {
  const t = balancePartyType.value
  if (!t) return [] as { party: Party; items: { l1Name: string; l2: Category }[] }[]
  const groups: { party: Party; items: { l1Name: string; l2: Category }[] }[] = []
  for (const p of getPartiesByType(t)) {
    groups.push({ party: p, items: stepL2s.value.filter(it => it.l2.name === p.name) })
  }
  return groups
})

// ========== Step: final (银行 + 预置科目 + 预览) ==========
// 预置 L2 列表（非 auto-build 的那些，本金/经营收入/分配与支出等）
const presetL2s = computed(() => {
  const result: { l1Name: string; l2: Category }[] = []
  for (const l1 of cats.value) {
    for (const l2 of l1.children || []) {
      if (l2.preset) result.push({ l1Name: l1.name, l2 })
    }
  }
  return result
})

// 预览数据
const previewBank = computed(() => bankOpening.value ? parseFloat(bankOpening.value) : 0)
const previewParties = computed(() => parties.value.map(p => ({
  name: p.name,
  types: p.types || [(p as any).type].filter(Boolean),
})))
const previewBalances = computed(() => {
  const items: { name: string; yuan: number }[] = []
  const flowNames = new Set(getPartiesByType('flow').map(p => p.name))
  for (const { l1Name, l2 } of [...stepL2s.value, ...presetL2s.value]) {
    // 流转企业费用存在往来单位基本信息，不入科目余额，故不列入预览
    if (flowNames.has(l2.name)) continue
    const input = openingInputs.value[l2.id]
    if (input && parseFloat(input) > 0) {
      items.push({ name: `${l1Name} / ${l2.name}`, yuan: parseFloat(input) })
    }
  }
  return items
})

async function onConfirm() {
  try {
    // 0. 长期投资/再投资：填写的余额双写 → 基本信息 investAmount + 同名科目期初（步骤2统一写入）
    for (const t of (['invest', 'reinvest'] as const)) {
      const l1Name = t === 'invest' ? '长期投资' : '再投资'
      const l1 = findL1(l1Name)
      for (const p of getPartiesByType(t)) {
        const l2 = l1?.children?.find(c => c.name === p.name)
        if (!l2) continue
        const yuan = parseFloat(openingInputs.value[l2.id])
        if (!(yuan > 0)) continue
        await api.put(`/parties/${p.id}`, { investAmountCents: Math.round(yuan * 100) })
      }
    }
    // 1. 更新银行期初
    if (previewBank.value > 0) {
      await api.put('/settings', { bankOpeningBalanceCents: Math.round(previewBank.value * 100) })
    }
    // 2. 流转企业：填写的流转费/管理费 → 存入往来单位基本信息（expectedLandFee/expectedMgmtFee），不入科目余额
    const flowPartyFees = new Map<number, { landFee?: number; mgmtFee?: number }>()
    const skipCat = new Set<number>()
    for (const p of getPartiesByType('flow')) {
      const rec: { landFee?: number; mgmtFee?: number } = {}
      for (const l1Name of ['土地流转费收入', '流转管理费']) {
        const l1 = findL1(l1Name)
        const l2 = l1?.children?.find(c => c.name === p.name)
        if (!l2) continue
        const yuan = parseFloat(openingInputs.value[l2.id])
        if (yuan > 0) {
          if (l1Name === '土地流转费收入') rec.landFee = Math.round(yuan * 100)
          else rec.mgmtFee = Math.round(yuan * 100)
          skipCat.add(l2.id)
        }
      }
      if (rec.landFee !== undefined || rec.mgmtFee !== undefined) flowPartyFees.set(p.id, rec)
    }
    for (const [pid, fee] of flowPartyFees) {
      const payload: any = {}
      if (fee.landFee !== undefined) payload.expectedLandFeeCents = fee.landFee
      if (fee.mgmtFee !== undefined) payload.expectedMgmtFeeCents = fee.mgmtFee
      if (Object.keys(payload).length) await api.put(`/parties/${pid}`, payload)
    }
    // 3. 更新其余填了余额的 L2（投资/再投资同名科目与预置科目）
    for (const [idStr, yuanStr] of Object.entries(openingInputs.value)) {
      if (skipCat.has(Number(idStr))) continue
      const yuan = parseFloat(yuanStr)
      if (yuan > 0) {
        await api.put(`/categories/${idStr}`, { openingBalanceCents: Math.round(yuan * 100) })
      }
    }
    // 4. 标记引导完成（写服务端 org.onboarded，成功后守卫才放行）
    try {
      await auth.markOnboarded()
    } catch (e: any) {
      alert('引导状态保存失败：' + (e?.message || e) + '，请稍后重试')
      return
    }
    router.push('/')
  } catch (e: any) {
    alert('保存失败：' + (e?.message || e) + (e?.response?.message ? '（' + e.response.message + '）' : ''))
  }
}

// ========== 导航 ==========
function nextStep() { skipThisStep.value = false; step.value++ }
function prevStep() { skipThisStep.value = false; step.value-- }
async function skipToHome() {
  // 跳过引导也必须可靠落标记（写服务端），否则后续任意导航都会被守卫打回引导页
  try {
    await auth.markOnboarded()
  } catch (e: any) {
    alert('引导状态保存失败：' + (e?.message || e) + '，请稍后重试')
    return
  }
  router.push('/')
}

// ========== 导入模式 ==========
async function downloadTemplate() {
  try {
    const res = await fetch('/api/onboarding/template', { credentials: 'include' })
    if (!res.ok) { alert('模板下载失败'); return }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = '基础数据导入模板.xlsx'
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: any) { alert('模板下载失败：' + (e.message || e)) }
}
async function onImportFile(e: any) {
  const f = e.target.files?.[0]
  if (!f) return
  importing.value = true
  importResult.value = null
  const fd = new FormData()
  fd.append('file', f)
  try {
    const resp = await api.post<ApiResponse<ImportResult>>('/onboarding/import', fd)
    importResult.value = resp.data
    // 上传后刷新单位，用于预览表
    await loadParties()
  } catch (err: any) {
    alert('导入失败：' + (err?.message || err))
  } finally {
    importing.value = false
    e.target.value = ''
  }
}
function onDrop(e: DragEvent) {
  e.preventDefault()
  dropHover.value = false
  const f = e.dataTransfer?.files?.[0]
  if (f) {
    if (fileInput.value) {
      const dt = new DataTransfer()
      dt.items.add(f)
      fileInput.value.files = dt.files
    }
    // 直接走统一处理
    const fake = { target: { files: [f], value: '' } }
    onImportFile(fake)
  }
}
const dropHover = ref(false)
function continueToFinal() {
  step.value = STEPS.length - 1 // 跳到最后一页：填银行与预置科目期初
  mode.value = 'manual'
}
function switchMode(t: 'manual' | 'import') {
  mode.value = t
  step.value = 0
  skipThisStep.value = false
}
function typeLabel(t: string) { return t === 'flow' ? '流转企业' : t === 'invest' ? '投资公司' : t === 'reinvest' ? '再投资' : t }

// 预览表：上传/导入后按本次成功单位展示（从 parties 推导业务摘要）
const importedParties = computed(() => {
  if (!importResult.value) return []
  const items: { type: string; name: string; contact: string; biz: string }[] = []
  for (const p of parties.value) {
    const tys = (p.types || [(p as any).type].filter(Boolean)) as string[]
    const t = tys[0]
    if (!t) continue
    const label = typeLabel(t)
    const contact = p.contactPhone || '—'
    let biz = ''
    if (t === 'flow') {
      const parts: string[] = []
      if (p.areaMu) parts.push(`${p.areaMu} 亩`)
      if (p.landFeePerMuCents) parts.push(`${(p.landFeePerMuCents / 100).toLocaleString()} 元/亩`)
      biz = parts.join(' · ') || '—'
    } else if (t === 'invest' || t === 'reinvest') {
      const parts: string[] = []
      if (p.investAmountCents) parts.push(`本金 ${(p.investAmountCents / 100).toLocaleString()} 元`)
      if (p.returnRateBps) parts.push(`${(p.returnRateBps / 100).toFixed(2)}%`)
      biz = parts.join(' · ') || '—'
    }
    items.push({ type: t, name: p.name, contact, biz })
  }
  return items
})
const importAllOk = computed(() => !!importResult.value && importResult.value.failed === 0)

// 检校项：基于导入结果推断
const checks = computed(() => {
  const r = importResult.value
  if (!r) return []
  return [
    { label: '单位名称检查', ok: r.failed === 0 || r.errors.every(e => e.message !== '同表内单位名称重复'), auto: false },
    { label: '金额与面积格式', ok: r.failed === 0, auto: false },
    { label: '自动建账', ok: r.created > 0 || r.failed === 0, auto: true, detail: r.created > 0 ? `已建 ${r.created} 个单位` : '无成功单位' },
  ]
})
</script>

<template>
<div class="onboarding-page">
  <div class="shell">
    <!-- ========== 头部引导 ========== -->
    <div class="head">
      <div class="brand">
        <div class="logo">集</div>
        <div>
          <b>集体经济管理平台</b>
          <small>首次使用 · 建账引导</small>
        </div>
      </div>
      <span class="chip" v-if="mode === 'import'">📥 导入模式 · 自动建账</span>
      <span class="chip" v-else>第 {{ step + 1 }} 步 / 共 7 步 · 手工录入</span>
    </div>

    <!-- ========== 模式二选一 ========== -->
    <div class="mode-grid">
      <div class="mode-card import" :class="{ selected: mode === 'import' }" @click="switchMode('import')">
        <span class="pick">✓</span>
        <div class="ic">📥</div>
        <h3>导入 Excel 快速建账</h3>
        <p>下载模板填写好往来单位，一次上传即可自动建好单位与科目，省时省心。</p>
        <span class="tag">推荐 · 快捷</span>
      </div>
      <div class="mode-card manual" :class="{ selected: mode === 'manual' }" @click="switchMode('manual')">
        <span class="pick"></span>
        <div class="ic">✍️</div>
        <h3>手工逐条录入</h3>
        <p>一步步手动添加单位、填期初余额，适合单位数量少的场景。</p>
        <span class="tag">适合少量数据</span>
      </div>
    </div>

    <!-- ========== 步骤条（仅手工模式） ========== -->
    <div v-if="mode === 'manual'" class="steps">
      <div
        v-for="(s, i) in STEPS" :key="i"
        class="step"
        :class="{ active: i === step, done: i < step }"
        @click="mode === 'manual' && (step = i)"
      >
        <span class="n">{{ i < step ? '✓' : i + 1 }}</span>{{ s.label }}
      </div>
    </div>

    <!-- ============ 导入模式内容 ============ -->
    <div v-if="mode === 'import'" class="card">
      <h2>导入 Excel 建账</h2>
      <div class="sub">先从模板下载表格，填好三张表（流转企业 / 投资公司 / 再投资）里的单位后上传。三类均可留空——<b>没有哪一类，就把对应 Sheet 留空</b>，系统按实际填写的类自动建账。</div>

      <div class="tip">✅ 新手上路：模板里已经留好了格式和示例，照着填 <b>单位名称</b> 和 <b>金额</b> 就行，留空的列可以不加。</div>

      <!-- 模板下载 -->
      <div class="dl">
        <div class="f">📄</div>
        <div class="info"><b>基础数据导入模板.xlsx</b><small>含「流转企业 / 投资公司 / 再投资」三张表 + 填写说明</small></div>
        <button class="btn ghost" @click="downloadTemplate">下载模板</button>
      </div>

      <!-- 上传区 -->
      <div
        class="drop" :class="{ hover: dropHover }"
        @click="fileInput && fileInput.click()"
        @dragover.prevent="dropHover = true"
        @dragleave="dropHover = false"
        @drop="onDrop"
      >
        <div class="big">📂</div>
        <p>点击选择，或把 Excel 文件拖到此处</p>
        <small>支持 .xlsx 格式 · 单次一个文件</small>
      </div>
      <input ref="fileInput" type="file" accept=".xlsx" class="hidden" @change="onImportFile" />

      <div v-if="importing" class="tip" style="margin-top:12px;border-left-color:var(--blue)">解析中，请稍候…</div>

      <!-- 导入结果 -->
      <div v-if="importResult" style="margin-top:12px">
        <div class="tip" style="border-left-color:var(--blue)">
          已识别到三类单位；若某类 Sheet 留空，则<b>跳过该类、不为其建账</b>，非必填。
        </div>

        <!-- 预览表 -->
        <table v-if="importedParties.length">
          <thead>
            <tr><th>类目</th><th>单位名称</th><th>联系方式</th><th>业务数据</th></tr>
          </thead>
          <tbody>
            <tr v-for="(it, idx) in importedParties" :key="idx">
              <td><span class="uart" :class="it.type">{{ typeLabel(it.type) }}</span></td>
              <td>{{ it.name }}</td>
              <td class="muted">{{ it.contact }}</td>
              <td class="muted">{{ it.biz }}</td>
            </tr>
          </tbody>
        </table>

        <!-- 检校项 -->
        <div class="check">
          <div v-for="(c, i) in checks" :key="i" class="row">
            <span class="ok" :class="{ auto: c.auto }">{{ c.ok ? '✓' : '✕' }}</span>
            <div><b>{{ c.label }}</b> <span class="muted small">· {{ c.ok ? '通过' : '存在需修正项' }}<template v-if="c.detail"> · {{ c.detail }}</template></span></div>
          </div>
        </div>

        <!-- 汇总 -->
        <div class="sum-line" v-if="!importAllOk">
          成功 <strong>{{ importResult.created }}</strong> 个，失败 <strong>{{ importResult.failed }}</strong> 个。
          <div v-for="(grp, idx) in importSummaryRows" :key="idx" class="muted small">
            {{ typeLabel(grp.type) }}：成功 {{ grp.created }}，失败 {{ grp.failed }}
          </div>
          <div v-for="(e, idx) in importResult.errors" :key="'e'+idx" class="err-line">第 {{ e.row }} 行「{{ e.name }}」：{{ e.message }}</div>
        </div>
      </div>

      <div class="footer">
        <button class="btn ghost" @click="switchMode('manual')">改为手工录入</button>
        <button class="btn primary" @click="continueToFinal">完成并继续 →</button>
      </div>
    </div>

    <!-- ============ 手工模式内容 ============ -->
    <div v-else class="card">
      <!-- add-party 步骤 -->
      <div v-if="currentStep.type === 'add-party'">
        <h2>手工录入 · 第 {{ step + 1 }} 步 {{ currentStep.label }}</h2>
        <div class="sub">
          逐个添加「{{ currentStep.label }}」单位。填错了可点 <b style="color:var(--jade)">🗑 删除</b> 后重输；<b>没有{{ currentStep.label }}可直接「跳过本步」</b>，均非必填。
        </div>

        <div style="margin-bottom:12px">
          <button class="btn primary" @click="openNewParty">＋ 添加{{ currentStep.label }}</button>
        </div>

        <div v-if="getPartiesByType(currentStep.partyType!).length > 0">
          <div v-for="p in getPartiesByType(currentStep.partyType!)" :key="p.id" class="unit-row">
            <div class="l">
              <span class="uart" :class="currentStep.partyType">{{ currentStep.label }}</span>
              {{ p.name }}
            </div>
            <button class="btn ghost small-del" @click="deletePartyByType(p, currentStep.partyType)">🗑 删除</button>
          </div>
        </div>
        <div v-else class="empty">暂无{{ currentStep.label }}，点击上方按钮添加</div>

        <div style="text-align:center;margin-top:14px">
          <a href="javascript:void(0)" class="skip" @click="nextStep">本类暂无单位，跳过此步 →</a>
        </div>

        <div class="footer">
          <button v-if="step > 0" class="btn ghost" @click="prevStep">← 上一步</button>
          <button v-else class="btn ghost" @click="skipToHome">跳过引导</button>
          <button class="btn primary" :disabled="!canNext" @click="nextStep">下一步 →</button>
        </div>
      </div>

      <!-- fill-balance 步骤 -->
      <div v-else-if="currentStep.type === 'fill-balance'">
        <h2>第 {{ step + 1 }} 步 · {{ currentStep.label }}</h2>
        <div class="sub">分别为每个<strong>{{ STEPS[step - 1]?.label }}</strong>录入其同名科目期初余额（建账时点的存量）。不填默认为 0。</div>

        <div v-if="unitBalances.length === 0" class="empty">暂无可填余额的单位（可能上一步没有录入单位）</div>
        <div v-for="g in unitBalances" :key="g.party.id" class="unit-block">
          <div class="unit-title">{{ g.party.name }}</div>
          <div v-if="g.items.length === 0" class="empty" style="padding:8px">（无已创建同名科目）</div>
          <div v-for="item in g.items" :key="item.l2.id" class="balance-row">
            <div class="balance-name">{{ item.l1Name }}</div>
            <input v-model="openingInputs[item.l2.id]" class="yuan-input" type="text" inputmode="decimal" placeholder="0" />
            <span class="yuan-tail">元</span>
          </div>
        </div>

        <div class="footer">
          <button class="btn ghost" @click="prevStep">← 上一步</button>
          <button class="btn primary" @click="nextStep">下一步 →</button>
        </div>
      </div>

      <!-- final 步骤 -->
      <div v-else>
        <h2>最后一步 · 其他设置</h2>
        <div class="sub">最后一步：设置银行存款期初 + 所有预置科目余额。</div>

        <div class="bank-block">
          <div class="bank-title">🏦 银行存款期初余额</div>
          <input v-model="bankOpening" class="yuan-input big" type="text" inputmode="decimal" placeholder="0.00" />
          <span class="yuan-tail">元</span>
        </div>

        <div class="presets-title">📊 预置科目期初余额</div>
        <template v-for="group in groupByL1(presetL2s)" :key="group.l1Name">
          <div class="group-title">{{ group.l1Name }}</div>
          <div v-for="item in group.items" :key="item.l2.id" class="balance-row">
            <div class="balance-name">{{ item.l2.name }}</div>
            <input v-model="openingInputs[item.l2.id]" class="yuan-input" type="text" inputmode="decimal" placeholder="0" />
            <span class="yuan-tail">元</span>
          </div>
        </template>

        <!-- 预览 -->
        <div class="preview-block">
          <div class="preview-title">📋 预览确认</div>
          <div class="preview-row"><span>银行存款期初</span><span class="amt">¥ {{ previewBank.toLocaleString() }}</span></div>
          <div class="preview-row"><span>往来单位</span><span>{{ previewParties.length }} 个</span></div>
          <div class="preview-sub">非零科目余额（{{ previewBalances.length }} 项）：</div>
          <div v-if="previewBalances.length === 0" class="empty-sub">（全部为 0）</div>
          <div v-for="b in previewBalances" :key="b.name" class="preview-row">
            <span>{{ b.name }}</span>
            <span class="amt">¥ {{ b.yuan.toLocaleString() }}</span>
          </div>
        </div>

        <div class="footer">
          <button class="btn ghost" @click="prevStep">← 上一步</button>
          <button class="btn primary" @click="onConfirm">✅ 确认并完成</button>
        </div>
      </div>
    </div>

    <!-- ========== 新增单位的弹窗 ========== -->
    <div v-if="showPartyForm" class="ob-modal-mask" @click.self="showPartyForm = false">
      <div class="ob-modal">
        <div class="ob-modal-title">新增{{ currentStep.partyType === 'flow' ? '流转企业' : currentStep.partyType === 'invest' ? '投资公司' : '再投资' }}</div>
        <div class="ob-modal-field">
          <label>名称</label>
          <input v-model="partyForm.name" class="ob-form-input" placeholder="公司/合作社/农户全称" />
        </div>
        <div class="ob-modal-hint">保存后自动在 <strong>{{ TYPE_TO_L1S[currentStep.partyType!].join('、') }}</strong> 下创建同名二级科目</div>
        <div class="ob-modal-actions">
          <button class="btn ghost" @click="showPartyForm = false">取消</button>
          <button class="btn primary" @click="saveParty">保存</button>
        </div>
      </div>
    </div>
  </div>
</div>
</template>

<script lang="ts">
// 顶层辅助：按 L1 分组
function groupByL1<T extends { l1Name: string }>(items: T[]): { l1Name: string; items: T[] }[] {
  const map: Record<string, T[]> = {}
  for (const it of items) {
    if (!map[it.l1Name]) map[it.l1Name] = []
    map[it.l1Name].push(it)
  }
  return Object.entries(map).map(([l1Name, items]) => ({ l1Name, items }))
}
export default { methods: { groupByL1 } }
</script>

<style scoped>
/* ========== 引导页局部变量（对齐 demo 配色，不污染全局主题） ========== */
.onboarding-page {
  --jade: #2b5876;
  --jade-deep: #16384d;
  --jade-soft: #eaf1f6;
  --blue: #5a9cb8;
  --blue-soft: #e8f1f6;
  --violet: #9a7fb0;
  --violet-soft: #f1edfc;
  --radius: 16px;

  min-height: 100vh;
  background: linear-gradient(180deg, #fdfaf3 0%, var(--paper, #faf8f4) 100%);
  color: var(--ink, #22312b);
  padding: 32px 16px 64px;
}
.shell { max-width: 840px; margin: 0 auto; }

/* ===== 头部 ===== */
.head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px; }
.brand { display: flex; align-items: center; gap: 12px; }
.brand .logo {
  width: 42px; height: 42px; border-radius: 12px;
  background: linear-gradient(135deg, var(--jade), var(--indigo)); color: #fff;
  display: flex; align-items: center; justify-content: center;
  font-size: 20px; font-weight: 700;
}
.brand b { font-size: 18px; letter-spacing: .5px; }
.brand small { display: block; color: var(--ink-muted); font-weight: 400; font-size: 12px; margin-top: 2px; }
.head .chip { font-size: 12px; color: var(--jade); background: var(--jade-soft); border-radius: 20px; padding: 6px 14px; }

/* ===== 模式二选一 ===== */
.mode-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 22px; }
.mode-card {
  background: #fff; border: 1.5px solid var(--line, #efe9dd); border-radius: var(--radius);
  padding: 20px; cursor: pointer; position: relative; transition: .18s;
}
.mode-card:hover { transform: translateY(-2px); box-shadow: 0 8px 24px rgba(20,61,48,.08); }
.mode-card.selected { border-color: var(--jade); box-shadow: 0 0 0 3px var(--jade-soft); }
.mode-card .ic {
  width: 44px; height: 44px; border-radius: 12px;
  display: flex; align-items: center; justify-content: center; font-size: 22px; margin-bottom: 12px;
}
.mode-card.import .ic { background: var(--blue-soft); }
.mode-card.manual .ic { background: var(--violet-soft); }
.mode-card .pick {
  position: absolute; top: 14px; right: 14px; width: 22px; height: 22px; border-radius: 50%;
  border: 2px solid #ddd; display: flex; align-items: center; justify-content: center;
  font-size: 13px; color: #fff;
}
.mode-card.selected .pick { background: var(--jade); border-color: var(--jade); }
.mode-card h3 { font-size: 16px; margin-bottom: 6px; }
.mode-card p { font-size: 13px; color: var(--ink-muted); line-height: 1.6; }
.mode-card .tag { display: inline-block; font-size: 11px; margin-top: 10px; border-radius: 10px; padding: 2px 9px; }
.mode-card.import .tag { background: var(--blue-soft); color: var(--blue); }
.mode-card.manual .tag { background: var(--violet-soft); color: var(--violet); }

/* ===== 步骤条 ===== */
.steps {
  display: flex; align-items: center; gap: 4px; background: #fff;
  border: 1px solid var(--line, #efe9dd); border-radius: var(--radius);
  padding: 12px 16px; margin-bottom: 22px; flex-wrap: wrap;
}
.step {
  display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--ink-muted);
  cursor: pointer; padding: 4px 6px; border-radius: 8px;
}
.step .n {
  width: 20px; height: 20px; border-radius: 50%; background: #f0ede7;
  display: flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 600; color: var(--ink-muted);
}
.step.done { color: var(--jade); }
.step.done .n { background: var(--jade-soft); color: var(--jade); }
.step.active { color: var(--jade-deep); font-weight: 600; }
.step.active .n { background: var(--jade); color: #fff; }

/* ===== 内容卡 ===== */
.card { background: #fff; border: 1px solid var(--line, #efe9dd); border-radius: var(--radius); padding: 24px; }
.card h2 { font-size: 18px; margin-bottom: 6px; }
.card .sub { font-size: 13px; color: var(--ink-muted); margin-bottom: 18px; line-height: 1.7; }

/* ===== 新手提示条 ===== */
.tip {
  background: var(--jade-soft); border-left: 4px solid var(--jade); border-radius: 10px;
  padding: 12px 14px; font-size: 13px; color: var(--ink); line-height: 1.7; margin-bottom: 20px;
}
.tip b { color: var(--jade-deep); }

/* ===== 拖拽上传 ===== */
.drop {
  border: 2px dashed #cdd6cf; border-radius: 14px; background: var(--paper, #faf8f4);
  padding: 34px 20px; text-align: center; cursor: pointer; transition: .18s; margin-bottom: 14px;
}
.drop.hover { border-color: var(--jade); background: var(--jade-soft); }
.drop .big { font-size: 36px; margin-bottom: 8px; }
.drop p { font-size: 14px; font-weight: 600; }
.drop small { color: var(--ink-muted); font-size: 12px; margin-top: 6px; display: inline-block; }

/* ===== 模板下载 ===== */
.dl {
  display: flex; align-items: center; gap: 12px; background: var(--paper, #faf8f4);
  border: 1px solid var(--line, #efe9dd); border-radius: 12px; padding: 12px 14px; margin-bottom: 20px;
}
.dl .f { font-size: 26px; }
.dl .info { flex: 1; }
.dl .info b { font-size: 13px; display: block; }
.dl .info small { color: var(--ink-muted); font-size: 12px; }

/* ===== 按钮 ===== */
.btn { border: none; cursor: pointer; font-size: 13px; border-radius: 20px; padding: 8px 20px; transition: .15s; white-space: nowrap; }
.btn.primary { background: var(--jade); color: #fff; }
.btn.primary:hover { background: var(--jade-deep); }
.btn.primary:disabled { background: #ccc; cursor: not-allowed; }
.btn.ghost { background: #fff; border: 1px solid #ccc; color: var(--ink-muted); }
.btn.ghost:hover { border-color: var(--jade); color: var(--jade); }
.small-del { font-size: 12px; padding: 4px 10px; }

/* ===== 预览表 ===== */
table { width: 100%; border-collapse: collapse; font-size: 13px; margin-bottom: 16px; }
th { font-size: 12px; color: var(--ink-muted); background: var(--paper, #faf8f4); padding: 10px; text-align: left; }
td { padding: 10px; border-top: 1px solid var(--line, #efe9dd); }
.uart { display: inline-block; font-size: 11px; border-radius: 10px; padding: 2px 9px; }
.uart.flow { background: var(--jade-soft); color: var(--jade); }
.uart.invest { background: var(--blue-soft); color: var(--blue); }
.uart.reinvest { background: var(--violet-soft); color: var(--violet); }

/* ===== 检校项 ===== */
.check { margin-top: 20px; }
.check .row {
  display: flex; align-items: center; gap: 10px; background: var(--paper, #faf8f4);
  border-radius: 10px; padding: 12px; margin-bottom: 8px; font-size: 13px;
}
.check .ok {
  width: 20px; height: 20px; border-radius: 50%; background: var(--jade); color: #fff;
  display: flex; align-items: center; justify-content: center; font-size: 12px;
}
.check .ok.auto { background: var(--blue); }

/* ===== 导入汇总/错误 ===== */
.sum-line { font-size: 13px; padding: 12px; background: var(--paper, #faf8f4); border-radius: 10px; margin-top: 12px; }
.sum-line strong { color: var(--jade); }
.err-line { color: var(--danger, #a33a2d); background: #fdf3f1; padding: 6px 10px; border-radius: 6px; margin: 4px 0; font-size: 12px; }

/* ===== footer ===== */
.footer { display: flex; justify-content: flex-end; gap: 12px; margin-top: 20px; padding-top: 18px; border-top: 1px solid var(--line, #efe9dd); }
.footer .btn:first-child { margin-right: auto; }

/* ===== 手工模式 ===== */
.unit-row {
  display: flex; align-items: center; justify-content: space-between;
  background: var(--paper, #faf8f4); border-radius: 10px; padding: 12px 14px; margin-bottom: 8px;
}
.unit-row .l { display: flex; align-items: center; gap: 10px; }
.empty { padding: 20px; text-align: center; color: var(--ink-muted); font-size: 13px; background: var(--paper, #faf8f4); border-radius: 10px; }
.skip { font-size: 13px; color: var(--jade); text-decoration: none; font-weight: 600; cursor: pointer; }
.skip:hover { color: var(--jade-deep); }

/* ===== fill-balance / final ===== */
.unit-block { background: var(--paper, #faf8f4); border: 1px solid #f0eadf; border-radius: 10px; padding: 8px 12px; margin-bottom: 12px; }
.unit-title { font-size: 13px; font-weight: 600; color: var(--ink, #22312b); padding: 2px 0 4px; border-bottom: 1px dashed #eee; margin-bottom: 4px; }
.balance-row { display: flex; align-items: center; gap: 12px; padding: 6px 0; }
.balance-name { flex: 1; font-size: 13px; color: var(--ink, #22312b); }
.yuan-input { width: 120px; padding: 6px 10px; border: 1px solid #ddd; border-radius: 6px; font-size: 13px; text-align: right; }
.yuan-input.big { width: 160px; font-size: 16px; padding: 10px 14px; }
.yuan-tail { color: var(--ink-muted); font-size: 12px; }
.bank-block { background: linear-gradient(135deg, var(--jade-bg, #f2f7fa) 0%, var(--terracotta-bg, #fbf6ea) 100%); border-radius: 10px; padding: 16px; display: flex; align-items: center; gap: 12px; margin-bottom: 20px; }
.bank-title { font-weight: 600; flex: 1; }
.group-title { font-size: 12px; font-weight: 600; color: var(--terracotta, #b86b3d); padding: 8px 0 4px; border-bottom: 1px dashed #eee; }
.presets-title { font-weight: 600; color: var(--terracotta, #b86b3d); margin-bottom: 8px; }

/* ===== 预览确认 ===== */
.preview-block { margin-top: 24px; padding: 16px; background: var(--paper, #faf8f4); border-radius: 10px; }
.preview-title { font-weight: 600; margin-bottom: 8px; }
.preview-row { display: flex; justify-content: space-between; padding: 4px 0; font-size: 13px; }
.preview-row .amt { color: var(--terracotta, #b86b3d); font-weight: 600; }
.preview-sub { color: var(--ink-muted); font-size: 12px; margin-top: 8px; margin-bottom: 4px; }
.empty-sub { padding: 8px 0; color: var(--ink-muted); font-size: 12px; }

/* ===== 弹窗 ===== */
.ob-modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.4); display: flex; align-items: center; justify-content: center; z-index: 100; }
.ob-modal { background: #fff; border-radius: var(--radius); padding: 24px; width: 90%; max-width: 360px; }
.ob-modal-title { font-size: 16px; font-weight: 600; margin-bottom: 16px; }
.ob-modal-field { margin-bottom: 12px; }
.ob-modal-field label { display: block; font-size: 12px; color: var(--ink-muted); margin-bottom: 4px; }
.ob-form-input { width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px; box-sizing: border-box; }
.ob-modal-hint { font-size: 11px; color: var(--ink-muted); background: var(--paper, #faf8f4); padding: 8px; border-radius: 6px; margin-bottom: 16px; line-height: 1.6; }
.ob-modal-actions { display: flex; justify-content: flex-end; gap: 8px; }

.hidden { display: none; }
.muted { color: var(--ink-muted); }
.small { font-size: 12px; }

@media (max-width: 640px) { .mode-grid { grid-template-columns: 1fr; } }
</style>