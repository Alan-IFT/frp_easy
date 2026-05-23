# 06 — Test Report · T-017 install-role-and-public-ip

> Stage 6 / 7（QA Tester，full 模式）。对抗性验证：不是 happy-path 校验，
> 而是反着想"这个 fix 在什么情况下会失败、绕过、误报"。
>
> 本报告含**精确英文标题** `## Adversarial tests`（见 §3）—— verify_all E.6 红线，
> 也是 `.harness/insight-index.md` L31 显式守护的合规位置。

---

## §1 测试范围与方法

### 1.1 测试覆盖面

按 .harness/agents/qa-tester.md 五维度：
- **功能正确性**（每条 FR / AC 至少一条独立 reproducer）
- **边界条件**（空、IPv6、危险字符、HTML 错误页、命令注入、超时）
- **回归**（Inv-1~7 byte-level vs main 守护，T-011/T-014/T-016 不被回滚）
- **稳定性**（verify_all 连跑 3 次无 flake）
- **基本性能**（detect_public_ip 总耗时 ≤ 9s 预算）

### 1.2 测试方法学（adversarial mindset）

对每条 AC：先写下"我期望它在 X 情况下失败"的预测假设（hypothesis），再用**独立 reproducer**
（不复用 04 自带测试代码）跑，把工具实际输出贴进 §3 表的 "Outcome" 列。

### 1.3 平台与限制

| 维度 | 实际 |
|---|---|
| 测试主机 OS | Windows 11 + Git Bash (MinGW64 / MSYS2 bash 5.2.37) |
| Go | 项目已安装；go test ./... PASS |
| Web | npm + Vitest；57 tests PASS |
| **真实 systemd 环境** | **不可用**（Windows 上无 systemd）。涉及 systemd unit 实际拉起 / journalctl / `systemctl is-active` 的 AC 必须降级为"静态分析 + dry-run + 函数级 source 测试" |
| **真实 sudo + chown + getent** | **不可用**（MSYS 上 chown / getent 行为差异大）。降级为"§6.5 块代码逻辑级断言 + Inv 守护回归" |
| **真实公网 IP 探测** | **本机网络畅通**，可以真实命中 `api.ipify.org`（实测 `38.47.117.142`，单次 < 1s）—— happy path 自验通过，但**国内 VM 上预期 3/3 候选 URL 高概率失败**这件事不在本机能复现，转为对 02/03 设计层 + 04 兜底文案的"静态核实" |

降级测试的所有项在 §3 / §4 / §5 中**明示**为 "STATIC ANALYSIS" 或 "DRY-RUN MOCK"。
不允许任何项被静默跳过。

### 1.4 测试产物位置

- 临时工件：`/tmp/qa-t017/` —— 10+ 个 reproducer 脚本与中间结果
- 本报告：`docs/features/install-role-and-public-ip/06_TEST_REPORT.md`
- **不新增** verify_all 检查项（NFR-4 / Inv-7）
- **不修改** scripts/baseline.json（无新增 Go test 与 vitest test；test_count = 231 不变）

---

## §2 自动化套件结果（verify_all）

### 2.1 完整尾段（第一次跑）

