<template>
  <van-tabbar v-model="active" route>
    <van-tabbar-item to="/" icon="plus">记账</van-tabbar-item>
    <van-tabbar-item to="/investment" icon="chart-trending-o">投资</van-tabbar-item>
    <van-tabbar-item to="/flow" icon="home-o">流转</van-tabbar-item>
    <van-tabbar-item to="/contacts" icon="friends-o">往来</van-tabbar-item>
    <van-tabbar-item to="/settings" icon="setting-o">设置</van-tabbar-item>
  </van-tabbar>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const active = ref(0)

// 底部 tab 索引，与上方 van-tabbar-item 顺序一一对应
const pathMap: Record<string, number> = {
  '/': 0,
  '/investment': 1,
  '/flow': 2,
  '/contacts': 3,
  '/settings': 4,
  '/categories': 4, // 科目管理是设置页的二级页，保持「设置」tab 高亮
}

// 把当前路由归到对应 tab（二级页也要保持父级 tab 高亮）
function resolveActive(path: string): number {
  if (pathMap[path] !== undefined) return pathMap[path]
  if (path.startsWith('/investment')) return 1
  if (path.startsWith('/flow')) return 2
  if (path.startsWith('/contacts')) return 3
  if (path.startsWith('/settings') || path.startsWith('/categories')) return 4
  return 0
}

watch(() => route.path, (p) => { active.value = resolveActive(p) }, { immediate: true })
</script>
