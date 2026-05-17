# 05 Code Review — T-006 e2e-smoke-tests

**审核日期**：2026-05-17  
**审核人**：Code Reviewer (Claude)

---

## 审核结论

**APPROVED WITH NOTES**

主要问题（均已修复）：

### CRITICAL — App.vue 缺失 NMessageProvider

**文件**：`web/src/App.vue`  
**问题**：所有页面组件（Setup.vue、Login.vue、AppLayout.vue 等 9 个）均调用 `useMessage()`，但 App.vue 根组件缺少 `<n-message-provider />`。在 Playwright headless 环境中，此错误导致组件 setup() 抛出异常，组件拒绝渲染（输出 `<!---->`），E2E 测试全部超时。  
**修复**：在 App.vue 中包裹 `<n-config-provider>` + `<n-message-provider>`，同时在 `<script setup>` 中导入两者。

```vue
<template>
  <n-config-provider>
    <n-message-provider>
      <router-view />
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { NConfigProvider, NMessageProvider } from 'naive-ui'
</script>
```

此为生产级 bug 修复（不仅影响测试），提示应尽早补全消息提示基础设施。

### MAJOR — start-e2e-server.sh 二进制缓存策略

**文件**：`scripts/start-e2e-server.sh`  
**问题原始**：仅在二进制不存在时才构建，导致前端重新构建后 binary 嵌入旧 dist/ 内容，测试使用过期 HTML。  
**修复**：添加时间戳比较（`find dist/ -newer $BIN`），如 dist/ 比二进制更新则触发重建。

### MINOR — TC-04 locator 严格模式冲突

**文件**：`web/tests/e2e/03-dashboard.spec.ts`  
**问题**：`page.getByText('仪表盘')` 同时匹配侧边栏菜单项和页面标题，Playwright 严格模式抛出。  
**修复**：改为 `page.getByText('仪表盘').first()`。

---

## 其他注意事项

- `playwright.config.ts` 中 `reuseExistingServer: !process.env.CI` 适合本地开发（减少启动时间），CI 始终使用全新服务器。webServer timeout 从 60s 增至 120s 以容纳 Go 编译时间，合理。
- `fixtures/auth.ts` 的 `bypassWizard()` 逻辑清晰，CSRF token 获取→POST 模式正确。
- 所有 AC 已通过最终验证：verify_all PASS:18 FAIL:0 SKIP:0。
