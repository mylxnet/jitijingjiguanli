<template>
  <div id="app">
    <template v-if="showNav">
      <SideNav class="desktop-only" />
      <main class="app-main">
        <router-view />
      </main>
      <BottomNav class="mobile-only" />
    </template>
    <router-view v-else />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import BottomNav from './components/BottomNav.vue'
import SideNav from './components/SideNav.vue'

const route = useRoute()
const showNav = computed(() => {
  return !['/login', '/register'].includes(route.path)
})
</script>

<style>
html,
body {
  margin: 0;
  padding: 0;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  background: #f7f7f5;
  -webkit-font-smoothing: antialiased;
}

/* 移动端（默认）：主区窄栏居中，底部 tabbar */
#app {
  min-height: 100vh;
  background: #f7f7f5;
}

.app-main {
  min-height: 100vh;
}

.desktop-only {
  display: none !important;
}

/* 宽屏：左侧导航 + 主区自适应 */
@media (min-width: 992px) {
  #app {
    display: flex;
  }

  .desktop-only {
    display: flex !important;
  }

  .mobile-only {
    display: none !important;
  }

  .app-main {
    flex: 1;
    max-width: 1180px;
    margin: 0 auto;
    width: 100%;
  }
}
</style>
