<template>
  <div class="settings-page">
    <div class="page-header">
      <h3>设置</h3>
    </div>

    <!-- 资金账户 -->
    <van-cell-group inset>
      <div class="section-label">资金账户</div>
      <van-field
        v-model="bankBalance"
        label="银行存款期初余额"
        type="number"
        placeholder="0.00"
        :disabled="saving"
      />
      <div v-if="currentBankBalance !== null" class="current-balance">
        当前银行存款余额：{{ formatFen(currentBankBalance) }}
      </div>
      <div style="margin: 12px 16px">
        <van-button
          round
          block
          type="primary"
          size="small"
          :loading="saving"
          @click="saveBankBalance"
        >保存</van-button>
      </div>
    </van-cell-group>

    <van-cell-group inset style="margin-top: 16px">
      <van-cell title="科目管理" is-link to="/categories" />
      <van-cell title="修改密码" is-link @click="showToast('v0.3.0 实现')" />
    </van-cell-group>

    <div style="margin: 16px; padding: 0 16px">
      <van-button round block type="danger" @click="handleLogout">登出</van-button>
    </div>

    <div class="version">v0.2.0</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../auth/store'
import { api } from '../../lib/http'
import { formatFen } from '../../types/api'
import type { ApiResponse } from '../../types/api'
import { showToast } from 'vant'

interface Settings {
  bankOpeningBalanceCents: number
}

const router = useRouter()
const auth = useAuthStore()

const bankBalance = ref('')
const currentBankBalance = ref<number | null>(null)
const saving = ref(false)

onMounted(async () => {
  await loadSettings()
})

async function loadSettings() {
  try {
    const res = await api.get<ApiResponse<Settings>>('/settings')
    const val = res.data.bankOpeningBalanceCents
    bankBalance.value = val > 0 ? (val / 100).toFixed(2) : ''
    // 计算当前余额需要汇总接口
    try {
      const summaryRes = await api.get<ApiResponse<any>>('/summary', { from: '', to: '' })
      currentBankBalance.value = summaryRes.data.capital?.bankBalanceCents ?? null
    } catch {
      currentBankBalance.value = null
    }
  } catch {
    // 忽略
  }
}

async function saveBankBalance() {
  const cents = Math.round(parseFloat(bankBalance.value || '0') * 100)
  if (cents < 0) {
    showToast('期初余额不能为负数')
    return
  }
  saving.value = true
  try {
    await api.put('/settings', { bankOpeningBalanceCents: cents })
    showToast('保存成功')
    await loadSettings()
  } catch (e: any) {
    showToast(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleLogout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.settings-page {
  padding: 16px 0;
  padding-bottom: 60px;
  min-height: 100vh;
  background: #f7f7f5;
}

.page-header {
  padding: 0 16px;
  margin-bottom: 16px;
}

.page-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: #2c2c2a;
  margin: 0;
}

.section-label {
  font-size: 12px;
  color: #8f8e88;
  padding: 12px 16px 0;
}

.current-balance {
  font-size: 12px;
  color: #5f5e5a;
  padding: 0 16px 8px;
}

.version {
  text-align: center;
  font-size: 12px;
  color: #8f8e88;
  margin-top: 24px;
}
</style>