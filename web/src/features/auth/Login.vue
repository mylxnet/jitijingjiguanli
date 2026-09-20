<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <div class="login-title">{{ displayTitle }}</div>
        <div v-if="orgName" class="login-subtitle">集体经济管理系统</div>
      </div>

      <van-form @submit="handleLogin" @failed="onValidateFailed" :show-error-message="false">
        <van-cell-group inset>
          <van-field
            v-model="username"
            name="username"
            label="账号"
            placeholder="请输入账号"
            :rules="[{ required: true, message: '请填写账号' }]"
          >
            <template #extra>
              <span v-if="error && errorField === 'username'" class="field-error" :title="error">{{ error }}</span>
            </template>
          </van-field>
          <van-field
            v-model="password"
            type="password"
            name="password"
            label="密码"
            placeholder="请输入密码"
            :rules="[{ required: true, message: '请填写密码' }]"
          >
            <template #extra>
              <span v-if="error && errorField === 'password'" class="field-error" :title="error">{{ error }}</span>
            </template>
          </van-field>
        </van-cell-group>

        <div style="margin: 16px">
          <van-button round block type="primary" native-type="submit" :loading="loading">
            登录
          </van-button>
        </div>

      </van-form>

      <div v-if="auth.registrationOpen === true" class="login-footer">
        <span>还没有账号？</span>
        <a class="register-link" @click="goRegister">注册组织</a>
      </div>
      <div class="login-footer" style="margin-top: 4px">
        <a class="register-link" @click="goResetPwd">忘记密码？</a>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore, ORG_NAME_KEY } from './store'
import { api } from '../../lib/http'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')
// 错误提示与输入框同行右侧显示，errorField 决定挂在哪一行
const errorField = ref<'username' | 'password'>('password')

function showError(field: 'username' | 'password', message: string) {
  errorField.value = field
  error.value = message
}

// 必填校验失败：Vant 的默认消息渲染在字段下方会撑高布局，改由 @failed 取字段名后同行显示
function onValidateFailed(payload: { errors: Array<{ name?: string; message: string }> }) {
  const first = payload?.errors?.[0]
  showError(first?.name === 'username' ? 'username' : 'password', first?.message || '请检查输入')
}

// flush:sync —— 登录失败时会先清空密码框再写提示，若用默认的 pre 队列，清空动作会在提示之后才触发而把提示抹掉
watch([username, password], () => {
  error.value = ''
}, { flush: 'sync' })

// 记住上次登录的组织名：大字显示组织名，小字显示系统名
const orgName = ref(localStorage.getItem(ORG_NAME_KEY) || '')
const displayTitle = computed(() => orgName.value || '集体经济管理系统')

// 进入登录页时查询注册是否仍开放；仅"系统中尚无用户"时展示「注册组织」入口
onMounted(() => {
  auth.loadRegistrationStatus()
})

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
    password.value = ''
    showError('password', e.message || '账号或密码错误')
  } finally {
    loading.value = false
  }
}

function goRegister() {
  router.push('/register')
}

function goResetPwd() {
  router.push('/reset-password')
}
</script>

<style scoped>
.login-page {
  /* 外层 .auth-wrap(App.vue:149) 已负责 100vh 居中 + 24px padding，#app/body 已铺 --paper 底。
     这里再声明一遍 min-height:100vh + padding 会让总高变成 100vh+48px，页面凭空多出可滚区，
     点提交时整页跳动 48~96px。 */
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
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
  color: var(--ink);
}

.login-subtitle {
  font-size: 13px;
  color: var(--ink-muted);
  margin-top: 4px;
}

/* 与输入框同行、靠右的错误提示：不占行高（line-height 继承 .van-cell 的 24px），出现/消失都不推移布局 */
.field-error {
  color: var(--danger-deep);
  font-size: 12px;
  line-height: inherit;
  white-space: nowrap;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.login-footer {
  text-align: center;
  padding: 0 16px 24px;
  font-size: 13px;
  color: var(--ink-muted);
}

.register-link {
  color: var(--jade);
  margin-left: 4px;
  cursor: pointer;
}
</style>