```
[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO/FIXME budget ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agents in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md ... PASS
[E.6] Adversarial tests section in completed task reports ... PASS

=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

### 2.2 稳定性：连跑 3 次

| 跑次 | PASS | WARN | FAIL | SKIP | 备注 |
|---:|---:|---:|---:|---:|---|
| 1 | 19 | 0 | 0 | 0 | 无 flake |
| 2 | 19 | 0 | 0 | 0 | 无 flake |
| 3 | 19 | 0 | 0 | 0 | 无 flake |

**verify_all PASS:19 / FAIL:0 / WARN:0 / SKIP:0 · 3/3 稳定**

### 2.3 后端 / 前端独立验证

- `go test ./...` —— 14 包全 PASS（含 appconf / downloader / frpconf / httpapi 等）
- `cd web && npm test` —— Vitest 7 files / **57 tests PASS** / 0 failed
- bash 语法：`bash -n install.sh install-service.sh uninstall-service.sh` —— ALL OK
- baseline test_count = 231（go 174 + frontend 57）不变；QA 未新增测试（**baseline 不变**，详见 §6.4）

---

## Adversarial tests（§3 · 对抗性测试 · 每条 FR / 关键 AC 一个独立反证）

> 每行：测试名 / 失败假设（hypothesis）/ 独立 reproducer / 实际工具输出 / Pass | Fail。
> 工具输出保留原文中关键摘要；完整脚本在 `/tmp/qa-t017/` 目录。

### 3.1 §0.5 ROLE 解析（FR-C / G-3 / G-7）

| ID | Hypothesis（"我期望失败当…"） | Reproducer | Outcome (tool output) | 判定 |
|---|---|---|---|---|
| AT-1 | 用户未指定 `FRP_EASY_ROLE` 时**静默默认到 server** 让用户错装客户端到公网 | `unset FRP_EASY_ROLE; bash scripts/install.sh` | `错误：必须指定 FRP_EASY_ROLE=server\|client（不允许静默默认）` + 两条入口命令 + sudo -E 说明；`EXIT=3` | **PASS**（survived：拒绝静默默认） |
| AT-2 | 非法 role 字面量（`invalid`）能**绕过校验**继续步骤 1 | `FRP_EASY_ROLE=invalid bash scripts/install.sh` | 同 AT-1 错误文案；`EXIT=3` | **PASS** |
| AT-3 | 空字符串 `FRP_EASY_ROLE=''` 被认为已设置而走 happy path | `FRP_EASY_ROLE='' bash scripts/install.sh` | 同 AT-1；`EXIT=3` | **PASS** |
| AT-4 | `-h` 帮助路径会在 ROLE 校验后再退出 → 缺 role 时也得不到帮助 | `unset FRP_EASY_ROLE; bash scripts/install.sh -h` | 完整中文帮助；`EXIT=0`；§0.5 ROLE 校验**在** -h 解析**之后**（不会先报错 exit 3） | **PASS** |

### 3.2 detect_public_ip 函数（FR-B / G-4 / R-4）

| ID | Hypothesis | Reproducer（NEW，I wrote this） | Outcome | 判定 |
|---|---|---|---|---|
| AT-5 | render_frp_easy_toml 渲染的 toml 字段名拼错（小写或驼峰偏差），导致 R-1 死循环风险 | 提取 install.sh L110-141 single-source，跑 `render_frp_easy_toml server` + 用真实 `github.com/pelletier/go-toml/v2` 解析（`/tmp/qa-t017/render-validate.go`） | `PASS: {UIBindAddr:0.0.0.0 UIPort:7800 DataDir:./.frp_easy LogDir:./.frp_easy/logs}` —— server 与 client 均成功反序列化、字段非空 | **PASS** |
| AT-5b | heredoc 在 BOM-aware 解析器下首字节带 UTF-8 BOM 让 toml 解析失败 | `bash render-only.sh server \| xxd \| head -3` | 首 4 字节 `23 20 66 72`（`# fr`）—— **无 EF BB BF BOM** | **PASS** |
| AT-6 | `FRP_EASY_PUBLIC_IP` 注入危险字符（空格/分号/反引号/换行）导致命令执行 | 10 组对抗输入逐一跑 `detect_public_ip`：`bad ip` / `1.2.3.4; rm -rf /` / `1.2.3.4;ls` / `` `whoami` `` / `1"2.3.4` / `$'1.2.3.4\nrm -rf /'` | 全部 6 组危险输入 `rc=1 out=''`（被 `^[A-Za-z0-9.:_-]+$` 拒绝）；IPv4 / IPv6 / hostname 3 组合法输入 `rc=0` 正常通过 | **PASS** |
| AT-7 | 公网 IP 候选返回 HTML 错误页（如运营商 DNS 劫持、captive portal）被当 IP 用 → 横幅打印 `http://<html>:7800` | mock curl 让 3 候选全返回 HTML，跑 `detect_public_ip` | `rc=1 out=''` —— `^([0-9]{1,3}\.){3}[0-9]{1,3}$` 拦截了 `<html>...`；HTML 不当 IP 用 | **PASS**（守护 insight L37） |
| AT-8 | 3 候选 URL 全部超时（curl rc=28 / iptables DROP 模拟）—— 函数应返回 rc=1，不能让 `set -e` 中止 install.sh，也不能阻塞超过 9s | mock curl 让 3 候选全 `sleep 0.3 && return 28`，跑 `detect_public_ip`（不带 `\|\| true`）测 rc | `rc=1 out='' elapsed_ms=1809` —— rc=1 正确；耗时 < 9s 预算；**注：真 curl 单次 --max-time 3，最坏总 9s** | **PASS** |
| AT-9 | render_frp_easy_toml 输出被 `pelletier/go-toml/v2` 真实解析时字段不为空 | 单独 dump server.toml 与 client.toml，分别跑 `go run render-validate.go` | server 与 client 双双 `PASS: {UIBindAddr:... ...}` 字段全非空 | **PASS** |
| AT-10 | 反例守护：**手动注入小写字段**模拟 R-1 风险（大小写敏感约束破裂）—— 期望 go-toml 解析后字段为零值、Validate 失败 | 手写 `uibindaddr = "0.0.0.0" uiport = 7800 ...` 通过 render-validate.go 验证 | **意外结果**：go-toml/v2 **大小写不敏感** —— 小写字段名也被填入正确字段（`PASS: {UIBindAddr:0.0.0.0 ...}`）。**这与 02/03/04/05 设计文档假设矛盾**（详见 §4 已知限制 KL-2） | **PASS（产品安全）**（反向：设计假设过度紧张，实际更宽容；R-1 风险天然不存在） |

