<template>
  <div class="users-page">
    <div class="page-header">
      <h3>用户管理</h3>
    </div>

    <!-- 统计卡（沿用合同管理页统计卡风格） -->
    <div class="stats">
      <div class="stat c1">
        <div class="stat-k">账号总数</div>
        <div class="stat-v">{{ users.length }}</div>
      </div>
      <div class="stat c2">
        <div class="stat-k">高级用户</div>
        <div class="stat-v">{{ adminCount }}</div>
      </div>
      <div class="stat c3">
        <div class="stat-k">一般用户</div>
        <div class="stat-v">{{ viewerCount }}</div>
      </div>
      <div class="stat c4">
        <div class="stat-k">已停用</div>
        <div class="stat-v">{{ disabledCount }}</div>
      </div>
    </div>

    <!-- 工具栏 -->
    <div class="toolbar">
      <span class="hint">
        <template v-if="auth.isOwner">所有者可创建<b>高级用户</b>与<b>一般用户</b>，并管理除自己外的任何账号。</template>
        <template v-else>高级用户只能创建与管理<b>一般用户</b>。</template>
      </span>
      <van-button size="small" plain type="primary" @click="openCreate">
        <van-icon name="plus" /> 新建账号
      </van-button>
    </div>

    <!-- 账号列表 -->
    <div class="user-list">
      <div class="user-row head">
        <div>账号</div>
        <div>角色</div>
        <div>状态</div>
        <div class="acts-col">操作</div>
      </div>

      <div v-if="loading" class="user-loading"><van-loading size="20" /> 加载中...</div>
      <div v-else-if="users.length === 0" class="user-empty">暂无账号</div>
      <template v-else>
        <div v-for="u in users" :key="u.id" class="user-row">
          <div class="who">
            <span class="avatar">{{ (u.username || '?').charAt(0) }}</span>
            <div class="who-main">
              <b>{{ u.username }}</b>
              <small>{{ formatDate(u.createdAt) }} 创建</small>
            </div>
          </div>
          <div><span class="chip" :class="'role-' + u.role">{{ roleLabel(u.role) }}</span></div>
          <div>
            <span class="chip" :class="u.status === 'disabled' ? 'st-off' : 'st-on'">
              {{ u.status === 'disabled' ? '已停用' : '启用' }}
            </span>
          </div>
          <div class="acts">
            <span v-if="u.id === auth.userId" class="hint">当前登录账号</span>
            <template v-else>
              <van-button
                v-if="auth.isOwner"
                size="mini"
                plain
                :disabled="!canOperate(u)"
                @click="openRole(u)"
              >改角色</van-button>
              <van-button
                size="mini"
                plain
                :disabled="!canOperate(u)"
                @click="toggleStatus(u)"
              >{{ u.status === 'disabled' ? '启用' : '停用' }}</van-button>
              <van-button
                size="mini"
                plain
                :disabled="!canOperate(u)"
                @click="resetPassword(u)"
              >重置密码</van-button>
            </template>
          </div>
        </div>
      </template>
    </div>

    <!-- 新建账号 -->
    <van-popup v-model:show="showCreate" :position="popupPos()" round closeable style="max-height: 90vh">
      <div class="popup-body">
        <div class="popup-title">新建账号</div>
        <div class="uform">
          <div class="uform-row">
            <label>账号</label>
            <div class="uform-control">
              <input v-model="createForm.username" class="uinput" placeholder="登录账号，如 zhangkuaiji" />
            </div>
          </div>
          <div class="uform-row">
            <label>初始密码</label>
            <div class="uform-control">
              <input v-model="createForm.password" type="password" class="uinput" placeholder="至少 6 位" />
              <div class="uform-note">首次登录后建议由本人在「设置-修改密码」中修改</div>
            </div>
          </div>
          <div class="uform-row">
            <label>角色</label>
            <div class="uform-control">
              <select v-model="createForm.role" class="uselect">
                <option v-for="r in creatableRoles" :key="r" :value="r">{{ roleLabel(r) }}</option>
              </select>
              <div class="uform-note">
                <template v-if="auth.isOwner">高级用户：可记账、可管理一般用户；一般用户：仅可查看与导出。</template>
                <template v-else>高级用户账号只能创建一般用户。</template>
              </div>
            </div>
          </div>
          <div v-if="createError" class="uform-err">{{ createError }}</div>
        </div>
        <div class="popup-foot">
          <van-button size="small" plain @click="showCreate = false">取消</van-button>
          <van-button size="small" plain type="primary" :loading="creating" @click="submitCreate">确定创建</van-button>
        </div>
      </div>
    </van-popup>

    <!-- 调整角色（仅所有者） -->
    <van-popup v-model:show="showRole" :position="popupPos()" round closeable style="max-height: 90vh">
      <div class="popup-body">
        <div class="popup-title">调整角色</div>
        <div class="uform">
          <div class="uform-row">
            <label>账号</label>
            <div class="uform-control">
              <b style="font-size: 14px">{{ roleTarget?.username }}</b>
              <span class="chip" :class="'role-' + (roleTarget?.role || 'viewer')">{{ roleLabel(roleTarget?.role || '') }}</span>
            </div>
          </div>
          <div class="uform-row">
            <label>调整为</label>
            <div class="uform-control">
              <select v-model="roleValue" class="uselect">
                <option value="admin">高级用户</option>
                <option value="viewer">一般用户</option>
              </select>
              <div class="uform-note">高级用户：可记账并管理一般用户；一般用户：仅可查看与导出。</div>
            </div>
          </div>
          <div v-if="roleError" class="uform-err">{{ roleError }}</div>
        </div>
        <div class="popup-foot">
          <van-button size="small" plain @click="showRole = false">取消</van-button>
          <van-button size="small" plain type="primary" :loading="roleSaving" @click="submitRole">保存</van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { showConfirmDialog, showToast } from 'vant'
