<template>
  <div class="investment-layout">
    <nav class="il-tabs">
      <button
        v-for="t in tabs" :key="t.key"
        class="il-tab"
        :class="{ active: activeTab === t.key }"
        @click="activeTab = t.key"
      >{{ t.icon }} {{ t.label }}</button>
    </nav>
    <div class="il-content">
      <InvestmentsPage :active-tab="activeTab" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import InvestmentsPage from './pages/InvestmentsPage.vue'

const tabs = [
  { key: 'longterm',  icon: '🏦', label: '长期投资' },
  { key: 'reinvest',  icon: '🔄', label: '再投资' },
  { key: 'returns',   icon: '💹', label: '投资收益' },
  { key: 'dist532',   icon: '📊', label: '532分配' },
]
const activeTab = ref('longterm')
</script>

<style scoped>
.investment-layout {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}
.il-tabs {
  display: flex;
  gap: 2px;
  padding: 10px 16px;
  background: #fafbfc;
  border-bottom: 1px solid var(--line-soft, #eaeaea);
  overflow-x: auto;
}
.il-tab {
  padding: 8px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--ink-500, #646566);
  border: none;
  background: transparent;
  cursor: pointer;
  white-space: nowrap;
  transition: all .15s;
}
.il-tab:hover { color: var(--ink-900, #1f2329); }
.il-tab.active {
  background: var(--jade, #07c160);
  color: #fff;
  font-weight: 600;
}
.il-content {
  flex: 1;
  padding: 16px 20px 40px;
  overflow-y: auto;
}
@media (max-width: 800px) {
  .il-tabs { padding: 8px 10px; gap: 0; }
  .il-tab { padding: 6px 10px; font-size: 12px; }
  .il-content { padding: 12px 10px 32px; }
}
</style>