import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '../../lib/http'
import type { ApiResponse } from '../../types/api'

// 本机记住的组织名（登录页大字展示）
export const ORG_NAME_KEY = 'jt_org_name'

export const useAuthStore = defineStore('auth', () => {
  const isLoggedIn = ref(false)
  const loading = ref(false)
  const error = ref('')
  // 当前登录用户所属组织 id（用于组织维度场景）
  const orgId = ref<string | number | null>(null)
  // 是否已完成（或已跳过）引导——以服务端 org.onboarded 为唯一权威来源，
  // 不再依赖 localStorage 标记，避免清缓存/换浏览器/换访问地址被误判为未建账。
  const onboarded = ref(false)
  // 系统是否仍可注册（尚无任何用户才为 true）；null = 尚未取到。
  // 登录页仅在确认为 true 时展示「注册组织」入口，避免已注册的系统挂着一个必然失败的死链接。
  const registrationOpen = ref<boolean | null>(null)

  // 查询注册开放状态（公开接口，无需登录）
  async function loadRegistrationStatus() {
    try {
      const res = await fetch('/api/auth/registration-status', { credentials: 'include' }).then(r => r.json())
      registrationOpen.value = !!res?.data?.open
    } catch {
      registrationOpen.value = null
    }
  }

  // 拉取当前组织信息（登录/注册/首屏后统一调用，保证 orgId/onboarded 始终可用）
  async function refreshOrg() {
    try {
      const me = await fetch('/api/me', { credentials: 'include' }).then(r => r.json())
      orgId.value = me?.data?.orgID ?? me?.data?.orgId ?? null
      onboarded.value = !!me?.data?.onboarded
    } catch {
      orgId.value = null
      onboarded.value = false
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
        onboarded.value = false
      }
    } catch {
      isLoggedIn.value = false
      orgId.value = null
      onboarded.value = false
    }
  }

  // 是否已完成引导（当前组织）。服务端权威：取不到组织信息时为 false，
  // 而路由守卫要求 orgId 有值才判定，因此请求异常时不会误跳引导页。
  function isOnboarded() {
    return !!orgId.value && onboarded.value
  }

  // 标记当前组织已完成（或已跳过）引导：写服务端，成功后才置位
  async function markOnboarded() {
    if (!orgId.value) await refreshOrg()
    if (!orgId.value) throw new Error('无法确认组织信息')
    await api.post('/api/onboarding/complete')
    onboarded.value = true
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

  // 会话失效/登出：同时清掉组织信息，避免跨会话/跨组织串用
  function clearAuth() {
    isLoggedIn.value = false
    orgId.value = null
    onboarded.value = false
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
    isLoggedIn, loading, error, orgId, onboarded, registrationOpen,
    checkLogin, refreshOrg, isOnboarded, markOnboarded, loadRegistrationStatus,
    login, markLoggedIn, clearAuth, logout,
  }
})
