<template>
  <nav class="side-nav">
    <router-link v-for="item in items" :key="item.path" :to="item.path" class="side-link" :class="{ active: isActive(item.path) }">
      <van-icon :name="item.icon" />
      <span>{{ item.label }}</span>
    </router-link>
    <div class="side-foot">
      <router-link to="/settings" class="side-link-sub" :class="{ active: isActive('/settings') }">
        <van-icon name="setting-o" />
        <span>设置</span>
      </router-link>
      <a class="side-link-sub" @click="onLogout">
        <van-icon name="close" />
        <span>退出</span>
      </a>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../features/auth/store'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const items = [
  { label: '看板', path: '/', icon: 'wap-home-o' },
  { label: '流水', path: '/transactions', icon: 'orders-o' },
  { label: '汇总', path: '/summary', icon: 'chart-trending-o' },
  { label: '往来', path: '/contacts', icon: 'friends-o' },
  { label: '记账', path: '/?record=open', icon: 'edit' },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

async function onLogout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.side-nav {
  width: 220px;
  min-height: 100vh;
  background: #2c2c2a;
  padding: 12px 0 20px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  position: sticky;
  top: 0;
  align-self: flex-start;
  box-sizing: border-box;
}

.side-link,
.side-link-sub {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 24px;
  color: #c9c6be;
  text-decoration: none;
  font-size: 15px;
}

.side-link.active,
.side-link-sub.active {
  color: #fff;
  background: rgba(255, 255, 255, 0.12);
  border-left: 3px solid #0f6e56;
}

.side-foot {
  margin-top: auto;
}
</style>
