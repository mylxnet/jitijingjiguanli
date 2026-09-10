import { computed, ref } from 'vue'
import { api } from '../../lib/http'
import type { ApiResponse, Party } from '../../types/api'

// 单位类型展示名
const TYPE_LABEL: Record<string, string> = {
  flow: '流转企业',
  invest: '投资公司',
  reinvest: '再投资',
  other: '其他',
}

// 取单位的类型列表（兼容旧的单值 type）
function partyTypes(p: Party): string[] {
  if (p.types && p.types.length) return p.types
  return p.type ? [p.type] : []
}

// 空值判定：<= 0、null、undefined、NaN 都算"没填"（防止脏数据被当成已填）
function blank(v?: number | null): boolean {
  return !(Number(v) > 0)
}

// 缺口判定口径（已定稿，"任一为 0 即缺"；同一单位缺多项也只计 1）：
//   flow           → 流转亩数 / 总流转费 / 总管理费
//   invest/reinvest→ 投资本金 / 年收益
//   other          → 不参与判定
export function missingFields(p: Party): string[] {
  const miss: string[] = []
  const types = partyTypes(p)
  if (types.includes('flow')) {
    if (blank(p.landMu) && blank(p.areaMu)) miss.push('流转亩数')
    if (blank(p.expectedLandFeeCents)) miss.push('总流转费')
    if (blank(p.expectedMgmtFeeCents)) miss.push('总管理费')
  }
  if (types.includes('invest') || types.includes('reinvest')) {
    if (blank(p.investAmountCents)) miss.push('投资本金')
    if (blank(p.expectedReturnCents)) miss.push('年收益')
  }
  return miss
}

export interface PartyIssue {
  id: number
  name: string
  typeLabel: string
  missing: string[]
}

// 模块级共享状态：侧边栏角标与缺失清单弹窗共用同一份数据
const parties = ref<Party[]>([])
const loading = ref(false)
let inflight: Promise<void> | null = null

const issues = computed<PartyIssue[]>(() =>
  parties.value.reduce<PartyIssue[]>((acc, p) => {
    const miss = missingFields(p)
    if (miss.length === 0) return acc
    const types = partyTypes(p)
    acc.push({ id: p.id, name: p.name, typeLabel: TYPE_LABEL[types[0]] || '其他', missing: miss })
    return acc
  }, [])
)

const issueCount = computed(() => issues.value.length)

// 拉取单位并重算缺口。
// - 并发调用复用同一请求，避免路由连续切换时重复拉取；
// - 失败时保留上一次结果，避免角标闪烁。
function refresh(): Promise<void> {
  if (inflight) return inflight
  loading.value = true
  inflight = api
    .get<ApiResponse<Party[]> | Party[]>('/parties')
    .then((res) => {
      const list = Array.isArray(res)
        ? res
        : (Array.isArray((res as ApiResponse<Party[]>)?.data) ? (res as ApiResponse<Party[]>).data : [])
      parties.value = list
    })
    .catch(() => {
      // ignore：保留旧数据
    })
    .finally(() => {
      loading.value = false
      inflight = null
    })
  return inflight
}

export function useContactIssues() {
  return { issues, issueCount, loading, refresh }
}
