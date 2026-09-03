<template>
  <div class="home-page">
    <!-- 顶部信息 -->
    <div class="home-header">
      <div class="today-info">
        <span class="today-date">{{ todayStr() }}</span>
        <span class="today-count" v-if="todayCount > 0">
          今日 {{ todayCount }} 笔
        </span>
      </div>
    </div>

    <!-- 无科目引导 -->
    <div v-if="noCategories" class="empty-guide">
      <div class="empty-icon">📋</div>
      <p>还没有科目，先去创建</p>
      <van-button type="primary" size="small" @click="goCategories">
        去创建科目
      </van-button>
    </div>

    <!-- 记账表单 -->
    <div v-else class="form-card">
      <!-- 收/支切换 -->
      <div class="direction-toggle">
        <van-button
          :type="direction === 'income' ? 'primary' : 'default'"
          size="small"
          @click="direction = 'income'"
        >收入</van-button>
        <van-button
          :type="direction === 'expense' ? 'primary' : 'default'"
          size="small"
          @click="direction = 'expense'"
        >支出</van-button>
      </div>

      <van-form @submit="handleSave">
        <!-- 日期 -->
        <van-field
          v-model="form.date"
          label="日期"
          placeholder="YYYY-MM-DD"
          :rules="[{ required: true, message: '请填写日期' }]"
        />

        <!-- 摘要 -->
        <van-field
          v-model="form.note"
          label="摘要"
          placeholder="买了什么、给了谁（可选）"
        />

        <!-- 科目选择 -->
        <van-field
          v-model="form.categoryName"
          is-link
          readonly
          label="科目"
          placeholder="请选择科目"
          :rules="[{ required: true, message: '请选择科目' }]"
          @click="showCategoryPicker = true"
        />
        <van-popup v-model:show="showCategoryPicker" position="bottom">
          <van-picker
            :columns="categoryOptions"
            @confirm="onCategoryConfirm"
            @cancel="showCategoryPicker = false"
          />
        </van-popup>

        <!-- 金额 -->
        <van-field
          v-model="form.amount"
          label="金额"
          type="number"
          placeholder="0.00"
          :rules="[
            { required: true, message: '请填写金额' },
            { validator: validateAmount, message: '金额必须大于 0' }
          ]"
        />

        <div style="margin: 16px">
          <van-button round block type="primary" native-type="submit" :loading="saving">
            保存
          </van-button>
        </div>
      </van-form>

      <div v-if="saveError" class="save-error">{{ saveError }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../lib/http'
import { todayStr, formatFen } from '../../types/api'
import { showToast } from 'vant'
import type { Category, ApiResponse } from '../../types/api'

const router = useRouter()
const saving = ref(false)
const saveError = ref('')
const noCategories = ref(false)
const showCategoryPicker = ref(false)
const direction = ref<'income' | 'expense'>('expense')
const todayCount = ref(0)

const categories = ref<Category[]>([])
const selectedCategoryId = ref<number | null>(null)

const form = ref({
  date: todayStr(),
  note: '',
  categoryName: '',
  amount: '',
})

const categoryOptions = ref<{ text: string; value: number }[]>([])

onMounted(async () => {
  await loadCategories()
})

async function loadCategories() {
  try {
    const res = await api.get<ApiResponse<Category[]>>('/categories')
    const cats = res.data
    // 展平为二级科目选择列表（仅普通科目；资产科目走资金划转，见 D10/R12）
    const options: { text: string; value: number }[] = []
    let hasActiveLevel2 = false
    for (const l1 of cats) {
      if (l1.children) {
        for (const l2 of l1.children) {
          if (l2.status === 'active' && l2.kind === 'normal') {
            options.push({
              text: `${l1.name} / ${l2.name}`,
              value: l2.id,
            })
            hasActiveLevel2 = true
          }
        }
      }
    }
    categoryOptions.value = options
    categories.value = cats
    if (!hasActiveLevel2) {
      noCategories.value = true
    }
  } catch {
    noCategories.value = true
  }
}

function onCategoryConfirm({ selectedOptions }: any) {
  const opt = selectedOptions[0]
  if (opt) {
    form.value.categoryName = opt.text
    selectedCategoryId.value = opt.value
  }
  showCategoryPicker.value = false
}

function validateAmount(val: string): boolean {
  const num = parseFloat(val)
  return !isNaN(num) && num > 0
}

async function handleSave() {
  if (!selectedCategoryId.value) {
    showToast('请选择科目')
    return
  }

  const amountFen = Math.round(parseFloat(form.value.amount) * 100)
  if (amountFen <= 0) {
    showToast('金额必须大于 0')
    return
  }

  saving.value = true
  saveError.value = ''
  try {
    await api.post('/transactions', {
      txnDate: form.value.date,
      direction: direction.value,
      amountCents: amountFen,
      categoryId: selectedCategoryId.value,
      note: form.value.note || '',
    })
    showToast('保存成功')
    // 重置表单
    form.value.note = ''
    form.value.categoryName = ''
    form.value.amount = ''
    selectedCategoryId.value = null
    todayCount.value++
  } catch (e: any) {
    saveError.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
}

function goCategories() {
  router.push('/categories')
}
</script>

<style scoped>
.home-page {
  padding: 16px;
  padding-bottom: 60px;
  min-height: 100vh;
  background: #f7f7f5;
}

.home-header {
  margin-bottom: 16px;
}

.today-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 14px;
  color: #5f5e5a;
}

.today-date {
  font-weight: 500;
}

.today-count {
  font-size: 12px;
  color: #8f8e88;
  background: #eceae4;
  padding: 2px 8px;
  border-radius: 99px;
}

.form-card {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}

.direction-toggle {
  display: flex;
  gap: 8px;
  padding: 16px 16px 0;
}

.empty-guide {
  text-align: center;
  padding: 60px 20px;
  background: #fff;
  border-radius: 12px;
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
}

.empty-guide p {
  color: #8f8e88;
  margin-bottom: 16px;
  font-size: 14px;
}

.save-error {
  color: #a32d2d;
  font-size: 13px;
  text-align: center;
  padding: 0 16px 16px;
}
</style>