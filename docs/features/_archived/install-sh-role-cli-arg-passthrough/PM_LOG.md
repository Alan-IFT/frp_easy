# PM_LOG — T-035 install-sh-role-cli-arg-passthrough

## 用户原始报告

```
alan@alan-911:~$ curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash
sudo: preserving the entire environment is not supported, '-E' is ignored
错误：必须指定 FRP_EASY_ROLE=server|client（不允许静默默认）
  服务端（公网 VM）：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=server sudo -E bash
  客户端（内网设备）：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash
  说明：sudo 需 -E 才能透传环境变量；服务端默认监听 0.0.0.0，客户端默认监听 127.0.0.1。
curl: (23) Failure writing output to destination, passed 1378 returned 1273
alan@alan-911:~$
```

环境：Ubuntu 26 LTS（alan@alan-911）。

## PM 决策原则（用户指令）

> 以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策。

用户授权 PM 直接决策与执行，且所有 commit / push 由 PM 操作。

## 路由

模式 = `/harness`（full 7-stage）。理由：
- 涉及多文件（install.sh + README.md + DEPLOYMENT.md）联动改动
- 改的是公开"一键安装"入口字串 —— 用户感知极强，回归代价高
- 与 T-017（role 拒绝静默默认）、T-026/T-031（install.ps1 -NoExit）属同一"安装入口字串"维护域，需 RA/Architect 系统性审视而非 trivial 直改

## 阶段进度

- [x] Stage 1 — Requirement Analyst
- [ ] Stage 2 — Solution Architect
- [ ] Stage 3 — Gate Reviewer
- [ ] Stage 4 — Developer
- [ ] Stage 5 — Code Reviewer
- [ ] Stage 6 — QA Tester
- [ ] Stage 7 — Delivery

## 派发说明（PM 上下文裁剪）

按 insight L31-L34、L38 既有事实，PM 派发上下文在 SDK 下无 Task 工具，因此本任务 7 个 stage 全部由 PM 在本上下文按 `.harness/agents/<name>.md` 角色契约自行扮演产出文档，与真派发任务字节级同构以保证 grep / archive-task / verify_all 等下游工具行为不变。
