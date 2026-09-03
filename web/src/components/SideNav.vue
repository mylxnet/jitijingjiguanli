<template>
  <nav class="side-nav">
    <div class="side-brand">集体台账</div>
    <router-link v-for="item in items" :key="item.path" :to="item.path" class="side-link" :class="{ active: isActive(item.path) }">
      <van-icon :name="item.icon" />
      <span>{{ item.label }}</span>
    </router-link>
    <div class="side-foot">
      <router-link to="/settings" class="side-link-sub" :class="{ active: isActive('/settings') }">设置</router-link>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'

const route = useRoute()

const items = [
  { label: '看板', path: '/', icon: 'wap-home-o' },
  { label: '流水', path: '/transactions', icon: 'orders-o' },
  { label: '汇总', path: '/summary', icon: 'chart-trending-o' },
  { label: '往来', path: '/contacts', icon: 'friends-o' },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<style scoped>
.side-nav {
  width: 220px;
  min-height: 100vh;
  background: #2c2c2a;
  padding: 20px 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  position: sticky;
  top: 0;
  align-self: flex-start;
  box-sizing: border-box;
}

.side-brand {
  color: #fff;
  font-size: 18px;
  font-weight: 600;
  padding: 8px 24px 20px;
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
