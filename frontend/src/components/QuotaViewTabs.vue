<script setup lang="ts">
import type { QuotaViewMode } from '@/types'

defineProps<{
  modelValue: QuotaViewMode
  items: Array<{ key: QuotaViewMode; label: string; caption: string }>
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: QuotaViewMode): void
}>()
</script>

<template>
  <section class="panel quota-workspace-tabs">
    <button
      v-for="item in items"
      :key="item.key"
      type="button"
      class="quota-workspace-tabs__item"
      :class="{ 'quota-workspace-tabs__item--active': item.key === modelValue }"
      @click="emit('update:modelValue', item.key)"
    >
      <strong>{{ item.label }}</strong>
      <span>{{ item.caption }}</span>
    </button>
  </section>
</template>
