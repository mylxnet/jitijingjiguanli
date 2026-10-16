import type { Directive } from 'vue'
import { useAuthStore } from '../features/auth/store'

/**
 * v-write —— 写操作入口的角色收口（见 docs/27 §6.4）。
 *
 * 用法：给「新增 / 编辑 / 删除 / 作废 / 上缴 / 上传 / 保存」等写操作元素加 `v-write`，
 * 只读账号（viewer，以及角色尚未取到时的首帧）会自动隐藏该元素。
 *
 * 注意：前端隐藏仅为体验收口，写权限的唯一权威在后端
 * （RequireWrite / RequireRole 会对 viewer 的写请求统一 403）。
 */
export const writeDirective: Directive<HTMLElement> = {
  mounted(el) {
    sync(el)
  },
  updated(el) {
    sync(el)
  },
}

function sync(el: HTMLElement) {
  const auth = useAuthStore()
  // 用 display 隐藏而非移除 DOM：角色到账后（登录/刷新完成）可自动恢复，避免破坏 Vue 的补丁过程
  el.style.display = auth.isWritable ? '' : 'none'
}
