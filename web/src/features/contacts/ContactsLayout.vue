<template>
  <div class="contacts-layout">
    <!-- Sub-tabs inside main white card -->
    <nav class="cl-tabs">
      <router-link to="/contacts/overview"           class="cl-tab">📊 概览</router-link>
      <router-link to="/contacts/parties"            class="cl-tab">🏢 单位列表</router-link>
      <router-link to="/contacts/receivables/dividend" class="cl-tab">💹 应收投资收益</router-link>
      <router-link to="/contacts/receivables/rent"     class="cl-tab">🌾 应收流转费</router-link>
      <router-link to="/contacts/receivables/service"  class="cl-tab">📋 应收管理费</router-link>
      <router-link to="/contacts/contracts"          class="cl-tab">📄 合同管理</router-link>
      <router-link to="/contacts/accrue"             class="cl-tab">🧭 年度计提</router-link>
    </nav>
    <div class="cl-content">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
// thin shell — just sub-tabs + outlet
</script>

<style scoped>
.contacts-layout {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}
.cl-tabs {
  display: flex;
  gap: 2px;
  padding: 10px 16px;
  background: #fafbfc;
  border-bottom: 1px solid var(--line-soft, #eaeaea);
  overflow-x: auto;
}
.cl-tab {
  padding: 8px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--ink-500, #646566);
  text-decoration: none;
  white-space: nowrap;
  transition: all .15s;
}
.cl-tab:hover { color: var(--ink-900, #1f2329); }
.cl-tab.router-link-active,
.cl-tab.router-link-exact-active {
  background: var(--jade, #07c160);
  color: #fff;
  font-weight: 600;
}
.cl-content {
  flex: 1;
  padding: 16px 20px 40px;
  overflow-y: auto;
}
@media (max-width: 800px) {
  .cl-tabs { padding: 8px 10px; gap: 0; }
  .cl-tab { padding: 6px 10px; font-size: 12px; }
  .cl-content { padding: 12px 10px 32px; }
}
</style>