import { api } from '../../lib/http'
import { popupPos } from '../../composables/useScreen'
import { useAuthStore } from '../auth/store'

interface UserRow {
  id: number
  username: string
  role: string
  status: string
  createdAt: string
}

const auth = useAuthStore()

const users = ref<UserRow[]>([])
const loading = ref(false)

const ROLE_LABEL: Record<string, string> = { owner: '所有者', admin: '高级用户', viewer: '一般用户' }
function roleLabel(r: string) { return ROLE_LABEL[r] || r }

const adminCount = computed(() => users.value.filter(u => u.role === 'admin').length)
const viewerCount = computed(() => users.value.filter(u => u.role === 'viewer').length)
const disabledCount = computed(() => users.value.filter(u => u.status === 'disabled').length)

function formatDate(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

async function loadUsers() {
  loading.value = true
  try {
    const res = await api.get<{ data?: { items?: UserRow[] } }>('/users')
    users.value = res.data?.items || []
  } catch (e: any) {
    showToast(e?.message || '读取账号列表失败')
    users.value = []
  } finally {
    loading.value = false
  }
}
onMounted(loadUsers)

// 可操作某一行：不能操作自己；owner 可操作除自己外任何人；admin 仅能操作一般用户
function canOperate(u: UserRow): boolean {
  if (u.id === auth.userId) return false
  if (auth.isOwner) return true
  return u.role === 'viewer'
}

// ---- 新建账号 ----
const showCreate = ref(false)
const createForm = ref({ username: '', password: '', role: 'viewer' })
const createError = ref('')
const creating = ref(false)
// 可创建角色范围与后端同源约束：owner→admin/viewer；admin→viewer
const creatableRoles = computed<string[]>(() => (auth.isOwner ? ['admin', 'viewer'] : ['viewer']))

function openCreate() {
  createForm.value = { username: '', password: '', role: creatableRoles.value[0] }
  createError.value = ''
  showCreate.value = true
}

async function submitCreate() {
  createError.value = ''
  const f = createForm.value
  if (!f.username.trim()) { createError.value = '请填写账号'; return }
  if (f.password.length < 6) { createError.value = '密码至少 6 位'; return }
  creating.value = true
  try {
    await api.post('/users', { username: f.username.trim(), password: f.password, role: f.role })
    showCreate.value = false
    showToast('账号已创建')
    await loadUsers()
  } catch (e: any) {
    createError.value = e?.message || '创建失败'
  } finally {
    creating.value = false
  }
}

// ---- 调整角色（仅所有者） ----
const showRole = ref(false)
const roleTarget = ref<UserRow | null>(null)
const roleValue = ref('viewer')
const roleSaving = ref(false)
const roleError = ref('')

function openRole(u: UserRow) {
  roleTarget.value = u
  roleValue.value = u.role === 'admin' ? 'admin' : 'viewer'
  roleError.value = ''
  showRole.value = true
}

async function submitRole() {
  if (!roleTarget.value) return
  roleError.value = ''
  roleSaving.value = true
  try {
    await api.patch(`/users/${roleTarget.value.id}`, { role: roleValue.value })
    showRole.value = false
    showToast('角色已调整')
    await loadUsers()
  } catch (e: any) {
    roleError.value = e?.message || '调整失败'
  } finally {
    roleSaving.value = false
  }
}

// ---- 启用 / 停用 ----
async function toggleStatus(u: UserRow) {
  const next = u.status === 'disabled' ? 'active' : 'disabled'
  const isDisable = next === 'disabled'
  try {
    await showConfirmDialog({
      title: isDisable ? '确认停用账号' : '确认启用账号',
      message: isDisable
        ? `停用后，${u.username} 将无法登录，其会话立即失效。账号不会被删除，之后可重新启用。`
        : `启用后，${u.username} 可重新登录使用。`,
      confirmButtonText: isDisable ? '确定停用' : '确定启用',
    })
  } catch {
    return // 取消
  }
  try {
    await api.patch(`/users/${u.id}`, { status: next })
    showToast(isDisable ? '已停用该账号' : '已启用该账号')
    await loadUsers()
  } catch (e: any) {
    showToast(e?.message || '操作失败')
  }
}

// ---- 重置密码 ----
async function resetPassword(u: UserRow) {
  try {
    await showConfirmDialog({
      title: '确认重置密码',
      message: `将把 ${u.username} 的密码重置为初始密码 admin888，并强制其下线。请提醒本人登录后立即修改密码。`,
      confirmButtonText: '确定重置',
    })
  } catch {
    return // 取消
  }
  try {
    await api.post(`/users/${u.id}/reset-password`)
    showToast('密码已重置为 admin888')
  } catch (e: any) {
    showToast(e?.message || '重置失败')
  }
}
</script>

<style scoped>
.users-page {
  padding: 16px;
  min-height: 100vh;
  background: var(--paper);
}

.page-header {
  margin-bottom: 14px;
}

.page-header h3 {
  font-size: 16px;
  font-weight: 500;
  color: var(--ink);
  margin: 0;
}

/* 统计卡：浅蓝/浅绿/浅紫/浅橙 */
.stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  margin-bottom: 14px;
}

