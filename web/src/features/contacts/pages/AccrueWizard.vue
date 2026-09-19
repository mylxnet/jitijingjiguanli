<!--
  年度计提 · 引导式分步向导
  分五步：①土地流转费 → ②投资收益 → ③再投资收益 → ④管理费 → ⑤确认汇总
  每一步从单位计提标准自动带出建议金额，可修改、可跳过，最后汇总一次提交。
-->
<template>
  <div class="aw-page">
    <div class="aw-header">
      <h2 class="aw-title">年度计提</h2>
      <p class="aw-sub">按 {{ year }} 年度引导式结转 · 金额已按单位基本信息自动带出，可修改</p>
    </div>

    <van-steps :active="step" active-color="var(--success)">
      <van-step v-for="s in STEPS" :key="s">{{ s }}</van-step>
    </van-steps>

    <div class="aw-body">
      <van-loading v-if="loading" />

      <!-- 分类步骤：某分类的单位明细 -->
      <template v-else-if="step < groups.length && currentGroup">
        <div class="aw-group-head">
          <span class="aw-group-label">{{ currentGroup.label }}</span>
          <span class="aw-group-count">共 {{ currentGroup.items.length }} 条 · 待计提 {{ pendingOf(currentGroup).length }} 条</span>
        </div>
        <div v-if="currentGroup.items.length === 0" class="aw-empty">该类暂无数据，可直接下一步</div>
        <div v-else class="aw-list">
          <div v-for="(it, i) in currentGroup.items" :key="i" class="aw-item" :class="{ exists: it.exists }">
            <div class="aw-item-left">
              <div class="aw-party">{{ it.partyName }}</div>
              <div class="aw-title">{{ it.title }}</div>
            </div>
            <div class="aw-item-right">
              <template v-if="it.exists">
                <span class="aw-exists">已存在</span>
              </template>
              <template v-else>
                <input
                  class="aw-input"
                  type="text"
                  inputmode="decimal"
                  :value="(it._amountCents / 100).toFixed(2)"
                  @input="onAmountInput(currentGroup.items, i, $event)"
                />
                <span class="aw-unit">元</span>
              </template>
            </div>
          </div>
        </div>
      </template>

      <!-- 第 4 步：确认汇总 -->
      <template v-else>
        <div class="aw-summary">
          <template v-for="g in groups" :key="g.key">
            <div v-if="pendingOf(g).length" class="aw-sum-group">
              <div class="aw-sum-group-head">
                <span class="aw-sum-group-label">{{ g.label }}</span>
                <span class="aw-sum-group-amt">{{ fmt(sumOf(g)) }}</span>
              </div>
              <div v-for="(it, i) in pendingOf(g)" :key="i" class="aw-sum-item">
                <span class="aw-sum-name">{{ it.partyName }}</span>
                <span class="aw-sum-amount">{{ fmt(it._amountCents) }}</span>
              </div>
            </div>
          </template>
          <div v-if="pendingAll.length === 0" class="aw-empty">没有需要计提的条目（均已存在或金额为空）</div>
          <div v-else class="aw-sum-total">
            <span>合计 {{ pendingAll.length }} 条</span>
            <span class="aw-sum-total-amt">{{ fmt(totalOf(pendingAll)) }}</span>
          </div>
        </div>
      </template>
    </div>

    <div class="aw-footer">
      <van-button v-if="step > 0" size="small" @click="step--">上一步</van-button>
      <van-button
        v-if="step < groups.length"
        type="primary" plain size="small" style="margin-left:auto"
        @click="step++">下一步</van-button>
      <van-button
        v-else
        type="primary" size="small" style="margin-left:auto"
        :loading="submitting" :disabled="pendingAll.length === 0"
        @click="submit">确认计提</van-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { showToast } from 'vant'
import { api } from '../../../lib/http'

interface It { kind: string; title: string; partyId: number; partyName: string; amountCents: number; exists: boolean; _amountCents: number }
interface Group { key: string; label: string; items: It[] }

const STEPS = ['土地流转费', '投资收益', '再投资收益', '管理费', '确认汇总']
const year = new Date().getFullYear()

const step = ref(0)
const loading = ref(true)
const submitting = ref(false)

const groups = ref<Group[]>([
  { key: 'rent',               label: '土地流转费', items: [] },
  { key: 'dividend',           label: '投资收益',   items: [] },
  { key: 'reinvest_dividend',  label: '再投资收益', items: [] },
  { key: 'service',            label: '管理费',     items: [] },
])