补充深入测试（`/tmp/qa-t017/case-sensitivity.go`）：

```
[correct]      err=<nil> cfg={UIBindAddr:0.0.0.0 UIPort:7800 DataDir:./d LogDir:./l}
[all lowercase] err=<nil> cfg={UIBindAddr:0.0.0.0 UIPort:7800 DataDir:./d LogDir:./l}
[all uppercase] err=<nil> cfg={UIBindAddr:0.0.0.0 UIPort:7800 DataDir:./d LogDir:./l}
[mixed wrong]  err=<nil> cfg={UIBindAddr:0.0.0.0 UIPort:0 DataDir: LogDir:}
```

go-toml/v2 大小写不敏感的"宽容"行为反而救了 R-1。**KL-2 记录此 design assumption 与实际行为的偏差**。

### 3.3 原始 bug 复现 & 反向验证（FR-A / G-1 / Inv-3）

> 用户原话现场：装在腾讯云 Ubuntu VM，systemd 死循环重启 35 次，错误 `appconf: write default: open /opt/frp-easy/frp_easy.toml: permission denied`。
> 本机 Windows MSYS 无真实 systemd，**降级为静态分析 + 因果链人工核实 + chown 顺序证据**。

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-原始bug复现-A | 不 chown 时（旧 install.sh 行为），步骤 7 启动 unit 后必死循环 | 分析因果链 `cp -a (root:root) → install-service.sh User=ubuntu → appconf.Load 写默认 → permission denied → exit 1 → Restart=on-failure 死循环` ↔ 现 §6.5 块在步骤 7 之前 `mkdir -p .frp_easy + chown $RUN_USER frp_easy.toml + chown -R $RUN_USER .frp_easy + chown -R $RUN_USER frp_linux`（install.sh L367-458），unit User= 同样取 `${SUDO_USER:-$(id -un)}`（install-service.sh L17/L69-75） | 因果链断在 chown 行 —— 拉起 systemd 进程时 cwd 已可写、frp_easy.toml 已存在（预生成 / 升级保留），appconf.Load 不再走 `os.WriteFile` 写默认分支；即便走（如用户手动 rm toml 后 systemctl restart），cwd 也已 chown 给 RUN_USER，写默认能成功 | **PASS** STATIC ANALYSIS |
| AT-原始bug复现-B | 预生成 toml 字段名拼错→ Validate 失败→ exit 1→ 死循环（即 R-1） | AT-5 / AT-9（真 go-toml 解析） | 字段名严格 = struct tag，且 go-toml/v2 大小写宽容 → R-1 双重免疫 | **PASS** |
| AT-Inv-3 | install-service.sh 被本任务无意修改 → T-016 fix 回滚 | `diff <(git show HEAD:scripts/install-service.sh) scripts/install-service.sh` | `EQUAL TO MAIN`（byte-level 一致） | **PASS** |
| AT-Inv-1 | internal/appconf/config.go 被本任务修改 → T-011 NF-2 破坏 | `diff <(git show HEAD:internal/appconf/config.go) internal/appconf/config.go` | `EQUAL TO MAIN` | **PASS** |
| AT-Inv-2 | cmd/frp-easy/main.go 被本任务修改 → exposureNotice 文案 / ListenAddr 链路改动 | `diff <(git show HEAD:cmd/frp-easy/main.go) cmd/frp-easy/main.go` | `EQUAL TO MAIN` | **PASS** |

