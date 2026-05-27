<template>
  <div class="allow-ports-editor">
    <n-alert type="info" :show-icon="false" style="margin-bottom: 12px">
      留空 = 允许所有端口；配置后只允许列出范围被 frpc 申请。改动需要 frps 重启生效（自动）。
    </n-alert>
    <n-space vertical :size="8">
      <div v-for="(r, i) in rows" :key="r._id" class="ape-row">
        <n-tag :type="r.kind === 'single' ? 'info' : 'success'" size="small" style="min-width: 56px">
          {{ r.kind === 'single' ? '单端口' : '范围' }}
        </n-tag>
        <template v-if="r.kind === 'single'">
          <n-input-number
            v-model:value="r.single"
            :min="1"
            :max="65535"
            placeholder="端口"
            style="width: 140px"
          />
        </template>
        <template v-else>
          <n-input-number
            v-model:value="r.start"
            :min="1"
            :max="65535"
            placeholder="起始"
            style="width: 120px"
          />
          <span style="padding: 0 4px">-</span>
          <n-input-number
            v-model:value="r.end"
            :min="1"
            :max="65535"
            placeholder="结束"
            style="width: 120px"
          />
        </template>
        <n-button size="small" tertiary type="error" @click="removeAt(i)">删除</n-button>
        <n-text
          v-if="rowErrors[i]"
          type="error"
          style="margin-left: 8px; font-size: 12px"
          class="ape-error"
        >
          {{ rowErrors[i] }}
        </n-text>
      </div>
      <n-space>
        <n-button size="small" @click="addRange">添加范围</n-button>
        <n-button size="small" @click="addSingle">添加单端口</n-button>
        <n-text v-if="rows.length === 0" depth="3" style="font-size: 12px; line-height: 28px">
          （暂无策略 = 允许所有端口）
        </n-text>
      </n-space>
    </n-space>
  </div>
</template>

<script setup lang="ts">
/**
 * T-040 · AllowPortsEditor —— frps allowPorts 端口策略编辑器
 *
 * 设计要点（继承 insight L13 / T-032 范式）：
 * - 单向数据流：props.initial 在 setup 时读一次种子，后续父级不再下推
 * - 不 emit、不用 v-model 桥（避免 v-model + composable 新对象 OOM 反馈环）
 * - 父级保存时调 defineExpose 的 getAllowPortsInput() 拉当前值
 * - hasValidationError() 让父级保存按钮在前端校验未过时拒发 PUT
 *
 * 校验逻辑严格镜像后端 frpconf.ValidateFrpsAllowPorts：
 * - 单端口 / 范围互斥（通过两按钮分流，UI 不可能同时填）
 * - 每端口 ∈ [1, 65535]
 * - start ≤ end
 * - 闭区间重叠检测（[1000,2000] 与 [2000,3000] 算重叠，2000 同属两段）
 */
import { ref, computed } from 'vue'
import {
  NAlert, NSpace, NTag, NInputNumber, NButton, NText,
} from 'naive-ui'
import type { AllowPortRange } from '../types'

interface Props {
  initial: AllowPortRange[]
}
const props = defineProps<Props>()

/**
 * 内部 Row：kind 字段决定 UI 形态 + 校验分支。
 * _id 让 v-for :key 稳定（避免 splice 删除中间行后 input 焦点错位）。
 */
interface Row {
  _id: number
  kind: 'range' | 'single'
  start: number | null
  end: number | null
  single: number | null
}

let nextId = 0
function newRow(kind: 'range' | 'single', start: number | null = null, end: number | null = null, single: number | null = null): Row {
  return { _id: ++nextId, kind, start, end, single }
}

// setup 时读一次 initial（单向数据流；之后不再 watch props.initial）
const rows = ref<Row[]>(
  (props.initial ?? []).map(r => {
    if (typeof r.single === 'number' && r.single > 0) {
      return newRow('single', null, null, r.single)
    }
    return newRow('range', r.start ?? null, r.end ?? null, null)
  }),
)

function addRange(): void {
  rows.value.push(newRow('range'))
}
function addSingle(): void {
  rows.value.push(newRow('single'))
}
function removeAt(i: number): void {
  rows.value.splice(i, 1)
}

/**
 * 每行的错误文案（null = OK）。
 * 与后端 ValidateFrpsAllowPorts 同步：互斥已被两按钮分流保证；这里只校验端口范围 + start≤end + 重叠。
 *
 * 重叠检测：i 行只与 j < i 的行比对，避免同一对错误显示两次（C-2 消化）。
 * 用户视角：第 i 行的"与第 j 行重叠"是首次发生位置；i < j 时 j 行已经能看到。
 */
const rowErrors = computed<(string | null)[]>(() => {
  const list = rows.value
  return list.map((r, i) => validateRow(r, i, list))
})

function validateRow(r: Row, idx: number, all: Row[]): string | null {
  if (r.kind === 'single') {
    if (r.single === null) return '请填写端口'
    if (r.single < 1 || r.single > 65535) return '端口范围 1-65535'
  } else {
    if (r.start === null || r.end === null) return '请填写起止端口'
    if (r.start < 1 || r.start > 65535) return '起始端口 1-65535'
    if (r.end < 1 || r.end > 65535) return '结束端口 1-65535'
    if (r.start > r.end) return '起始端口必须 ≤ 结束端口'
  }
  // overlap：只与索引更小的行比对
  const myLo = r.kind === 'single' ? r.single! : r.start!
  const myHi = r.kind === 'single' ? r.single! : r.end!
  for (let j = 0; j < idx; j++) {
    const o = all[j]
    if (rowHasInputErr(o)) continue
    const oLo = o.kind === 'single' ? o.single! : o.start!
    const oHi = o.kind === 'single' ? o.single! : o.end!
    if (myLo <= oHi && oLo <= myHi) {
      return `与第 ${j + 1} 行区间重叠`
    }
  }
  return null
}

function rowHasInputErr(r: Row): boolean {
  if (r.kind === 'single') return r.single === null || r.single < 1 || r.single > 65535
  return r.start === null || r.end === null
    || r.start < 1 || r.start > 65535
    || r.end < 1 || r.end > 65535
    || r.start > r.end
}

const hasError = computed<boolean>(() => rowErrors.value.some(e => e !== null))

defineExpose({
  /**
   * 父级保存时调用：返回当前 list 的 AllowPortRange[] 形态（按用户顺序）。
   * Single != 0 的行 → {single: N}；其余 → {start, end}。
   * 注意：本方法不再校验；父级必须先调 hasValidationError() 决策是否走 PUT。
   */
  getAllowPortsInput(): AllowPortRange[] {
    return rows.value.map<AllowPortRange>(r => {
      if (r.kind === 'single') {
        return { single: r.single ?? 0 }
      }
      return { start: r.start ?? 0, end: r.end ?? 0 }
    })
  },
  hasValidationError(): boolean {
    return hasError.value
  },
})
</script>

<style scoped>
.ape-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
