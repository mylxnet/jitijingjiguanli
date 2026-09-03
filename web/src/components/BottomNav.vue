<template>
  <van-tabbar v-model="active" route>
    <van-tabbar-item to="/" icon="plus">记账</van-tabbar-item>
    <van-tabbar-item to="/transactions" icon="orders-o">流水</van-tabbar-item>
    <van-tabbar-item to="/summary" icon="chart-trending-o">汇总</van-tabbar-item>
    <van-tabbar-item to="/settings" icon="setting-o">设置</van-tabbar-item>
  </van-tabbar>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const active = ref(0)

const pathMap: Record<string, number> = {
  '/': 0,
  '/transactions': 1,
  '/summary': 2,
  '/settings': 3,
  '/categories': 3, // 科目管理是设置页的二级页，保持「设置」tab 高亮
}

active.value = pathMap[route.path] ?? 0
</script>