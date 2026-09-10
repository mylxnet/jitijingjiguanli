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

    <!-- 账号与科目 -->
    <van-cell-group inset style="margin-top: 16px">
      <div class="section-label">查询</div>
      <van-cell title="流水清单" is-link to="/transactions" />
      <van-cell title="科目汇总" is-link to="/summary" />
      <div class="section-label">账号</div>
      <van-cell title="科目管理" is-link to="/categories" />
      <van-cell title="修改密码" is-link @click="openPwdDialog" />
    </van-cell-group>

    <!-- 备份与恢复 -->
    <van-cell-group inset style="margin-top: 16px">
      <div class="section-label">备份与恢复</div>
      <div class="backup-tip">
        每日 03:00 自动备份，保留最近 {{ keepBackup }} 份；备份目录建议放在与数据不同的磁盘
      </div>
      <div style="margin: 4px 16px 8px">
        <van-button
          round
          block
          type="primary"
          size="small"
          :loading="backingUp"
          @click="createBackup"
        >立即备份</van-button>
      </div>
      <div v-if="backups.length === 0" class="backup-empty">尚无备份记录</div>
      <van-cell v-for="(b, i) in backups" :key="b.id" :title="formatBackupTime(b.createdAt)">
        <template #label>
          <span class="backup-size">{{ formatBytes(b.sizeBytes) }}</span>
        </template>
        <template #right-icon>
          <van-button size="mini" plain type="warning" @click="restoreBackup(b)">恢复</van-button>
          <van-button
            size="mini"
            plain
            :disabled="i === 0"
            class="backup-del-btn"
            @click="deleteBackup(b)"
          >删除</van-button>
        </template>
      </van-cell>
    </van-cell-group>

    <!-- 操作日志 -->
    <van-cell-group inset style="margin-top: 16px">
      <div class="section-label">操作日志</div>
      <div style="margin: 4px 16px 8px">
        <van-button
          round
          block
          size="small"
          @click="showOpLog = true"
        >查看操作日志</van-button>
      </div>
    </van-cell-group>

    <!-- 系统重置 -->
    <van-cell-group inset style="margin-top: 16px">
      <div class="section-label danger">危险操作</div>
      <div style="margin: 4px 16px 8px">
        <van-button
          round
          block
          size="small"
          :loading="resetting"
          @click="handleReset"
        >系统重置</van-button>
      </div>
      <div class="reset-tip">
        清空所有业务数据，保留预置科目结构。重置后需重新注册。
      </div>
    </van-cell-group>

    <div style="margin: 16px; padding: 0 16px">
      <van-button round block type="danger" @click="handleLogout">登出</van-button>
    </div>

    <div class="version">v{{ APP_VERSION }}</div>

    <!-- 修改密码 -->
    <van-popup v-model:show="showPwd" :position="popupPos()" round closeable style="max-height: 90vh">
      <div class="pwd-popup">
        <div class="popup-title">修改密码</div>
        <van-field v-model="pwdForm.oldPassword" type="password" label="原密码" placeholder="当前登录密码" />
        <van-field v-model="pwdForm.newPassword" type="password" label="新密码" placeholder="至少 6 位" />
        <van-field v-model="pwdForm.confirm" type="password" label="确认新密码" placeholder="再次输入新密码" />
        <div v-if="pwdError" class="pwd-error">{{ pwdError }}</div>
        <div class="pwd-save">
          <van-button round block type="primary" :loading="savingPwd" @click="savePassword">保存新密码</van-button>
        </div>
      </div>
    </van-popup>

    <!-- 恢复中遮罩 -->
    <van-overlay :show="restoring" :z-index="2000">
      <div class="restore-mask">
        <van-loading color="#fff" size="36" />
        <div class="restore-text">正在恢复，请勿关闭页面</div>
      </div>
    </van-overlay>

    <!-- 操作日志 -->
    <OperationLogDialog v-model:show="showOpLog" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../auth/store'
import { APP_VERSION } from '../../version'
import { api } from '../../lib/http'
import { formatFen } from '../../types/api'
import type { ApiResponse } from '../../types/api'
import { showToast, showDialog } from 'vant'

import { popupPos } from '../../composables/useScreen';
import OperationLogDialog from './OperationLogDialog.vue'
interface Settings {
  bankOpeningBalanceCents: number
}

interface BackupItem {
  id: string
  sizeBytes: number
  createdAt: string
}

const router = useRouter()
const auth = useAuthStore()

const bankBalance = ref('')
const currentBankBalance = ref<number | null>(null)
const saving = ref(false)

const keepBackup = 30
const backingUp = ref(false)
const backups = ref<BackupItem[]>([])
const restoring = ref(false)
const showOpLog = ref(false)

const showPwd = ref(false)
const savingPwd = ref(false)
const pwdError = ref('')
const pwdForm = ref({ oldPassword: '', newPassword: '', confirm: '' })
const resetting = ref(false)

onMounted(async () => {
  await Promise.all([loadSettings(), loadBackups()])
})