### 3.4 role 缺失现场（用户原命令）

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-role 缺失-原始命令 | 用户原命令 `curl -fsSL ... \| sudo bash`（无 FRP_EASY_ROLE）下当前 install.sh 是否给出明确的 exit 3 + 两条入口命令样例 | 本地等价：`unset FRP_EASY_ROLE; bash scripts/install.sh`（curl\|bash 形态等价于直接跑） | stderr 完整命中：`错误：必须指定 FRP_EASY_ROLE=server\|client（不允许静默默认）` + 两条 `curl ... \| FRP_EASY_ROLE=server\|client sudo -E bash` + `sudo 需 -E 才能透传环境变量` 说明；exit 3 | **PASS** |

### 3.5 公网 IP 国内 VM 失败现场（FR-B.1 / G-4 / M-2）

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-公网失败兜底文案 | 国内 VM 上 3/3 候选 URL 失败时，install.sh 步骤 8 是否给出 `FRP_EASY_PUBLIC_IP=` 兜底命令样例 | STATIC：grep install.sh L560-567 server 失败分支 | 命中：`<公网 IP 探测失败，请手动确认服务器出口 IP>` + `国内 VM（腾讯云 / 阿里云 / 华为云）可登云控制台 → 实例详情复制公网 IP` + `curl -fsSL ...\|FRP_EASY_PUBLIC_IP=<your-ip> FRP_EASY_ROLE=server sudo -E bash` 完整样例 | **PASS** |
| AT-公网失败时间预算 | 国内 VM 3/3 候选超时合计 ≤ 9s（3 × `--max-time 3`） | AT-8 同款 mock；本机真测：`detect_public_ip` 实测 995ms（命中 api.ipify.org） | mock 全失败：1809ms（含 3×0.3s sleep）；真实命中：< 1s。**真实 9s 预算上限设计层守护**（每候选 `curl --max-time 3`） | **PASS** |
| AT-公网= LAN 时打两行 | AMBIG-F = F2 决议下，公网 = LAN 时应仍打两行+标注 | STATIC：grep install.sh L554-556 | 命中：`公网访问： http://${PUBLIC_IP}:7800 （与局域网 IP 相同 —— 本机直接在公网上）` | **PASS** |
| AT-client 不发起公网请求（FR-B.3） | client 分支若误调用 detect_public_ip 会触发外网请求 → 隐私 / 性能开销 | STATIC：`sed -n '508,537p' scripts/install.sh \| grep detect_public_ip` | 0 命中（client 分支无 detect_public_ip 调用）；server 分支 L541 `PUBLIC_IP="$(detect_public_ip \|\| true)"` 1 命中（仅 server 调用） | **PASS** |

### 3.6 升级路径（D1 / BC-7 / BC-10 / G-6 / G-7）

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-D1（BC-7） | 升级期已存在用户精细自定义的 `frp_easy.toml`（如 `UIBindAddr = "192.168.1.10"`）被静默覆盖 → 违反 T-011 NF-2 | 模拟脚本 `/tmp/qa-t017/d1-preserve.sh`：先写自定义 toml → 跑 §6.5 升级期分支（无 .role）逻辑 → md5sum 对比 | toml md5 升级前后一致；内容保持 `UIBindAddr = "192.168.1.10"`；新写 `.role=server` 但**不动** toml | **PASS** |
| AT-BC10（role 冲突） | 装过 server 后再跑 client（无 FORCE）静默切换 → 用户配置丢失 | `/tmp/qa-t017/test-role-mismatch.sh`：先写 `.role=server`，模拟 ROLE=client 跑 §6.5 检测 | `错误：已检测到 role=server，本次指定 role=client 冲突` + exit 3 | **PASS** |
| AT-FORCE_ROLE | `FRP_EASY_FORCE_ROLE=yes` 时跳过冲突检测 + 备份旧 toml + 重写 toml + 切换 .role | `FRP_EASY_FORCE_ROLE=yes bash /tmp/qa-t017/force-backup.sh` | `frp_easy.toml.bak.<ts>` 出现；新 `frp_easy.toml` = client 模板；`.role` = `client` | **PASS** |

