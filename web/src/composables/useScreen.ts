import { ref, onBeforeUnmount } from 'vue'

/**
 * 桌面端断点：≥800px 视为桌面（显示 SideNav + 居中弹窗）。
 * 响应式 isDesktop 可直接在 template 里用：
 *   <van-popup :position="popupPos()" ...>
 */
const BP = 800
const width = ref(typeof window !== 'undefined' ? window.innerWidth : 1024)
const isDesktop = ref(width.value >= BP)

// 模块首次 import 时就注册一次 window resize 监听（全局共享）
if (typeof window !== 'undefined') {
  window.addEventListener('resize', () => {
    width.value = window.innerWidth
    isDesktop.value = width.value >= BP
  })
}

/** 让组件拿到响应式状态 */
export function useScreen() {
  return { width, isDesktop, BP }
}

/** 桌面端居中弹窗的 position 绑定 */
export function popupPos() {
  return isDesktop.value ? 'center' : 'bottom'
}
