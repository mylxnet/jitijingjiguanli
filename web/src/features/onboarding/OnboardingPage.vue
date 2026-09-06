<!--
  引导页（Onboarding）— 新版按类型分步
  7 步：①流转企业 → ②流转企业余额 → ③投资公司 → ④投资公司余额 → ⑤再投资 → ⑥再投资余额 → ⑦其他设置+预览
  完成后写 localStorage: jt_onboarding_done_{orgId}
-->
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import type { ApiResponse, Category, Party } from '../../types/api'

const router = useRouter()

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

// 本地表单
const editingParty = ref<Party | null>(null)
const showPartyForm = ref(false)
const partyForm = ref({ name: '' })

// 本地期初输入：key = 二级科目 id（number），value = 元字符串
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
function findOrCreateL2(l1Name: string, l2Name: string): Category | null {
  const l1 = findL1(l1Name)
  if (!l1) return null
  const exist = l1.children?.find(c => c.name === l2Name)
  if (exist) return exist
  // 通过 HTTP POST 新建
  api.post('/categories', { name: l2Name, level: 2, parentId: l1.id, kind: 'equity' })
    .then(() => loadCats())
  return null
}

// ========== Step: add-party ==========
function openNewParty() {
  editingParty.value = null
  partyForm.value = { name: '' }
  showPartyForm.value = true
}
function editParty(p: Party) {
  editingParty.value = p
  partyForm.value = { name: p.name }
  showPartyForm.value = true
}
async function saveParty() {
  const name = partyForm.value.name.trim()
  if (!name) return
  const t = currentStep.value.partyType!
  try {
    if (editingParty.value) {
      await api.put(`/parties/${editingParty.value.id}`, { name })
    } else {
      await api.post('/parties', { name, types: [t] })
      // 自动在所有关联 L1 下建同名 L2
      for (const l1Name of TYPE_TO_L1S[t]) {
        findOrCreateL2(l1Name, name)
      }
    }
    showPartyForm.value = false
    await loadParties()
    await loadCats()
  } catch (e: any) {
    alert('保存失败：' + (e.message || e))
  }
}
async function deleteParty(p: Party) {
  if (!confirm('确认删除？')) return
  try {
    await api.del(`/parties/${p.id}`)
    await loadParties()
  } catch (e: any) { alert('删除失败：' + (e.message || e)) }
}

// ========== Step: fill-balance ==========
// 本步涉及的 L2 列表
const stepL2s = computed(() => {
  const s = currentStep.value
  if (s.type !== 'fill-balance') return []
  const result: { l1Name: string; l2: Category }[] = []
  for (const l1Name of (s.l1Names || [])) {
    const l1 = findL1(l1Name)
    if (!l1) continue
    for (const l2 of l1.children || []) {
      result.push({ l1Name, l2 })
    }
  }
  return result
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
  for (const { l1Name, l2 } of [...stepL2s.value, ...presetL2s.value]) {
    const input = openingInputs.value[l2.id]
    if (input && parseFloat(input) > 0) {
      items.push({ name: `${l1Name} / ${l2.name}`, yuan: parseFloat(input) })
    }
  }
  return items
})

async function onConfirm() {
  // 1. 更新银行期初
  if (previewBank.value > 0) {
    await api.put('/settings', { bankBalanceCents: Math.round(previewBank.value * 100) })
  }
  // 2. 更新所有填了余额的 L2
  for (const [idStr, yuanStr] of Object.entries(openingInputs.value)) {
    const yuan = parseFloat(yuanStr)
    if (yuan > 0) {
      await api.put(`/categories/${idStr}`, { openingBalanceCents: Math.round(yuan * 100) })
    }
  }
  // 3. 写 localStorage 标记完成
  try {
    const me = await api.get<ApiResponse<any>>('/auth/me')
    const orgId = (me.data?.orgId ?? 'default')
    localStorage.setItem(`jt_onboarding_done_${orgId}`, '1')
  } catch { /* ignore */ }
  router.push('/')
}

