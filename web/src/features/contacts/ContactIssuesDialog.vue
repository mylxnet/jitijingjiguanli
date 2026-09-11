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
      <div class="ci-head">数据缺失（{{ issueCount }} 个单位）</div>
      <div v-if="issueCount === 0" class="ci-empty">各单位数据齐全</div>
      <template v-else>
        <div class="ci-list">
          <div v-for="it in issues" :key="it.id" class="ci-row" @click="goEdit(it.id)">
            <div class="ci-main">
              <span class="ci-name">{{ it.name }}</span>
              <span class="ci-tag">{{ it.typeLabel }}</span>
            </div>
            <div class="ci-miss">缺：{{ it.missing.join('、') }}</div>
          </div>
        </div>
        <div class="ci-tip">点击某一行可直接去补数据</div>
      </template>
    </div>
  </van-popup>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { popupPos } from '../../composables/useScreen'
import { useContactIssues } from './useContactIssues'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const router = useRouter()
const { issues, issueCount } = useContactIssues()

const visible = computed({
  get: () => props.show,
  set: (v: boolean) => emit('update:show', v),
})

// 跳到单位列表并打开该单位的编辑弹窗（定位到「年度数据」页补数据）
function goEdit(id: number) {
  visible.value = false
  router.push({ path: '/contacts/parties', query: { editParty: String(id) } })
}
</script>

<style scoped>
.ci-popup {
  padding: 16px 14px 18px;
  min-width: 320px;
}
.ci-head {
  font-size: 15px;
  font-weight: 600;
  color: #1f2329;
  margin-bottom: 10px;
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
