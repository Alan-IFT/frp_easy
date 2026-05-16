# Gate Review — T-004 tech-debt-cleanup

**审查日期**：2026-05-16  
**结论**：**APPROVED FOR DEVELOPMENT**

所有 7 项改动范围明确，改动局部，无跨包破坏性变化，AC 可验证，风险低。

## 要点

1. OPT-1（verify_all 路径）：pushd/popd 模式标准，PS1 对应 Push-Location/Pop-Location，合理
2. OPT-2（wizard 守卫）：新增守卫不影响现有逻辑，shouldShow=false 是正确的重定向条件
3. OPT-4（slog 双写）：io.MultiWriter 标准模式，无风险
4. OPT-5（版本注入）：fallback "dev" 保证无 git tag 时不报错
5. OPT-6（ParseIPFromJSON）：两实现语义完全相同，统一后代码更简洁
6. OPT-7（health 端点）：必须在 ReadyGate Use() 之前注册，设计已注明
7. OPT-8（TOML 预检）：os.IsNotExist 是正确的空值检测方式

## 开发者注意事项

- health handler 必须在 `r.Use(ReadyGate(...))` **之前**在 chi router 注册，否则启动期间返回 503
- autoRestoreProcs 增加 configPaths 参数，main.go 已有 frpcTOML/frpsTOML 变量可直接传入
- verify_all.sh 修复后，如果前端 lint 或 typecheck 有问题，应修复，不应绕过
- 完成后运行 `scripts/verify_all.sh`，FAIL 必须为 0，且 B.1/B.3 不得为 SKIP