// ========== 导航 ==========
function nextStep() { skipThisStep.value = false; step.value++ }
function prevStep() { skipThisStep.value = false; step.value-- }
async function skipToHome() {
  try {
    const me = await api.get<ApiResponse<any>>('/auth/me')
    const orgId = (me.data?.orgId ?? 'default')
    localStorage.setItem(`jt_onboarding_done_${orgId}`, '1')
  } catch { /* ignore */ }
  router.push('/')
}
</script>

<template>
<div class="onboarding-page">
  <div class="ob-container">
    <!-- 顶部步骤条 -->
    <div class="ob-steps">
      <div
        v-for="(s, i) in STEPS" :key="i"
        class="ob-step"
        :class="{ active: i === step, done: i < step, type: s.type }"
      >
        <div class="ob-step-num">{{ i + 1 }}</div>
        <div class="ob-step-label">{{ s.label }}</div>
      </div>
    </div>

    <!-- 步骤主体 -->
    <div class="ob-body">

      <!-- ========== add-party 步骤 ========== -->
      <div v-if="currentStep.type === 'add-party'" class="ob-step-panel">
        <p class="ob-hint">
          录入<strong>{{ currentStep.label }}</strong>往来单位。<br />
          每个单位保存后，自动在 <strong>{{ TYPE_TO_L1S[currentStep.partyType!].join('、') }}</strong> 下创建同名二级科目。
        </p>

        <div class="ob-actions">
          <button class="btn-primary" @click="openNewParty">+ 新增{{ currentStep.label }}</button>
        </div>

        <div class="ob-party-list">
          <div v-if="getPartiesByType(currentStep.partyType!).length === 0" class="ob-empty">
            暂无{{ currentStep.label }}，点击上方按钮添加
          </div>
          <div v-for="p in getPartiesByType(currentStep.partyType!)" :key="p.id" class="ob-party-row">
            <div class="ob-pname">{{ p.name }}</div>
            <div class="ob-ptags">
              <span v-for="t in (p.types || [(p as any).type].filter(Boolean))" :key="t" class="ob-type-tag">{{ t === 'flow' ? '流转企业' : t === 'invest' ? '投资公司' : '再投资' }}</span>
            </div>
            <div class="ob-pactions">
              <button class="btn-mini" @click="editParty(p)">编辑</button>
              <button class="btn-mini danger" @click="deleteParty(p)">删除</button>
            </div>
          </div>
        </div>

        <div class="ob-nav">
          <button v-if="step > 0" class="btn-ghost" @click="prevStep">← 上一步</button>
          <button v-else class="btn-ghost" @click="skipToHome">跳过引导</button>
          <button class="btn-primary" :disabled="!canNext" @click="nextStep">
            下一步 →
          </button>
        </div>
      </div>

      <!-- ========== fill-balance 步骤 ========== -->
      <div v-else-if="currentStep.type === 'fill-balance'" class="ob-step-panel">
        <p class="ob-hint">
          为这些自动创建的二级科目设置期初余额（建账时点的存量）。不填默认为 0。
        </p>

        <div class="ob-balance-block">
          <template v-for="group in groupByL1(stepL2s)" :key="group.l1Name">
            <div class="ob-group-title">{{ group.l1Name }}</div>
            <div v-for="item in group.items" :key="item.l2.id" class="ob-balance-row">
              <div class="ob-balance-name">{{ item.l2.name }}</div>
              <input
                v-model="openingInputs[item.l2.id]"
                class="ob-yuan-input"
                type="text" inputmode="decimal"
                placeholder="0"
              />
              <span class="ob-yuan-tail">元</span>
            </div>
          </template>
          <div v-if="stepL2s.length === 0" class="ob-empty">
            暂无可填余额的二级科目（可能上一步没有录入单位）
          </div>
        </div>

        <div class="ob-nav">
          <button class="btn-ghost" @click="prevStep">← 上一步</button>
          <button class="btn-primary" @click="nextStep">下一步 →</button>
        </div>
      </div>

      <!-- ========== final 步骤 ========== -->
      <div v-else class="ob-step-panel">
        <p class="ob-hint">最后一步：设置银行存款期初 + 所有预置科目余额。</p>

        <!-- 银行存款 -->
        <div class="ob-bank-block">
          <div class="ob-bank-title">🏦 银行存款期初余额</div>
          <input v-model="bankOpening" class="ob-yuan-input big" type="text" inputmode="decimal" placeholder="0.00" />
          <span class="ob-yuan-tail">元</span>
        </div>

        <!-- 预置科目 -->
        <div class="ob-presets-block">
          <div class="ob-presets-title">📊 预置科目期初余额</div>
          <template v-for="group in groupByL1(presetL2s)" :key="group.l1Name">
            <div class="ob-group-title">{{ group.l1Name }}</div>
            <div v-for="item in group.items" :key="item.l2.id" class="ob-balance-row">
              <div class="ob-balance-name">{{ item.l2.name }}</div>
              <input v-model="openingInputs[item.l2.id]" class="ob-yuan-input" type="text" inputmode="decimal" placeholder="0" />
              <span class="ob-yuan-tail">元</span>
            </div>
          </template>
        </div>

        <!-- 预览 -->
        <div class="ob-preview-block">
          <div class="ob-preview-title">📋 预览确认</div>
          <div class="ob-preview-row"><span>银行存款期初</span><span class="amt">¥ {{ previewBank.toLocaleString() }}</span></div>
          <div class="ob-preview-row"><span>往来单位</span><span>{{ previewParties.length }} 个</span></div>
          <div class="ob-preview-sub">非零科目余额（{{ previewBalances.length }} 项）：</div>
          <div v-if="previewBalances.length === 0" class="ob-empty-sub">（全部为 0）</div>
          <div v-for="b in previewBalances" :key="b.name" class="ob-preview-row">
            <span>{{ b.name }}</span>
            <span class="amt">¥ {{ b.yuan.toLocaleString() }}</span>
          </div>
        </div>

        <div class="ob-nav">
          <button class="btn-ghost" @click="prevStep">← 上一步</button>
          <button class="btn-primary" @click="onConfirm">✅ 确认并完成</button>
        </div>
      </div>
    </div>
  </div>

  <!-- ========== 新增/编辑往来单位弹窗（简化：只有名称，类型由当前步骤决定） ========== -->
  <div v-if="showPartyForm" class="ob-modal-mask" @click.self="showPartyForm = false">
    <div class="ob-modal">
      <div class="ob-modal-title">{{ editingParty ? '编辑' : '新增' }}{{ currentStep.partyType === 'flow' ? '流转企业' : currentStep.partyType === 'invest' ? '投资公司' : '再投资' }}</div>
      <div class="ob-modal-field">
        <label>名称</label>
        <input v-model="partyForm.name" class="ob-form-input" placeholder="公司/合作社/农户全称" />
      </div>
      <div class="ob-modal-hint">
        保存后自动在 <strong>{{ TYPE_TO_L1S[currentStep.partyType!].join('、') }}</strong> 下创建同名二级科目
      </div>
      <div class="ob-modal-actions">
        <button class="btn-ghost" @click="showPartyForm = false">取消</button>
        <button class="btn-primary" @click="saveParty">保存</button>
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
.onboarding-page { min-height: 100vh; background: var(--paper, #faf8f4); padding: 24px 16px 80px; }
.ob-container { max-width: 720px; margin: 0 auto; }

/* steps */
.ob-steps { display: flex; gap: 4px; margin-bottom: 28px; flex-wrap: wrap; }
.ob-step { display: flex; align-items: center; gap: 6px; padding: 6px 10px; border-radius: 20px; font-size: 12px; color: var(--ink-muted); background: #f4f2ef; }
.ob-step.active { background: var(--jade); color: #fff; }
.ob-step.done { color: var(--jade); }
.ob-step-num { width: 20px; height: 20px; border-radius: 50%; background: rgba(0,0,0,.08); display: flex; align-items: center; justify-content: center; font-weight: 600; font-size: 11px; }
.ob-step.active .ob-step-num { background: rgba(255,255,255,.25); }
.ob-step.done .ob-step-num { background: #d8ece2; }

/* body */
.ob-body { background: #fff; border-radius: 12px; padding: 24px; box-shadow: 0 2px 12px rgba(0,0,0,.06); }
.ob-hint { color: var(--ink-muted); font-size: 13px; line-height: 1.7; margin-bottom: 16px; }
.ob-hint strong { color: var(--jade); }

/* actions */
.ob-actions { margin-bottom: 16px; }
.btn-primary { background: var(--jade); color: #fff; border: none; padding: 8px 20px; border-radius: 20px; cursor: pointer; font-size: 13px; }
.btn-primary:disabled { background: #ccc; cursor: not-allowed; }
.btn-ghost { background: transparent; border: 1px solid #ddd; padding: 8px 20px; border-radius: 20px; cursor: pointer; font-size: 13px; color: var(--ink-muted); }

/* party list */
.ob-party-list { display: flex; flex-direction: column; gap: 8px; }
.ob-party-row { display: flex; align-items: center; gap: 12px; padding: 10px 12px; background: #faf8f4; border-radius: 8px; }
.ob-pname { flex: 1; font-weight: 500; }
.ob-ptags { display: flex; gap: 4px; }
.ob-type-tag { padding: 2px 8px; background: #eef6f1; color: var(--jade); border-radius: 10px; font-size: 11px; }
.ob-pactions { display: flex; gap: 6px; }
.btn-mini { padding: 4px 12px; border-radius: 12px; font-size: 11px; border: 1px solid #ddd; background: #fff; cursor: pointer; }
.btn-mini.danger { color: #c0392b; border-color: #f5c6cb; }
.ob-empty { padding: 24px; text-align: center; color: var(--ink-muted); font-size: 13px; background: #faf8f4; border-radius: 8px; }
.ob-empty-sub { padding: 8px 0; color: var(--ink-muted); font-size: 12px; }

/* balance */
.ob-balance-block { display: flex; flex-direction: column; gap: 12px; }
.ob-group-title { font-size: 12px; font-weight: 600; color: var(--terracotta); padding: 8px 0 4px; border-bottom: 1px dashed #eee; }
.ob-balance-row { display: flex; align-items: center; gap: 12px; padding: 6px 0; }
.ob-balance-name { flex: 1; font-size: 13px; color: var(--ink); }
.ob-yuan-input { width: 120px; padding: 6px 10px; border: 1px solid #ddd; border-radius: 6px; font-size: 13px; text-align: right; }
.ob-yuan-input.big { width: 160px; font-size: 16px; padding: 10px 14px; }
.ob-yuan-tail { color: var(--ink-muted); font-size: 12px; }

/* bank + preset */
.ob-bank-block { background: linear-gradient(135deg, #eef6f1 0%, #fdf4ec 100%); border-radius: 10px; padding: 16px; display: flex; align-items: center; gap: 12px; margin-bottom: 20px; }
.ob-bank-title { font-weight: 600; flex: 1; }
.ob-presets-block { margin-top: 16px; }
.ob-presets-title { font-weight: 600; color: var(--terracotta); margin-bottom: 8px; }

/* preview */
.ob-preview-block { margin-top: 24px; padding: 16px; background: #faf8f4; border-radius: 10px; }
.ob-preview-title { font-weight: 600; margin-bottom: 8px; }
.ob-preview-row { display: flex; justify-content: space-between; padding: 4px 0; font-size: 13px; }
.ob-preview-row .amt { color: var(--terracotta); font-weight: 600; }
.ob-preview-sub { color: var(--ink-muted); font-size: 12px; margin-top: 8px; margin-bottom: 4px; }

/* nav */
.ob-nav { display: flex; justify-content: space-between; margin-top: 24px; gap: 12px; }

/* modal */
.ob-modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,.4); display: flex; align-items: center; justify-content: center; z-index: 100; }
.ob-modal { background: #fff; border-radius: 12px; padding: 24px; width: 90%; max-width: 360px; }
.ob-modal-title { font-size: 16px; font-weight: 600; margin-bottom: 16px; }
.ob-modal-field { margin-bottom: 12px; }
.ob-modal-field label { display: block; font-size: 12px; color: var(--ink-muted); margin-bottom: 4px; }
.ob-form-input { width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px; box-sizing: border-box; }
.ob-modal-hint { font-size: 11px; color: var(--ink-muted); background: #faf8f4; padding: 8px; border-radius: 6px; margin-bottom: 16px; line-height: 1.6; }
.ob-modal-hint strong { color: var(--jade); }
.ob-modal-actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
