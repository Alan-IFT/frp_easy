# 用户原始输入 — T-019

## 触发场景

用户在 Win11 管理员终端执行：

```
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
```

属于升级安装路径（"检测到已存在安装，执行升级"）。

## 失败现象（原始 stdout/stderr）

```
==> [1/8] 检查运行环境...
==> [2/8] 探测 CPU 架构...
    检测到平台：windows-amd64
==> [3/8] 查询 GitHub 滚动发布...
==> [4/8] 解析发布包下载地址...
    滚动发布版本：rolling
==> [5/8] 下载并校验发布包...
==> [6/8] 检测到已存在安装，执行升级（保留 frp_easy.toml 与 .frp_easy\）...
==> [7/8] 注册 Windows 服务...
==> 已生成服务包装脚本：C:\Program Files\frp-easy\frp-easy-svc.cmd
==> 检测到已存在的服务：frp-easy（将刷新 binPath / DisplayName / 失败动作 并重启）
[SC] ChangeServiceConfig 成功
[SC] StartService 失败 1053:

服务没有及时响应启动或控制请求。

Write-Error:
Line |
 286 |      & $svc
     |      ~~~~~~
     | sc.exe start 失败（退出码 1053）；请运行 sc query frp-easy 查看状态。
```

## 任务

修复，确保 Win11 管理员终端运行 install.ps1 后服务能正常启动到 RUNNING 状态。
