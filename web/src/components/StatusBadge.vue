<template>
  <n-tag :type="tagType" :bordered="false" size="small">
    {{ stateLabel }}
  </n-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NTag } from 'naive-ui'
import type { ProcessState } from '../types'
import { getTagType, getStateLabel } from '../composables/statusUtils'

const props = defineProps<{
  state: ProcessState
}>()

const tagType = computed(() => getTagType(props.state))
const stateLabel = computed(() => getStateLabel(props.state))
</script>