async function loadSettings() {
  try {
    const res = await api.get<ApiResponse<Settings>>('/settings')
    const val = res.data.bankOpeningBalanceCents
    bankBalance.value = val > 0 ? (val / 100).toFixed(2) : ''
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

// ---- 修改密码 ----
function openPwdDialog() {
  pwdForm.value = { oldPassword: '', newPassword: '', confirm: '' }
  pwdError.value = ''
  showPwd.value = true
}

async function savePassword() {
  pwdError.value = ''
  if (!pwdForm.value.oldPassword || !pwdForm.value.newPassword) {
    pwdError.value = '请填写原密码与新密码'
    return
  }
  if (pwdForm.value.newPassword.length < 6) {
    pwdError.value = '新密码至少 6 位'
    return
  }
  if (pwdForm.value.newPassword !== pwdForm.value.confirm) {
    pwdError.value = '两次输入的新密码不一致'
    return
  }
  savingPwd.value = true
  try {
    await api.put('/auth/password', {
      oldPassword: pwdForm.value.oldPassword,
      newPassword: pwdForm.value.newPassword,
    })
    showPwd.value = false
    showToast('修改成功')
  } catch (e: any) {
    pwdError.value = e.message || '修改失败'
  } finally {
    savingPwd.value = false
  }
}

// ---- 备份与恢复 ----
async function loadBackups() {
  try {
    const res = await api.get<ApiResponse<{ items: BackupItem[] }>>('/backups')
    backups.value = res.data.items || []
  } catch {
    backups.value = []
  }
}

async function createBackup() {
  backingUp.value = true
  try {
    await api.post('/backups')
    showToast('备份完成')
    await loadBackups()
  } catch (e: any) {
    showToast(e.message || '备份失败')
  } finally {
    backingUp.value = false
  }
}

async function restoreBackup(b: BackupItem) {
  try {
    await showDialog({
      title: '确认恢复',
      message: `将用 ${formatBackupTime(b.createdAt)} 的备份覆盖当前数据，之后需要重新登录。确定恢复吗？`,
      showCancelButton: true,
    })
  } catch {
    return // 取消
  }
  restoring.value = true
  try {
    await api.post(`/backups/${encodeURIComponent(b.id)}/restore`)
    showToast('恢复成功，请重新登录')
    await auth.logout()
    router.push('/login')
  } catch (e: any) {
    restoring.value = false
    showToast(e.message || '恢复失败')
  }
}

async function deleteBackup(b: BackupItem) {
  try {
    await showDialog({
      title: '确认删除',
      message: `确定删除 ${formatBackupTime(b.createdAt)} 的备份吗？此操作不可恢复。`,
      showCancelButton: true,
    })
  } catch {
    return // 取消
  }
  try {
    await api.del(`/backups/${encodeURIComponent(b.id)}`)
    showToast('删除成功')
    await loadBackups()
  } catch (e: any) {
    showToast(e.message || '删除失败')
  }
}

function formatBackupTime(ts: string): string {
  if (!ts) return ts
  // jz-backup-YYYYMMDD-HHMMSS.db → YYYY-MM-DD HH:MM
  const m = ts.match(/^(\d{4})(\d{2})(\d{2})-(\d{2})(\d{2})(\d{2})$/)
  if (m) return `${m[1]}-${m[2]}-${m[3]} ${m[4]}:${m[5]}`
  return ts
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(2)} MB`
}

async function handleLogout() {
  await auth.logout()
  router.push('/login')
}

async function handleReset() {
  // 第一次确认：警告
  try {
    await showDialog({
      title: '警告：系统重置',
      message: '此操作将清空所有业务数据（往来单位、流水、应收、合同、操作日志等），\n仅保留预置科目结构。\n\n重置后需重新注册，数据不可恢复！',
      showCancelButton: true,
      confirmButtonText: '我已了解，继续',
    })
  } catch {
    return // 取消
  }
  // 第二次确认：输入文字
  const confirmText = '确认重置'
  const input = prompt(`请输入"${confirmText}"确认重置：`)
  if (input !== confirmText) {
    if (input !== null) showToast('输入不正确，已取消')
    return
  }
  resetting.value = true
  try {
    await api.post('/system/reset')
	    // 重置会删除组织行，重新注册的组织 onboarded 默认为 0，会再次进入引导页
	    showToast('系统已重置，即将跳转到注册页')
	    await auth.logout()
	    router.push('/register')
  } catch (e: any) {
    showToast(e.message || '重置失败')
  } finally {
    resetting.value = false
  }
}
</script>

<style scoped>
.settings-page {
  padding: 16px 0;
padding-bottom: 60px;
  min-height: 100vh;
  background: var(--paper);
}

.page-header {
  padding: 0 16px;
  margin-bottom: 16px;
}

.page-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: var(--ink);
  margin: 0;
}

.section-label {
  font-size: 12px;
  color: var(--ink-muted);
  padding: 12px 16px 0;
}
.section-label.danger {
  color: var(--expense, #e74c3c);
  font-weight: 600;
}

.current-balance {
  font-size: 12px;
  color: var(--ink-soft);
  padding: 0 16px 8px;
}

.version {
  text-align: center;
  font-size: 12px;
  color: var(--ink-muted);
  margin-top: 24px;
}

.backup-tip {
  font-size: 12px;
  color: var(--ink-muted);
  padding: 8px 16px 4px;
  line-height: 1.5;
}

.backup-empty {
  text-align: center;
  font-size: 12px;
  color: var(--ink-muted);
  padding: 12px 0;
}

.backup-size {
  font-size: 11px;
  color: var(--ink-muted);
}

.reset-tip {
  font-size: 11px;
  color: var(--expense, #e74c3c);
  padding: 0 16px 12px;
  line-height: 1.5;
}

.pwd-popup {
  padding: 16px 0 24px;
  max-height: 80vh;
  overflow-y: auto;
}

.popup-title {
  font-size: 16px;
  font-weight: 500;
  color: var(--ink);
  padding: 0 16px 12px;
}

.pwd-error {
  color: var(--expense);
  font-size: 13px;
  padding: 0 16px 8px;
}

.pwd-save {
  margin: 8px 16px 0;
}

.restore-mask {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
}

.restore-text {
  color: #fff;
  font-size: 14px;
}
</style>