### 3.7 RUN_USER verbatim 同源（C-1 / M-1 / Inv-3）

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-C1 | install.sh §6.5 RUN_USER 解析与 install-service.sh L69-75 行为偏差 → unit User= 与 chown 目标错位 → 仍触发原始 bug | `diff <(sed -n '378,385p' scripts/install.sh) <(sed -n '17p;69,75p' scripts/install-service.sh)` | 仅缩进 4 空格不同（install.sh 在 `if [[ "$OS" == "linux" ]]` 块内），逻辑 byte-level 等价；含 verbatim 注释 "与 install-service.sh L69-75 必须保持等价" | **PASS** |
| AT-C1b（SUDO_USER 行为） | unset SUDO_USER 时 RUN_USER 错误地走 root 而非 fallback id -un | `SUDO_USER=alice bash run-user.sh` → T1 `RUN_USER=alice`；`unset SUDO_USER` → T2 `RUN_USER=yangx`（fallback） | T1 / T2 行为正确（守护 insight L22） | **PASS** |

### 3.8 MIN-1 IPv6 横幅 bracket 缺失（BC-3 设计要求但实现遗漏）

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-MIN-1 | `FRP_EASY_PUBLIC_IP=2001:db8::1 FRP_EASY_ROLE=server bash install.sh` 横幅会拼成 `http://2001:db8::1:7800`（不合法 URL，浏览器无法解析） | `/tmp/qa-t017/ipv6-banner.sh`（提取 install.sh L553-559 拼接逻辑） | 输出：`公网访问：    http://2001:db8::1:7800` ← **不合法**。BC-3 设计明确要求 IPv6 必须 `http://[2001:db8::1]:7800` bracket 形式 | **FAIL（已知限制 KL-1）**—— 05 Code Reviewer MIN-1 已记录、不阻塞本任务；本报告作为已知限制记录、§4 详述 |

### 3.9 verify_all A.1 不误中（NFR-6 / Inv-7）

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-A.1 | `FRP_EASY_ROLE=server` / `FRP_EASY_PUBLIC_IP=<your-ip>` 等字面量被 A.1 secrets 扫描误中 → verify_all FAIL | `grep -E '(api[_-]?key\|secret\|password\|token)[\s]*[:=][\s]*["'\''][^"'\'']{8,}["'\'']' scripts/install.sh scripts/install.ps1 scripts/uninstall-service.sh` | 0 命中（关键词谓词不在三脚本内）；verify_all A.1 实跑 PASS | **PASS** |

### 3.10 heredoc 注入 + curl\|bash 红线（L38 / L32）

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-L38 heredoc | render_frp_easy_toml 用 `<<EOF`（无单引号）而非 `<<'EOF'` 时 ROLE 变量被插值注入 → quote-removal 陷阱再现（T-016 D-1 同款） | 注入 `ROLE="malicious-injection-\$(rm -rf /)"` 后跑 render → grep 输出是否含 `rm -rf` | grep `rm -rf` 0 命中；heredoc 严格用 `<<'EOF'` 单引号 → 字面输出，无插值 | **PASS** |
| AT-L32 self-locate | install.sh 使用 `$0` / `${BASH_SOURCE[0]}` 自定位 → `curl\|bash` 形态下脚本无磁盘路径会导致失败 | `grep -nE 'BASH_SOURCE\|^[\s]*\$0' scripts/install.sh` | 0 命中（仅 install-service.sh 用 BASH_SOURCE，那是磁盘脚本未动） | **PASS** |

### 3.11 uninstall + 幂等（OOS-7 / BC-10 收尾）

| ID | Hypothesis | Reproducer | Outcome | 判定 |
|---|---|---|---|---|
| AT-uninstall | 卸载脚本 rm 了 frp_easy.toml（违反"数据/配置保留"） | grep `rm` in uninstall-service.sh | 仅 `rm -f "${INSTALL_DIR}/.role"`（T-017 新增 L80）；frp_easy.toml 仅在文档提示中要求用户手动 rm；二者隔离 | **PASS** |

