# PM_LOG — T-038 boot-autostart-hardening

> PM Orchestrator 在此记录每个阶段的派发与决策。

## 任务背景

用户主诉（2026-05-25）：
- "frp client 安装,当前看起来是安装在用户级，设备关机重启后，远程就无法再次连接"
- "可能 frps server 也是如此"
- "理论上应该是只要设备开机就可以用，不管是否登录"
- 决策原则：用户体验好、符合软件工程标准、长期易使用易维护

## PM 预热观察

读 install.sh / install.ps1 / install-service.sh / install-service.ps1 / main.go autoRestoreProcs：

- Linux: `install-service.sh` 写 `/etc/systemd/system/frp-easy.service` + `User=${SUDO_USER或id-un}` + `Restart=on-failure` + `WantedBy=multi-user.target` —— **是 system-level service**，理论上开机自启，不依赖 login session。
- Windows: `install-service.ps1` 用 `sc.exe create binPath= "<exe>" start= auto` —— SCM 默认 `LocalSystem`，**system-level**，开机自启不依赖 login。
- frp-easy 内部：main.go L294 `autoRestoreProcs()` 读 `kv.mode.frpc.enabled` / `mode.frps.enabled` → `pm.Start(kind)` 自动恢复 frpc/frps 子进程（AC-9）。

**链路理论上完整**。但用户实测"reboot 后远程失联"——可能根因：

1. **链路 A 假设**：用户没跑 install 脚本，而是双击 frp-easy.exe / 裸跑 `./frp-easy` → 没注册服务 → reboot 后无任何进程。
2. **链路 B 假设**：服务注册了，frp-easy 启动了，但 `mode.frpc.enabled` 在 UI 启动 proxy 时**没被持久化**为 true → autoRestoreProcs 跳过 → frpc 没被拉起。
3. **链路 C 假设**：服务注册了，但 LocalSystem / RUN_USER 账户访问 frpc.exe 路径有问题（cwd / 权限）。
4. **链路 D 假设**：UI 上某个"启动"按钮调的不是 procmgr.Start（不写 kv.mode.enabled），所以 autoRestoreProcs 无法识别"上次在跑"。

需要 RA 把这 4 个链路逐一证伪/证实，定位真因，并把不存在的链路也写明"已确认不是根因"以避免重做。

## Stage 1 派发（即将进行）

派发给 Requirement Analyst — 不直接生成代码，仅写 01_REQUIREMENT_ANALYSIS.md，并：
- 实际 grep 代码确认 4 个假设链路；
- 给出明确的 functional / non-functional 要求；
- 列出所有 ambiguity（按 CLAUDE.md "你来决策即可"原则 PM 直接决议，不打扰用户）。

## insight-index 命中扫描

- L30 / L38：多任务并行工作树 → verify_all 跨任务归责动作。本任务可能与 archived 任务无并行，但 dev 阶段要警惕本机 7800 端口被 archived frp-easy 服务占用。
- L20 / L21 / L27：PM 派发上下文可能裁剪 Task / Bash / PowerShell 工具——若 Task 派发不可达，PM 自身角色化跑 RA → SA → GR → Dev → CR → QA。

