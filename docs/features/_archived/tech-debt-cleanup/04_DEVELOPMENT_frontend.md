# Development Record — Frontend partition

## Partition
dev-frontend — owns: `web/**`

## Files changed (this partition only)
- `web/src/router.ts` — 添加向导路由守卫，防止已完成向导的用户重复访问 /wizard

## 改动摘要

### 问题（TD-1）
`router.ts` 的 `beforeEach` 守卫原有逻辑仅在导航到 `/dashboard` 时检查是否需要显示向导，但对直接访问 `/wizard` 的情况缺乏保护。已完成向导后的用户仍可通过地址栏直接输入 `/wizard` 进入向导页，造成重复向导体验。

### 解决方案
在现有 `/dashboard` 向导检查块之后、`return true` 之前，新增如下守卫块：

```typescript
// 向导已完成时，直接访问 /wizard 重定向到 /dashboard（TD-1 修复）
if (auth.user !== null && to.path === '/wizard') {
  const wizard = useWizardStore()
  if (!wizard.checked) {
    await wizard.checkWizard()
  }
  if (!wizard.shouldShow) {
    return '/dashboard'
  }
}
```

### 逻辑说明
- `wizard.shouldShow` 为 `false`（向导已完成或不需要显示）→ 重定向到 `/dashboard`
- `wizard.shouldShow` 为 `true`（向导应该显示）→ 允许访问 `/wizard`
- `wizard.checked` 为 `false` 时先调用 `checkWizard()` 获取状态，再做判断（避免重复 API 调用）
- 与已有代码风格一致：使用 `const wizard = useWizardStore()`，`await wizard.checkWizard()`

### 未修改的内容
现有所有守卫逻辑均未改动，仅新增一个独立的守卫块。

## Out-of-partition coordination
本次修改完全在 `web/**` 范围内，无需后端或 DB 分区协调。

## Verdict
READY FOR REVIEW (frontend partition complete — TD-1 /wizard 重复访问修复)
