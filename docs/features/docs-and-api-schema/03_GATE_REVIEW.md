# Gate Review — T-005 docs-and-api-schema

**任务 ID**：T-005  
**Slug**：docs-and-api-schema  
**审查日期**：2026-05-16  
**模式**：full

---

## Verdict

```
APPROVED WITH CONDITIONS
```

开发可以推进，须在实施时满足以下两个条件：

**条件 C-1（DownloadState status 值）**：Developer 写入 openapi.yaml 时，须将 `DownloadState.status.description` 更正为 `"idle | downloading | success | failed"`（不得沿用设计文档中错误的 `"done | error"`），并将 `error` 字段描述更正为 `错误信息（status=failed 时）`。

**条件 C-2（OPT-9 标注时机）**：Developer 更新 `docs/project-status.html` §6 时，对 OPT-9 行保持原样，不预先添加 `已实现` badge。遵循需求 §3 A-5 和 §5 边界条件。

---

## 关键发现

**F-1：DownloadState status 描述值与实际 Go 常量不符**

- 设计文档写：`"idle | downloading | done | error"`
- 实际 Go 常量（`internal/downloader/downloader.go:32-36`）：`StatusSuccess="success"`，`StatusFailed="failed"`
- Developer 必须以代码为准

**F-2：OPT-9 标注责任在需求与设计之间轻微歧义**

- 需求 A-5 说由 QA 更新；设计说由 Developer 在验收后更新
- 以需求为准：OPT-9 行在开发阶段保持原样

---

## 审计检查清单

| 维度 | 结论 | 备注 |
|---|---|---|
| 需求完整性 | PASS | 15 条 AC 全部可机器验证 |
| 设计完整性 | WARN | F-1 status 值不符；F-2 标注责任歧义 |
| 复用正确性 | WARN | F-1 须修正 |
| 风险覆盖 | PASS | 6 项风险均已识别 |
| 迁移安全性 | PASS | 无数据迁移，可随时回滚 |
| 边界处理 | PASS | 范围外条目明确 |
| 测试可行性 | PASS | 每条 AC 均有可执行验证命令 |
| 范围外清晰度 | PASS | 显式 Out-of-scope 章节 |
