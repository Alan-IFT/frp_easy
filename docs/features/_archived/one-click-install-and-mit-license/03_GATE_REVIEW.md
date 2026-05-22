# 03 Gate Review — T-012 one-click-install-and-mit-license

> Harness 流水线 stage 3 产出。Gate Reviewer 独立核验（已实读 install-service.{sh,ps1}、package.sh、release.yml、README.md、DEPLOYMENT.md）。
> 上游：01（READY FOR DESIGN）、02（READY）。

## 1. 八维审计

| # | 维度 | 结论 |
|---|---|---|
| 1 | 需求完整性 | PASS — 17 In-scope 可观察，12 BC 给明退出码与中文文案，5 开放问题 PM 已裁决 |
| 2 | 设计完整性 | PASS — §13 把 17 AC / 12 BC 逐条落到小节/退出码/文案 |
| 3 | 复用正确性 | PASS — install-service.{sh,ps1}、package.sh L251 LICENSE 打包逻辑经实读核验属实，install.* 不复刻 systemd/sc.exe |
| 4 | 风险覆盖 | PASS — curl -f 与状态码冲突、限流响应是合法 JSON、set -e 陷阱、升级误删配置均有缓解 |
| 5 | 迁移安全 | PASS — 无 schema 变更；升级用白名单逐项覆盖，frp_easy.toml 与 .frp_easy/ 显式排除 |
| 6 | 边界处理 | PASS — 空/损坏包、中途失败清理、非 root、缺依赖、限流、无 release、非 amd64、macOS 均有设计 |
| 7 | 测试可行性 | PASS — 静态 AC 可机械验；AC-10/AC-11 因无 release 降级为交付后人工验证，合理 |
| 8 | 范围边界 | PASS — §15 八条 Out-of-scope 明确 |

8 维全 PASS，无 WARN/FAIL。

## 2. 发现（均 INFO 级，不阻塞）

- **F-1**：`NOTICE` 不会被打进发布包（package.sh L251 只打包 LICENSE）。本期 Out-of-scope 不改 package.sh，故不要求 NOTICE 进包；建议 PM 知会用户，后续任务可补。
- **F-2**：升级重跑 install-service.sh 与 DEPLOYMENT.md C.2.4「升级无需重跑」措辞张力。Developer 改文档时加一句澄清（一键安装升级会自动重跑服务注册，幂等安全）。
- **F-3**：`bash "$SERVICE_SCRIPT"` 调用方式正确，无需处置。
- **F-4**：`curl|sudo bash` 下 SUDO_USER 正确传递，服务以真实调用者运行，与 insight-index 一致。
- **F-5（需知会用户）**：release.yml 只产 linux-amd64 + windows-amd64，无 darwin。macOS 一键安装将得到「无 macOS 专用包，请用源码构建」的 exit 1 友好报错 —— 这是**预期行为**。Architect 对 R-6 的裁决（macOS 无资产→exit 1 定制文案，BC-11 完整路径代码实现但当前不可达）经核验合理。

## 3. 安全暴露评审（curl|bash / irm|iex）
不否决该模式（用户明确要求，业界惯例）。缓解到位：文档明示风险 + 给出「先下载审阅再执行」备选（AC-15 grep 校验）、raw-url 写死、脚本不写明文凭据。PASS。

## 4. 升级红线评审
升级分支用白名单逐项覆盖（只覆盖 bin/frp_linux/scripts 等发布包文件），脚本中绝不出现针对 frp_easy.toml/.frp_easy 的 rm/cp。Developer 须保证「先覆盖完所有文件、再调用服务脚本」的顺序，且 rm -rf 目标变量永不为空。PASS。

## 5. 开发期高概率提问（预答）
- Q：升级 `rm -rf scripts/` 会删掉正在调用的 install-service.sh 吗？→ 覆盖在调用之前完成，调用时已是新版本；须严格保持「先覆盖、再调用」顺序。
- Q：去掉 `curl -f` 后 5xx body 会被错切吗？→ 先判状态码后解析 JSON，5xx body 不会进解析。
- Q：`irm|iex` 下 `exit N` 退出整个会话？→ 会，属预期（对称于 curl|bash），收尾文案可提示「窗口可关闭」。
- Q：`tar xzf` 解压（非校验）阶段失败？→ 建议补中文文案 `|| { echo "错误：发布包解压失败（磁盘空间不足或权限问题）。" >&2; exit 1; }`。
- Q：`-h/--help` 在管道形态收不到参数？→ AC-5 测法是 `bash install.sh -h`（本地形态），help 分支须在依赖检测之前。

## 6. 红线合规
不碰 `.harness/`、`.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md`、归档文档；release.yml/install-service.*/package.* 明列不修改。LICENSE 英文标准全文为 NFR-5 明示的输出语言例外。

## 7. Verdict

**APPROVED FOR DEVELOPMENT**

8 维全 PASS，5 条 INFO 发现不阻塞。给 PM/用户的知会：F-5（macOS exit 1 是预期行为）、F-1（NOTICE 不入发布包）、AC-10/AC-11 需用户先发布 GitHub Release 才能实测。开发期润色：tar xzf 解压失败补中文文案、DEPLOYMENT.md 消解 C.2.4 措辞张力。
