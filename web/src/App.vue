<template>
  <div id="app" :class="{ 'app-shell': showNav }">
    <template v-if="showNav">
      <SideNav class="desktop-only" />
      <main class="app-main">
        <router-view />
      </main>
      <BottomNav class="mobile-only" />
    </template>
    <div v-else class="auth-wrap">
      <router-view />
    </div>
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

/* 登录/注册等无导航页：整体居中并占满可用区 */
.auth-wrap {
  min-height: 100vh;
  flex: 1;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.auth-wrap .login-page,
.auth-wrap .register-page {
  min-height: auto;
  width: 100%;
}

.desktop-only {
  display: none !important;
}

/* 只在手机/窄屏显示（桌面被原生下拉替代） */
.only-mobile {
  display: block;
}

@media (min-width: 992px) {
  .only-mobile {
    display: none !important;
  }
}

/* 宽屏：仅带导航的主页面启用左右布局（登录/注册保持普通块级全宽居中） */
@media (min-width: 992px) {
  .app-shell {
    display: flex;
  }

  .desktop-only {
    display: flex !important;
  }

  .mobile-only {
    display: none !important;
  }

  .app-shell .app-main {
    flex: 1;
    min-width: 0;
    width: auto;
    padding: 0 16px;
    box-sizing: border-box;
  }

  /* 桌面端去“手机感”：按钮/单元格/字号整体放大 */
  .van-button--mini {
    height: 36px;
    padding: 0 16px;
    font-size: 14px;
  }

  .van-button--small {
    height: 42px;
    padding: 0 22px;
    font-size: 15px;
  }

  .van-cell {
    padding: 14px 16px;
  }

  .van-cell__title,
  .van-cell__label {
    font-size: 15px;
  }

  .van-field__control,
  .van-cell__value {
    font-size: 15px;
  }

  .van-cell__right-icon {
    font-size: 16px;
  }

  .van-popup .popup-title,
  .van-dialog__header {
    font-size: 18px;
  }

  .van-picker-column__item {
    font-size: 16px;
  }

  /* 桌面端：底部弹层全部改居中弹窗，避免手机式贴底 */
  .van-popup--bottom {
    left: 50% !important;
    top: 50% !important;
    right: auto !important;
    bottom: auto !important;
    transform: translate(-50%, -50%) !important;
    width: min(680px, 92vw) !important;
    max-height: 85vh !important;
    border-radius: 12px !important;
  }

  .van-action-sheet {
    width: min(560px, 92vw) !important;
    left: 50% !important;
    right: auto !important;
    transform: translateX(-50%) !important;
  }
}
</style>
