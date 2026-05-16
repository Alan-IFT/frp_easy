# 项目 SPEC

这个目录是**项目级需求的真相源**。每个重要 feature 在进入流水线之前，应该有自己的 SPEC 文档。

## 文件命名

- `README.md` — 本文件，索引。
- `<feature>.md` — 每个 feature 一份 SPEC，例：`csv-export.md`、`auth-v2.md`。

## SPEC 应包含什么

- **目标**：一段话，业务理由。
- **范围**：做什么、不做什么。
- **约束**：技术、合规、时间。
- **验收标准**：可测试的条件。
- **风险 / 待解问题** 让用户决定。

## SPEC 如何进入流水线

1. 你在这里写一份粗略的 SPEC（或者粘贴一段聊天记录）。
2. 交给 Requirement Analyst：_"把 `docs/spec/csv-export.md` 精化成任务需求。"_
3. Analyst 写 `docs/features/<task>/01_REQUIREMENT_ANALYSIS.md` 引用这份 SPEC。
4. PM Orchestrator 推进剩下流程。

## 风格

用确定性语言："必须"、"将"。避免"应该"、"可能"、"也许"。
拿不准时，写成 open question；Requirement Analyst 会和用户一起解决。