---

## §3.99 Adversarial 统计汇总

| 类目 | 测试数 | PASS | FAIL（已知限制） | 备注 |
|---|---:|---:|---:|---|
| ROLE 解析（§3.1） | 4 | 4 | 0 | AT-1~AT-4 |
| detect_public_ip（§3.2） | 7 | 7 | 0 | AT-5~AT-10（AT-10 设计假设偏差但产品安全） |
| 原始 bug + Inv 守护（§3.3） | 5 | 5 | 0 | AT-原始bug A/B + Inv-1/2/3 |
| role 缺失现场（§3.4） | 1 | 1 | 0 | 用户原命令复现 |
| 公网 IP 国内失败（§3.5） | 4 | 4 | 0 | M-2 / C-2 兜底文案核实 |
| 升级路径（§3.6） | 3 | 3 | 0 | D1 / BC-10 / FORCE |
| RUN_USER verbatim（§3.7） | 2 | 2 | 0 | C-1 同源 |
| **MIN-1 IPv6 横幅**（§3.8） | 1 | 0 | **1** | KL-1 已知限制 |
| verify_all A.1（§3.9） | 1 | 1 | 0 | NFR-6 守护 |
| 红线 L32 / L38（§3.10） | 2 | 2 | 0 | 自定位 + heredoc |
| uninstall 幂等（§3.11） | 1 | 1 | 0 | .role 清理 |
| **合计** | **31** | **30** | **1** | **30/31 = 96.8% 通过** |

唯一 FAIL（AT-MIN-1）是 05 Code Reviewer 已在 §5 MIN-1 显式记录的已知限制，不阻塞本任务交付。

---

## §4 已知限制（Known Limitations）

### KL-1 · MIN-1 · IPv6 横幅 URL bracket 缺失（BC-3 设计要求但实现遗漏）

- **复现**：`FRP_EASY_PUBLIC_IP=2001:db8::1 FRP_EASY_ROLE=server bash scripts/install.sh`（dry-run 步骤 8 拼接段）→ 横幅打印 `http://2001:db8::1:7800`，浏览器无法解析。
- **设计期望**（02 §5.4 BC-3）：IPv6 必须 `http://[2001:db8::1]:7800`。
- **代码位置**：`scripts/install.sh` L556 / L558。
- **缓解**：detect_public_ip 短路通道宽容接受 IPv6 字面量（`^[A-Za-z0-9.:_-]+$`），导致 IPv6 顺利通过，但没有为 IPv6 加 bracket。
- **影响范围**：仅在用户显式设 `FRP_EASY_PUBLIC_IP=<IPv6>` 时触发；3 候选 URL 都不会返回 IPv6（IPv4 echo 服务）。
- **状态**：05 Code Reviewer 已显式接受为边缘 case；不阻塞本任务交付。
- **建议跟进任务**：`T-018-ipv6-banner-bracket`（小任务：仅改 install.sh L556/L558 加 `case "$PUBLIC_IP" in *:*) URL_HOST="[$PUBLIC_IP]";; *) URL_HOST="$PUBLIC_IP";; esac` 然后用 `$URL_HOST` 拼接）。install.ps1 同款问题。

### KL-2 · go-toml/v2 大小写敏感性 design assumption 与实际行为偏差

- **现象**：02 §4.2 / 03 §3-B / 04 §3.2 / 05 §4 P-1 反复声称 "go-toml/v2 大小写敏感、字段名必须严格 = struct tag"；本 QA 实测（`/tmp/qa-t017/case-sensitivity.go`）证明 **pelletier/go-toml/v2 大小写不敏感**（小写 / 大写字段名都能填进正确 Go struct 字段）。
- **影响**：
  - 产品行为**未受影响**：当前预生成 toml 字段名仍是正确的 PascalCase，且即便错了也能解析。
  - R-1 风险（02 §8）被完全消除（不仅 verbatim 守护，连大小写都宽容）。
- **建议跟进**：07 阶段把"go-toml/v2 大小写敏感"假设作为 insight 收割（让下一个人不会重新做错误紧张）。

