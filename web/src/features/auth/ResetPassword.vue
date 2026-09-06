<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <div class="login-title">重置密码</div>
        <div class="login-subtitle">集体经济管理系统</div>
      </div>

      <div class="rp-status" :class="statusClass">{{ statusText }}</div>

      <div v-if="!done" class="rp-spinner">重置中...</div>

      <div v-if="done" style="margin: 16px">
        <van-button round block type="primary" @click="goLogin" :loading="loading">
          返回登录
        </van-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'

const router = useRouter()
const loading = ref(false)
const done = ref(false)
const statusText = ref('正在重置密码...')
const statusClass = ref('')

onMounted(async () => {
  try {
    await api.post('/auth/reset-password', {})
    statusText.value = '密码已重置为 admin888，3 秒后自动跳转登录页'
    statusClass.value = 'rp-success'
    done.value = true
    setTimeout(() => goLogin(), 3000)
  } catch (e: any) {
    statusText.value = e?.response?.message || e?.message || '重置失败'
    statusClass.value = 'rp-error'
    done.value = true
  }
})

function goLogin() {
  loading.value = true
  router.push('/login')
}
</script>

<style scoped>
.rp-status {
  text-align: center;
  margin: 24px 16px;
  padding: 12px;
  border-radius: 8px;
  font-size: 14px;
}
.rp-success {
  background: #f0f9eb;
  color: #67c23a;
}
.rp-error {
  background: #fef0f0;
  color: #f56c6c;
}
.rp-spinner {
  text-align: center;
  color: #999;
  margin: 24px 0;
}
</style>