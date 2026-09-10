<template>
  <div class="register-page">
    <div class="register-card">
      <div class="register-header">
        <div class="register-title">注册组织</div>
        <div class="register-subtitle">一个组织一个账号，注册后即可开始记账</div>
      </div>

      <van-form @submit="handleRegister">
        <van-cell-group inset>
          <van-field
            v-model="orgName"
            name="orgName"
            label="组织名称"
            placeholder="如：XX村"
            :rules="[{ required: true, message: '请填写组织名称' }]"
          />
          <van-field
            v-model="username"
            name="username"
            label="账号"
            placeholder="登录账号"
            :rules="[{ required: true, message: '请填写账号' }]"
          />
          <van-field
            v-model="password"
            type="password"
            name="password"
            label="密码"
            placeholder="至少 6 位"
            :rules="[{ required: true, message: '请填写密码' }, { validator: (v) => v.length >= 6, message: '密码至少 6 位' }]"
          />
          <van-field
            v-model="confirm"
            type="password"
            name="confirm"
            label="确认密码"
            placeholder="再次输入密码"
            :rules="[{ required: true, message: '请再次输入密码' }, { validator: (v) => v === password, message: '两次输入的密码不一致' }]"
          />
        </van-cell-group>

        <div v-if="error" class="register-error">{{ error }}</div>

        <div style="margin: 16px">
          <van-button round block type="primary" native-type="submit" :loading="loading">
            注册并登录
          </van-button>
          <van-button round block plain style="margin-top: 8px" @click="goLogin">
            已有账号，去登录
          </van-button>
        </div>
      </van-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore, ORG_NAME_KEY } from './store'
import { api } from '../../lib/http'
import { showToast } from 'vant'
import type { ApiResponse } from '../../types/api'

const router = useRouter()
const auth = useAuthStore()

const orgName = ref('')
const username = ref('')
const password = ref('')
const confirm = ref('')
const loading = ref(false)
const error = ref('')

async function handleRegister() {
  loading.value = true
  error.value = ''
  try {
    // 注册成功即建立会话（后端 Set-Cookie），返回与登录一致的结构
    const res = await api.post<ApiResponse<{ user: { username: string } }>>('/auth/register', {
      orgName: orgName.value,
      username: username.value,
      password: password.value,
    })
    if (res.data.user) {
      localStorage.setItem(ORG_NAME_KEY, orgName.value.trim())
      auth.markLoggedIn()
      await auth.refreshOrg() // 填充 orgId，保证引导标记与守卫判定使用同一组织键
      showToast('注册成功，已自动登录')
      router.push('/onboarding')
    }
  } catch (e: any) {
    if (e?.response?.code === 'USERNAME_TAKEN') {
      error.value = '该账号已被注册，请换一个账号'
    } else if (e?.response?.code === 'REGISTRATION_CLOSED') {
      error.value = '系统已注册，禁止重复注册'
    } else {
      error.value = e.message || '注册失败'
    }
  } finally {
    loading.value = false
  }
}

function goLogin() {
  router.push('/login')
}
</script>

<style scoped>
.register-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--paper);
  padding: 24px;
}

.register-card {
  width: 100%;
  max-width: 360px;
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}

.register-header {
  text-align: center;
  padding: 32px 16px 20px;
}

.register-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--ink);
}

.register-subtitle {
  font-size: 12px;
  color: var(--ink-muted);
  margin-top: 6px;
}

.register-error {
  color: var(--expense);
  font-size: 13px;
  text-align: center;
  padding: 8px 16px 0;
}
</style>
