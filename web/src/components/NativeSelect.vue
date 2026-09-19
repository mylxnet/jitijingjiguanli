<template>
  <div class="ns-wrap">
    <span v-if="label" class="ns-label">{{ label }}</span>
    <select class="ns-select" :value="modelValue" :disabled="disabled" @change="onChange">
      <option value="" disabled>{{ placeholder || '请选择' }}</option>
      <option v-for="o in options" :key="o.value" :value="o.value">{{ o.text ?? o.name }}</option>
    </select>
  </div>
</template>

<script setup lang="ts">
interface Option {
  text?: string
  name?: string
  value: number | string
}

withDefaults(defineProps<{
  modelValue?: number | string | null
  options?: Option[]
  placeholder?: string
  label?: string
  disabled?: boolean
}>(), {
  modelValue: null,
  options: () => [],
  placeholder: '',
  label: '',
  disabled: false,
})

const emit = defineEmits<{ (e: 'update:modelValue', v: number | string | null): void }>()

function onChange(e: Event) {
  const raw = (e.target as HTMLSelectElement).value
  if (raw === '') {
    emit('update:modelValue', null)
    return
  }
  const num = Number(raw)
  emit('update:modelValue', Number.isNaN(num) ? raw : num)
}
</script>

<style scoped>
/* 默认（手机/窄屏）隐藏；≥992px 桌面才显示原生下拉 */
.ns-wrap {
  display: none;
}

@media (min-width: 992px) {
  .ns-wrap {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
  }

  .ns-label {
    width: 70px;
    font-size: 15px;
    color: var(--ink-muted);
    flex: none;
  }

  .ns-select {
    flex: 1;
    min-width: 0;
    height: 40px;
    border: 1px solid var(--line);
    border-radius: 8px;
    font-size: 15px;
    padding: 0 10px;
    background: #fff;
    color: var(--ink);
  }
}
</style>
