<template>
  <div id="app" :class="{ 'app-shell': showNav }">
    <!-- 登录/注册：无导航，居中 -->
    <template v-if="!showNav">
      <div class="auth-wrap">
        <router-view />
      </div>
    </template>
    <!-- 登录后：SideNav(桌面) + 主区(卡片wrap + router-view) + BottomNav(移动) -->
    <template v-else>
      <!-- 左侧 SideNav，仅桌面显示 -->
      <SideNav class="sidebar-desktop" />

      <!-- 中间主区，桌面时用白色卡片包裹 -->
      <main class="app-main-wrap">
        <div class="app-card-inner">
          <router-view />
        </div>
      </main>

      <!-- 底部 BottomNav，仅移动显示 -->
      <BottomNav class="bottomnav-mobile" />
    </template>

    <!-- 全局记账弹窗 -->
    <RecordPopup ref="recordPopupRef" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, provide } from 'vue'
import { useRoute } from 'vue-router'
import BottomNav from './components/BottomNav.vue'
import SideNav from './components/SideNav.vue'
import RecordPopup from './features/transaction/RecordPopup.vue'

const route = useRoute()
const showNav = computed(() => !['/login', '/register', '/onboarding'].includes(route.path))

const recordPopupRef = ref<InstanceType<typeof RecordPopup> | null>(null)
provide('openRecord', (opts?: { biz?: string; partyId?: number; amount?: string }) => {
  recordPopupRef.value?.openRecord(opts)
})
</script>

<style>
html, body {
  margin: 0;
  padding: 0;
}

body {
  font-family: 'Noto Sans SC', -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Microsoft YaHei', sans-serif;
  background: var(--paper);
  -webkit-font-smoothing: antialiased;
}

#app {
  min-height: 100vh;
  background: var(--paper);
}

.app-shell {
  display: flex;
  min-height: 100vh;
  background: var(--paper);
}

/* ========== 默认（移动端） ========== */

/* SideNav 隐藏 */
.sidebar-desktop {
  display: none !important;
}

/* BottomNav 显示 */
.bottomnav-mobile {
  display: block;
}

/* 主区：全宽，无卡片包裹 */
.app-main-wrap {
  flex: 1;
  min-width: 0;
  min-height: 100vh;
  background: var(--paper);
}

.app-card-inner {
  min-height: 100vh;
  padding-bottom: 64px; /* 给 BottomNav 留空间 */
  background: var(--paper);
}

/* ========== 桌面端 (≥800px) ========== */
@media (min-width: 800px) {
  /* SideNav 显示 */
  .sidebar-desktop {
    display: flex !important;
  }

  /* BottomNav 隐藏 */
  .bottomnav-mobile,
  .van-tabbar {
    display: none !important;
  }

  /* 主区 padding + 白色卡片 */
  .app-main-wrap {
    padding: 14px;
    box-sizing: border-box;
  }

  .app-card-inner {
    min-height: calc(100vh - 28px);
    background: #fff;
    border-radius: var(--r-xl);
    box-shadow: var(--shadow-lg);
    border: 1px solid var(--line-soft);
    overflow: hidden;
    padding-bottom: 0;
  }
}

/* ========== 桌面端弹窗禁止过渡动画（避免打开瞬间位移动画） ========== */
@media (min-width: 800px) {
  /* 禁用 Vant 过渡动画类 */
  .van-fade-enter-active,
  .van-fade-leave-active {
    animation: none !important;
  }
  .van-popup-slide-bottom-enter-active,
  .van-popup-slide-bottom-leave-active {
    animation: none !important;
  }
  /* 强制弹窗居中定位 */
  .van-popup--center {
    top: 50% !important;
    left: 50% !important;
    transform: translate(-50%, -50%) !important;
    animation: none !important;
  }
  .van-overlay {
    animation: none !important;
  }
}

/* ========== 登录/注册 ========== */
.auth-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  box-sizing: border-box;
}

.auth-wrap .login-page,
.auth-wrap .register-page {
  width: 100%;
  max-width: 400px;
}
</style>