const currentGroup = computed(() => step.value < groups.value.length ? groups.value[step.value] : null)

async function load() {
  loading.value = true
  try {
    const r = await api.get<any>('/recv-standards/preview?year=' + year)
    const items: It[] = ((r as any)?.data?.items || r?.items || []).map((it: any) => ({ ...it, _amountCents: it.amountCents }))
    groups.value.forEach(g => { g.items = items.filter(it => it.kind === g.key) })
  } catch (e: any) {
    showToast('加载失败：' + (e?.message || '未知错误'))
  } finally { loading.value = false }
}

function onAmountInput(list: It[], i: number, e: Event) {
  const val = parseFloat((e.target as HTMLInputElement).value)
  list[i]._amountCents = !isNaN(val) && val >= 0 ? Math.round(val * 100) : 0
}

function pendingOf(g: Group) {
  return g.items.filter(it => !it.exists && it._amountCents > 0)
}

const pendingAll = computed(() => groups.value.flatMap(g => pendingOf(g)))

function sumOf(g: Group) {
  return g.items.filter(it => !it.exists).reduce((s, it) => s + it._amountCents, 0)
}
function totalOf(list: It[]) { return list.reduce((s, it) => s + it._amountCents, 0) }

const fmt = (c: number) => '¥' + (c / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })

async function submit() {
  const items = pendingAll.value.map(it => ({ partyId: it.partyId, recvKind: it.kind, amountCents: it._amountCents }))
  submitting.value = true
  try {
    const res = await api.post('/receivables/batch', { recvYear: year, title: year + '年度计提', items })
    const created = (res as any)?.data?.created ?? items.length
    showToast('计提完成，共 ' + created + ' 条')
    await load()
  } catch (e: any) {
    showToast('计提失败：' + (e?.message || '未知错误'))
  } finally { submitting.value = false }
}

onMounted(load)
</script>

<style scoped>
.aw-page { display: flex; flex-direction: column; min-height: 100%; }
.aw-header { margin-bottom: 16px; }
.aw-title { font-size: 20px; font-weight: 600; margin: 0 0 4px; }
.aw-sub { font-size: 12px; color: #969799; margin: 0; }
.aw-body { flex: 1; padding: 16px 0; }
.aw-group-head { display: flex; align-items: baseline; gap: 10px; margin-bottom: 10px; }
.aw-group-label { font-size: 15px; font-weight: 600; }
.aw-group-count { font-size: 12px; color: #969799; }
.aw-empty { padding: 48px 0; text-align: center; color: #969799; font-size: 13px; }
.aw-list { display: flex; flex-direction: column; gap: 8px; }
.aw-item { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 10px 12px; border: 1px solid #ebebeb; border-radius: 8px; }
.aw-item.exists { opacity: .55; }
.aw-item-left { min-width: 0; }
.aw-party { font-size: 14px; font-weight: 500; }
.aw-title { font-size: 11px; color: #969799; margin-top: 2px; }
.aw-item-right { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.aw-input { width: 110px; padding: 6px 8px; border: 1px solid #c8c9cc; border-radius: 6px; font-size: 14px; text-align: right; }
.aw-input:focus { border-color: var(--success); }
.aw-unit { font-size: 12px; color: #969799; }
.aw-exists { font-size: 11px; color: #969799; background: #e8e8e8; padding: 2px 8px; border-radius: 4px; }
.aw-summary { display: flex; flex-direction: column; gap: 14px; }
.aw-sum-group { border: 1px solid #ebebeb; border-radius: 8px; padding: 10px 12px; }
.aw-sum-group-head { display: flex; justify-content: space-between; margin-bottom: 8px; }
.aw-sum-group-label { font-size: 14px; font-weight: 600; }
.aw-sum-group-amt { font-size: 14px; color: var(--success); font-weight: 600; }
.aw-sum-item { display: flex; justify-content: space-between; font-size: 13px; color: #3a3a3a; padding: 4px 0; }
.aw-sum-amount { font-variant-numeric: tabular-nums; }
.aw-sum-total { display: flex; justify-content: space-between; font-size: 15px; font-weight: 600; padding: 10px 2px; border-top: 1px dashed #ddd; }
.aw-sum-total-amt { color: var(--success); }
.aw-footer { display: flex; align-items: center; padding: 14px 0 0; border-top: 1px solid #f0f0f0; }
</style>