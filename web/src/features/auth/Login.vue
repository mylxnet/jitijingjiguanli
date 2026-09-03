<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <div class="login-title">{{ displayTitle }}</div>
        <div v-if="orgName" class="login-subtitle">集体经济管理系统</div>
      </div>

      <van-form @submit="handleLogin">
        <van-cell-group inset>
          <van-field
            v-model="username"
            name="username"
            label="账号"
            placeholder="请输入账号"
            :rules="[{ required: true, message: '请填写账号' }]"
          />
          <van-field
            v-model="password"
            type="password"
            name="password"
            label="密码"
            placeholder="请输入密码"
            :rules="[{ required: true, message: '请填写密码' }]"
          />
        </van-cell-group>

        <div v-if="error" class="login-error">{{ error }}</div>

        <div style="margin: 16px">
          <van-button round block type="primary" native-type="submit" :loading="loading">
            登录
          </van-button>
        </div>
      </van-form>

      <div class="login-footer">
        <span>还没有账号？</span>
        <a class="register-link" @click="goRegister">注册组织</a>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore, ORG_NAME_KEY } from './store'
import { api } from '../../lib/http'
import { showToast } from 'vant'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

// 记住上次登录的组织名：大字显示组织名，小字显示系统名
const orgName = ref(localStorage.getItem(ORG_NAME_KEY) || '')
const displayTitle = computed(() => orgName.value || '集体经济管理系统')

async function handleLogin() {
  loading.value = true
  error.value = ''
  try {
    await auth.login(username.value, password.value)
    try {
      const me = await api.get<{ data?: { orgName?: string } }>('/me')
      if (me.data?.orgName) {
        orgName.value = me.data.orgName
        localStorage.setItem(ORG_NAME_KEY, me.data.orgName)
      }
    } catch {
      // 记不住组织名也不影响登录
    }
    router.push('/')
  } catch (e: any) {
    error.value = e.message || '账号或密码错误'
    password.value = ''
  } finally {
    loading.value = false
  }
}

function goRegister() {
  router.push('/register')
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f7f7f5;
  padding: 24px;
}

.login-card {
  width: 100%;
  max-width: 360px;
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}

.login-header {
  text-align: center;
  padding: 32px 16px 20px;
}

.login-title {
  font-size: 22px;
  font-weight: 600;
  color: #2c2c2a;
}

.login-subtitle {
  font-size: 13px;
  color: #8f8e88;
  margin-top: 4px;
}

.login-error {
  color: #a32d2d;
  font-size: 13px;
  text-align: center;
  padding: 8px 16px 0;
}

.login-footer {
  text-align: center;
  padding: 0 16px 24px;
  font-size: 13px;
  color: #8f8e88;
}

.register-link {
  color: #0f6e56;
  margin-left: 4px;
  cursor: pointer;
}
</style>