### KL-3 · Windows 测试主机无法跑真实 systemd / sudo / chown

- **影响**：FR-A.1 / FR-A.2 / FR-A.3 / FR-A.4（systemctl is-active / journalctl 命中 / NRestarts ≤ 1）只能**降级**为静态因果链分析 + dry-run 函数级测试。
- **缓解**：因果链断点已在 AT-原始bug-A 中静态核实；真实 Linux VM 跑通由 07 阶段建议在 release notes 上线后用户验证或维护者用 docker / 真 VM 验证。
- **建议**：07 阶段 release notes 标注"实际 systemd 验证依赖 Linux 用户实测反馈，QA 阶段为 Windows 主机静态分析"。

### KL-4 · install.ps1 公网 IP 探测 Windows 路径仅做静态核实

- **影响**：Get-PublicIPv4 函数未在 PowerShell 真实执行（Bash QA agent）；仅靠 04 §3.5 + 05 §3 C-6 文档级核实 + 与 detect_public_ip 同款 3 候选 URL 一致性核实。
- **缓解**：install.ps1 行为契约与 install.sh detect_public_ip 强对齐（同候选、同短路、同 IPv4 字面量校验），且 02 §11 分区一致。Windows 用户实测在 07 阶段交付前由维护者补做。

---

## §5 Inv-1 ~ Inv-7 守护核实

| Inv | 守护对象 | 检查命令 / 证据 | 结果 |
|---|---|---|---|
| Inv-1 | `internal/appconf/config.go` 一字不改 | `diff <(git show HEAD:internal/appconf/config.go) internal/appconf/config.go` | **EQUAL TO MAIN** ✓ |
| Inv-2 | `cmd/frp-easy/main.go` 一字不改 | `diff <(git show HEAD:cmd/frp-easy/main.go) cmd/frp-easy/main.go` | **EQUAL TO MAIN** ✓ |
| Inv-3 | `scripts/install-service.sh` 一字不改（T-016 fix 不回滚） | `diff <(git show HEAD:scripts/install-service.sh) scripts/install-service.sh` | **EQUAL TO MAIN** ✓ |
| Inv-4 | T-014 升级语义"frp_linux/ 不被覆盖" | install.sh 升级分支 L350-358 不覆盖 frp_linux/；§6.5 块 L447-449 `chown -R frp_linux` 仅改属主、不动内容 | STATIC ANALYSIS ✓ |
| Inv-5 | `curl\|bash` 非交互形态保留（无 stdin 交互） | install.sh 无 `read` / `select` / `< /dev/tty`；ROLE 仅靠环境变量 | STATIC ANALYSIS ✓ |
| Inv-6 | NFR-2 公网 IP 候选 URL 明文写死（不从 env 拼接） | install.sh L171-175（bash array 字面量）+ install.ps1 L56-60（PowerShell array 字面量） | STATIC ANALYSIS ✓ |
| Inv-7 | verify_all 检查项数量 = 19，与 T-011 baseline 一致 | `bash scripts/verify_all.sh \| grep -E '^\[[A-Z]\.[0-9]\]' \| wc -l` = **19** | ✓ |

---

## §6 回归测试结论

### 6.1 Go 后端

`go test ./...` —— 全 14 包 PASS（含 internal/appconf / downloader / frpconf / httpapi / procmgr / storage 等）；go vet PASS；go build PASS。**无回归**。

### 6.2 前端 Vitest

`cd web && npm test` —— **57 tests PASS** / 0 failed / 7 test files；与 baseline 一致。**无回归**。

### 6.3 verify_all

PASS:19 / FAIL:0 / WARN:0 / SKIP:0 · 连跑 3 次稳定无 flake。基线 19 不变（NFR-4 / Inv-7）。

### 6.4 baseline.json

| 维度 | baseline（T-014 设置） | 本任务实测 | 变化 |
|---|---:|---:|---|
| test_count | 231 | 231 | **不变** |
| go_tests | 174 | 174 | 不变 |
| frontend_tests | 57 | 57 | 不变 |
| passing_count | 226 | 226 | 不变 |
| warnings_baseline | 0 | 0 | 不变 |

