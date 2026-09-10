import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '../../lib/http'
import type { ApiResponse } from '../../types/api'

// 本机记住的组织名（登录页大字展示）
export const ORG_NAME_KEY = 'jt_org_name'

// 引导完成标记：读/写必须走同一套 key，避免出现 `_default` 之类不匹配的键
export function onboardingKey(id: string | number) {
  return `jt_onboarding_done_${id}`
}

export const useAuthStore = defineStore('auth', () => {
  const isLoggedIn = ref(false)
  const loading = ref(false)
  const error = ref('')
  // 当前登录用户所属组织 id（用于引导完成标记等按组织区分的场景）
  const orgId = ref<string | number | null>(null)
  // 已确认「有业务数据并补写标记」的组织，避免每次导航重复请求
  const onboardChecked = ref<string | number | null>(null)

  // 拉取当前组织信息（登录/注册/首屏后统一调用，保证 orgId 始终可用）
  async function refreshOrg() {
    try {
      const me = await fetch('/api/me', { credentials: 'include' }).then(r => r.json())
      orgId.value = me?.data?.orgID ?? me?.data?.orgId ?? null
    } catch {
      orgId.value = null
    }
  }

  // 检查是否已登录（通过 Cookie session）
  // 使用原生 fetch 绕过 http.ts 的 onUnauthorized 回调，
  // 避免在路由守卫中触发 router.push 造成导航冲突
  async function checkLogin() {
    try {
      const res = await fetch('/api/categories', { credentials: 'include' })
      isLoggedIn.value = res.ok
      if (res.ok) {
        await refreshOrg()
      } else {
        orgId.value = null
      }
    } catch {
      isLoggedIn.value = false
      orgId.value = null
    }
  }

  // 是否已完成引导（当前组织）
  function isOnboarded() {
    return !!orgId.value && !!localStorage.getItem(onboardingKey(orgId.value))
  }

  // 标记当前组织已完成引导
  function markOnboarded() {
    if (orgId.value) localStorage.setItem(onboardingKey(orgId.value), '1')
  }

  // 老组织兼容：无标记但该组织已有业务数据（存在往来单位）→ 视为已建账，补写标记，
  // 避免"已在使用的老组织登录后被反复拉进引导页"。
  async function ensureOnboarded() {
    if (!orgId.value || isOnboarded() || onboardChecked.value === orgId.value) return
    try {
      const res = await fetch('/api/parties', { credentials: 'include' })
      if (res.ok) {
        const j = await res.json()
        const list = j?.data ?? j ?? []
        if (Array.isArray(list) && list.length > 0) {
          localStorage.setItem(onboardingKey(orgId.value), '1')
          onboardChecked.value = orgId.value
        }
      }
    } catch {
      // 忽略：查询失败时保持原判定
    }
  }

  async function login(username: string, password: string) {
    loading.value = true
    error.value = ''
    try {
      // 后端成功响应为 { data: { user: { username }, expiresAt } }
      const res = await api.post<ApiResponse<{ user: { username: string } }>>('/auth/login', {
        username,
        password,
      })
      if (res.data.user) {
        isLoggedIn.value = true
        await refreshOrg()
      }
    } catch (e: any) {
      error.value = e.message || '登录失败'
      throw e
    } finally {
      loading.value = false
    }
  }

  // 注册成功即已建立会话，无需重复调用登录（随后由调用方 refreshOrg 填充 orgId）
  function markLoggedIn() {
    isLoggedIn.value = true
  }

  // 会话失效/登出：同时清掉组织 id，避免跨会话/跨组织串用
  function clearAuth() {
    isLoggedIn.value = false
    orgId.value = null
  }

  async function logout() {
    try {
      await api.post('/auth/logout')
    } catch {
      // ignore
    }
    clearAuth()
  }

  return {
    isLoggedIn, loading, error, orgId,
    checkLogin, refreshOrg, isOnboarded, markOnboarded, ensureOnboarded,
    login, markLoggedIn, clearAuth, logout,
  }
})