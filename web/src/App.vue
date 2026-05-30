<template>
  <n-config-provider :theme="activeTheme">
    <!-- T-066：NGlobalStyle 让 <body> 背景/前景随主题切换（修 Login/Setup/Wizard
         整页硬编码背景的最干净办法）。须在 config-provider 子树内才能读主题 themeVars。 -->
    <n-global-style />
    <n-message-provider>
      <router-view />
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
// T-066 / dark-theme-support · 02 §2 / §6
// App.vue 在 setup 顶层调 useTheme() —— 这是 useOsTheme() 的首次（合规）调用点，
// 惰性建立 osThemeRef（useOsTheme 须在组件 setup 内）。activeTheme 绑到
// <n-config-provider :theme>：null=浅色 / darkTheme=暗色，随偏好/OS 响应式切换。
import { NConfigProvider, NMessageProvider, NGlobalStyle } from 'naive-ui'
import { useTheme } from './composables/useTheme'

const { activeTheme } = useTheme()
</script>