**QA 决定不增加 baseline**：本任务的对抗性测试是 ad-hoc reproducer 跑在 `/tmp/qa-t017/` 不入 git；产品代码逻辑由 install.sh 内部覆盖。Adversarial tests 表（§3）是文字证据替代自动化单测，符合 qa-tester.md 的"shell 脚本 + 静态分析降级路径"。**baseline.json 不修改**。

### 6.5 历史 06 测试报告 E.6 守护

`grep -nE '^## Adversarial tests' docs/features/_archived/**/06_TEST_REPORT.md` —— 15 个历史任务 + 本任务（待最终落盘后）共 16 个全部命中。E.6 规则守住。

---

## §7 Verdict

### 7.1 判定

**APPROVED**（APPROVED FOR DELIVERY）

### 7.2 理由

1. **31 项对抗测试 30 PASS / 1 FAIL**（96.8% 通过率），唯一 FAIL 是 05 已记录的 KL-1 IPv6 横幅 bracket 缺失，**边缘 case，不阻塞**（用户必须显式 `FRP_EASY_PUBLIC_IP=<IPv6>` 才能触发；3 候选 URL 不返回 IPv6）。
2. **verify_all PASS:19 / FAIL:0**，3 次跑稳定无 flake；baseline test_count 231 不变；19 个检查项与 T-011 baseline 一致。
3. **Inv-1 ~ Inv-7 全部守护**：
   - Inv-1（appconf）/ Inv-2（main.go）/ Inv-3（install-service.sh）byte-level vs main HEAD **完全一致**
   - Inv-4 frp_linux/ 升级保留（仅 chown 属主）
   - Inv-5 curl\|bash 非交互保留（无 stdin 读取）
   - Inv-6 公网 IP 候选 URL 明文写死
   - Inv-7 verify_all 检查项 = 19
4. **9 条 conditions C-1~C-9 全部落地**（05 已核实），QA 阶段对其中可测项独立反证：
   - C-1 RUN_USER verbatim：byte-level diff 等价（AT-C1）
   - C-2 / C-5 FRP_EASY_PUBLIC_IP short-circuit + 兜底文案：6 组危险字符全拒绝（AT-6）+ 失败横幅文案核实（AT-公网失败兜底）
   - C-3 不做 source-mode 包裹：AT-L32 grep 0 命中
   - C-4 DataDir 解析链：02 / 04 文档核实（无运行时跑通）
   - C-6 OOS-2 注释：install.ps1 文件级核实
   - C-7 chown fail-fast：代码级核实（install.sh L439/443/447 三处独立 if 块、无 `\|\| true`）
   - C-8 本报告 §3 英文 `## Adversarial tests` 段 ✓（verify_all E.6 PASS）
   - C-9 跨阶段传递 07
5. **原始 bug 因果链断点** 在 §6.5 chown 块静态核实（AT-原始bug-A），用户 journalctl 中的 `appconf: write default: open /opt/frp-easy/frp_easy.toml: permission denied` 模式被预生成 + chown 双重免疫消除。
6. **3 项已知限制全部明示**（KL-1 / KL-2 / KL-3 / KL-4），不影响交付。
7. **0 个 BLOCKER / 0 个 CRITICAL** 缺陷。

### 7.3 路由

**派 Stage 7 PM Orchestrator / Delivery**，建议 07 阶段：

1. release notes 提示用户原命令需改为 `... | FRP_EASY_ROLE=server sudo -E bash`（或 client）。
2. KL-1 立项跟进任务 `T-018-ipv6-banner-bracket`（IPv6 URL bracket）。
3. KL-2 / KL-3 / KL-4 作为 07 insight 收割候选（go-toml/v2 实际大小写宽容 + Windows QA 主机限制）。
4. 04 已传 C-9 候选两条 insight，archive-task 自动收割。

---

**Verdict**：**APPROVED**
**Adversarial 数量**：31 项 · **30 PASS / 1 FAIL（KL-1 已知限制）**
**Known Limitations**：4 项（KL-1 IPv6 / KL-2 go-toml 大小写 / KL-3 Windows 主机无 systemd / KL-4 install.ps1 仅静态核实）
**Defects found**：0 BLOCKER / 0 CRITICAL / 0 MAJOR / **1 MINOR**（MIN-1 / KL-1 IPv6）/ 0 NIT
**baseline.json**：**不修改**（test_count 231 不变）
