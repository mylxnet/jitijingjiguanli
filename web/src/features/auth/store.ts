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
  // 当前登录用户所属组织 id（用于引导完成标记等按组织区分的场景）
  const orgId = ref<string | number | null>(null)

  // 检查是否已登录（通过 Cookie session）
  // 使用原生 fetch 绕过 http.ts 的 onUnauthorized 回调，
  // 避免在路由守卫中触发 router.push 造成导航冲突
  async function checkLogin() {
    try {
      const res = await fetch('/api/categories', { credentials: 'include' })
      isLoggedIn.value = res.ok
      if (res.ok) {
        try {
          const me = await fetch('/api/me', { credentials: 'include' }).then(r => r.json())
          orgId.value = me?.data?.orgID ?? me?.data?.orgId ?? null
        } catch {
          orgId.value = null
        }
      } else {
        orgId.value = null
      }
    } catch {
      isLoggedIn.value = false
      orgId.value = null
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
      }
    } catch (e: any) {
      error.value = e.message || '登录失败'
      throw e
    } finally {
      loading.value = false
    }
  }

  // 注册成功即已建立会话，无需重复调用登录
  function markLoggedIn() {
    isLoggedIn.value = true
  }

  async function logout() {
    try {
      await api.post('/auth/logout')
    } catch {
      // ignore
    }
    isLoggedIn.value = false
  }

  return { isLoggedIn, loading, error, orgId, checkLogin, login, markLoggedIn, logout }
})