.stat {
  border-radius: var(--r-sm);
  padding: 10px 12px;
  box-shadow: var(--shadow-sm);
}

.stat-k {
  font-size: 12px;
  color: var(--ink-muted);
}

.stat-v {
  font-size: 16px;
  font-weight: 700;
  color: var(--ink);
  margin-top: 2px;
}

.stat.c1 { background: #e8f0fb; }
.stat.c2 { background: #e6f5f4; }
.stat.c3 { background: #f1edfc; }
.stat.c4 { background: #fff1e0; }

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.hint {
  font-size: 12px;
  color: var(--ink-muted);
}

.hint b {
  color: var(--jade);
  font-weight: 600;
}

/* 账号列表 */
.user-list {
  background: #fff;
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  overflow: hidden;
}

.user-row {
  display: grid;
  grid-template-columns: 1.6fr 120px 100px 260px;
  gap: 12px;
  align-items: center;
  padding: 12px 16px;
  border-top: 1px solid var(--line);
}

.user-row:first-child {
  border-top: 0;
}

.user-row.head {
  background: var(--paper);
  font-size: 12px;
  color: var(--ink-muted);
  padding: 10px 16px;
}

.acts-col {
  text-align: right;
}

.who {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.avatar {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: var(--jade-light);
  color: var(--jade);
  display: grid;
  place-items: center;
  font-size: 14px;
  font-weight: 600;
  flex-shrink: 0;
}

.who-main {
  min-width: 0;
}

.who-main b {
  font-size: 14px;
  display: block;
  color: var(--ink);
}

.who-main small {
  font-size: 11px;
  color: var(--ink-muted);
}

.chip {
  display: inline-block;
  font-size: 12px;
  border-radius: 10px;
  padding: 2px 10px;
  white-space: nowrap;
}

.role-owner { background: var(--jade); color: #fff; }
.role-admin { background: #e6f5f4; color: #1f5c48; }
.role-viewer { background: #f1edfc; color: #6b4fa8; }
.st-on { background: var(--jade-light); color: var(--jade); }
.st-off { background: #f0ede7; color: var(--ink-muted); }

.acts {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  flex-wrap: wrap;
  align-items: center;
}

.user-loading,
.user-empty {
  text-align: center;
  color: var(--ink-muted);
  font-size: 13px;
  padding: 32px 0;
}

/* 弹窗 */
.popup-body {
  width: min(520px, calc(100vw - 32px));
  padding: 18px 16px 20px;
  max-height: 80vh;
  overflow-y: auto;
  box-sizing: border-box;
}

.popup-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--ink);
  padding-left: 8px;
  margin-bottom: 8px;
}

.uform-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 8px;
  border-bottom: 1px solid var(--line);
}

.uform-row label {
  width: 72px;
  font-size: 13px;
  color: var(--ink-soft);
  flex-shrink: 0;
  line-height: 38px;
}

.uform-control {
  flex: 1;
  min-width: 0;
}

.uinput,
.uselect {
  width: 100%;
  height: 38px;
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 0 12px;
  font-size: 13px;
  color: var(--ink);
  background: #fff;
  outline: none;
  box-sizing: border-box;
}

.uinput:focus,
.uselect:focus {
  border-color: var(--jade);
}

.uform-note {
  font-size: 11px;
  color: var(--ink-muted);
  margin-top: 6px;
  line-height: 1.5;
}

.uform-err {
  color: var(--danger);
  font-size: 12px;
  padding: 8px 8px 0;
}

.popup-foot {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  padding-top: 16px;
}
</style>
