<template>
  <div class="flow-layout">
    <nav class="fl-tabs">
      <button
        v-for="t in tabs" :key="t.key"
        class="fl-tab"
        :class="{ active: activeTab === t.key }"
        @click="activeTab = t.key"
      >{{ t.icon }} {{ t.label }}</button>
    </nav>
    <div class="fl-content">
      <FlowPage :active-tab="activeTab" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import FlowPage from './pages/FlowPage.vue'

const tabs = [
  { key: 'rent',    icon: '🏠', label: '土地流转费收入' },
  { key: 'service', icon: '📋', label: '流转管理费' },
]
const activeTab = ref('rent')
</script>

<style scoped>
.flow-layout {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}
.fl-tabs {
  display: flex;
  gap: 2px;
  padding: 10px 16px;
  background: #fafbfc;
  border-bottom: 1px solid var(--line-soft);
  overflow-x: auto;
}
.fl-tab {
  padding: 8px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--ink-500);
  border: none;
  background: transparent;
  cursor: pointer;
  white-space: nowrap;
  transition: all .15s;
}
.fl-tab:hover { color: var(--ink-900); }
.fl-tab.active {
  background: var(--jade, #2b5876);
  color: #fff;
  font-weight: 600;
}
.fl-content {
  flex: 1;
  padding: 16px 20px 40px;
  overflow-y: auto;
}
@media (max-width: 800px) {
  .fl-tabs { padding: 8px 10px; gap: 0; }
  .fl-tab { padding: 6px 10px; font-size: 12px; }
  .fl-content { padding: 12px 10px 32px; }
}
</style>