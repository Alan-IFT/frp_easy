<template>
  <n-modal
    :show="show"
    preset="dialog"
    type="warning"
    :title="title"
    :content="content"
    positive-text="确认"
    negative-text="取消"
    @positive-click="handleConfirm"
    @negative-click="handleCancel"
    @update:show="(val: boolean) => emit('update:show', val)"
  />
</template>

<script setup lang="ts">
import { NModal } from 'naive-ui'

defineProps<{
  show: boolean
  title: string
  content: string
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'confirm'): void
  (e: 'cancel'): void
}>()

function handleConfirm() {
  emit('confirm')
  emit('update:show', false)
}

function handleCancel() {
  emit('cancel')
  emit('update:show', false)
}
</script>
