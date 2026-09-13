<template>
  <van-popup
    v-model:show="visible"
    :position="popupPos()"
    round
    closeable
    teleport="body"
    style="max-height: 70vh"
  >
    <div class="ci-popup">
      <template v-if="issueCount === 0">
        <div class="ci-empty">各单位数据齐全，无待处理事项</div>
      </template>
      <template v-else>
        <div v-if="issues.length">
          <div class="ci-section-title">数据缺失（{{ issues.length }} 个单位）</div>
          <div class="ci-list">
            <div v-for="it in issues" :key="'m' + it.id" class="ci-row" @click="goEdit(it.id)">
              <div class="ci-main">
                <span class="ci-name">{{ it.name }}</span>
                <span class="ci-tag">{{ it.typeLabel }}</span>
              </div>
              <div class="ci-miss">缺：{{ it.missing.join('、') }}</div>
            </div>
          </div>
        </div>

        <div v-if="expiring.length">
          <div class="ci-section-title" :class="{ 'ci-section-gap': issues.length > 0 }">合同到期（{{ expiring.length }} 条）</div>
          <div class="ci-list">
            <div v-for="e in expiring" :key="'e' + e.contractId" class="ci-row" @click="goExpiring()">
              <div class="ci-main">
                <span class="ci-name">{{ e.partyName }}</span>
                <span class="ci-tag">{{ typeLabelOf(e.type) }}</span>
              </div>
              <div class="ci-miss">{{ e.hasExpired ? '已到期' : '即将到期（剩' + e.daysUntil + '天）' }}</div>
              <div class="ci-sub">{{ contractName(e) }} · 到期 {{ e.expiresAt }}</div>
            </div>
          </div>
        </div>
        <div class="ci-tip">点击单元行前往处理</div>
      </template>
    </div>
  </van-popup>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { popupPos } from '../../composables/useScreen'
import type { ExpiringContract } from '../../types/api'
import { TYPE_LABEL, useContactIssues } from './useContactIssues'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const router = useRouter()
const { issues, expiring, issueCount } = useContactIssues()

const visible = computed({
  get: () => props.show,
  set: (v: boolean) => emit('update:show', v),
})

function typeLabelOf(t: string): string {
  return TYPE_LABEL[t] || '其他'
}
function contractName(e: ExpiringContract): string {
  return e.contractTitle || e.fileName
}

// 跳到单位列表并打开该单位的编辑弹窗（定位到「年度数据」页补数据）
function goEdit(id: number) {
  visible.value = false
  router.push({ path: '/contacts/parties', query: { editParty: String(id) } })
}

// 跳到合同管理页（统一在「合同管理」查看/处理到期合同）
function goExpiring() {
  visible.value = false
  router.push({ path: '/contacts/contracts' })
}
</script>

<style scoped>
.ci-popup {
  padding: 16px 14px 18px;
  min-width: 320px;
}
.ci-section-title {
  font-size: 12px;
  font-weight: 600;
  color: #646566;
  margin: 6px 0 4px;
}
.ci-section-gap {
  margin-top: 12px;
}
.ci-sub {
  margin-top: 3px;
  font-size: 11px;
  color: #646566;
}
.ci-empty {
  padding: 32px 0;
  text-align: center;
  color: #969799;
  font-size: 13px;
}
.ci-list {
  max-height: 48vh;
  overflow-y: auto;
}
.ci-row {
  padding: 9px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background .12s;
}
.ci-row:hover {
  background: #f7f8fa;
}
.ci-row + .ci-row {
  border-top: 1px solid #f2f3f5;
}
.ci-main {
  display: flex;
  align-items: center;
  gap: 6px;
}
.ci-name {
  font-size: 13px;
  font-weight: 600;
  color: #1f2329;
}
.ci-tag {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 8px;
  background: #f2f3f5;
  color: #646566;
  white-space: nowrap;
}
.ci-miss {
  margin-top: 3px;
  font-size: 12px;
  color: #ee0a24;
}
.ci-tip {
  margin-top: 10px;
  text-align: center;
  font-size: 11px;
  color: #969799;
}
</style>
