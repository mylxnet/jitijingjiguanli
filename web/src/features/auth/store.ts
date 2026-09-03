import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, setOnUnauthorized } from '../../lib/http'
import type { ApiResponse } from '../../types/api'

export const useAuthStore = defineStore('auth', () => {
  const isLoggedIn = ref(false)
  const loading = ref(false)
  const error = ref('')

  // 检查是否已登录（通过 Cookie session）
  async function checkLogin() {
    try {
      // 尝试调用一个受保护的 API 来验证会话
      await api.get<any>('/categories')
      isLoggedIn.value = true
    } catch {
      isLoggedIn.value = false
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

  async function logout() {
    try {
      await api.post('/auth/logout')
    } catch {
      // ignore
    }
    isLoggedIn.value = false
  }

  return { isLoggedIn, loading, error, checkLogin, login, logout }
})