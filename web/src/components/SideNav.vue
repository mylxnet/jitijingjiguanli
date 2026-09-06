<template>
  <nav class="side-nav">
	    <div class="side-top">
	      <div class="side-org">{{ orgName }}</div>
	      <div class="side-org-sub">集体经济管理系统</div>
	    </div>
    <div class="side-nav-wrap">
      <a
        v-for="item in items"
        :key="item.label"
        class="side-link"
        :class="{ active: item.path && isActive(item.path) }"
        @click="onItemClick(item)"
      >
        <span class="side-nav-icon">{{ item.iconChar }}</span>
        <span>{{ item.label }}</span>
      </a>
    </div>
    <div class="side-foot">
      <router-link to="/settings" class="side-link-sub" :class="{ active: isActive('/settings') }">
        <span class="side-nav-icon">⚙</span>
        <span>设置</span>
      </router-link>
      <a class="side-link-sub danger" @click="onLogout">
        <span class="side-nav-icon">🚪</span>
        <span>退出</span>
      </a>
      <div class="side-version">v0.10.0</div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../features/auth/store'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const openRecord = inject<() => void>('openRecord', () => {})

const orgName = computed(() => localStorage.getItem('jt_org_name') || '组织')

const items = [
  { label: '收支总览', path: '/', iconChar: '◆' },
  { label: '往来单位', path: '/contacts', iconChar: '◉' },
  { label: '投资管理', path: '/investment', iconChar: '📈' },
  { label: '流转管理', path: '/flow', iconChar: '🏠' },
  { label: '快速记账', path: '', iconChar: '＋', action: 'record' },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

function onItemClick(item: typeof items[number]) {
  if (item.action === 'record') {
    openRecord()
  } else if (item.path) {
    router.push(item.path)
  }
}

async function onLogout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.side-nav {
  width: 196px;
  min-height: 100vh;
  background: var(--jade-deep);
  color: rgba(255, 255, 255, .9);
  padding: 16px 0;
  display: flex;
  flex-direction: column;
  position: sticky;
  top: 0;
  align-self: flex-start;
  box-sizing: border-box;
  flex-shrink: 0;
}

.side-top {
  padding: 0 16px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, .08);
  margin-bottom: 8px;
}

.side-org {
  font-family: 'Noto Serif SC', serif;
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.side-org-sub {
  font-size: 10px;
  color: rgba(255, 255, 255, .5);
  letter-spacing: .1em;
  margin-top: 2px;
}

.side-nav-wrap {
  flex: 1;
  padding: 4px 10px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.side-link,
.side-link-sub {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border-radius: var(--r-sm);
  color: rgba(255, 255, 255, .7);
  text-decoration: none;
  font-size: 13px;
  transition: all .15s;
  user-select: none;
  cursor: pointer;
}

.side-link:hover,
.side-link-sub:hover {
  background: rgba(255, 255, 255, .06);
  color: #fff;
}

.side-link.active {
  background: rgba(255, 255, 255, .15);
  color: #fff;
  font-weight: 600;
}

.side-nav-icon {
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  font-size: 14px;
  flex-shrink: 0;
  background: rgba(255, 255, 255, .1);
  border-radius: 6px;
}

.side-link.active .side-nav-icon {
  background: var(--jade);
}

.side-foot {
  margin-top: auto;
  padding: 8px 10px 0;
  border-top: 1px solid rgba(255, 255, 255, .08);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.side-link-sub.danger:hover {
  background: rgba(163, 58, 45, .2);
  color: #f5c6c0;
}

.side-version {
  text-align: center;
  font-size: 11px;
  color: rgba(255, 255, 255, .35);
  padding: 8px 0 4px;
}
</style>
