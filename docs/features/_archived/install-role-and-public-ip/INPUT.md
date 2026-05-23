# INPUT — T-017 install-role-and-public-ip

> 用户原话（2026-05-23），来自一台腾讯云 Ubuntu VM 实测。

> 出错了，且只有 127.0.0.1 和局域网 ip，而不是公网 ip，若是服务端，根本没法访问到 UI 页面，
> 需要修复错误，并更正 ip 选择，理论上好像只能在安装脚本运行过程中选择是服务端还是客户端，
> 没法在安装后，因为服务端需要公网 ip，而客户端应该是监听 127.0.0.1 才是最安全的。

## 实测现场（用户提供 journalctl 节选）

- 安装结果横幅：
  - `本机访问：    http://127.0.0.1:7800`
  - `局域网访问：  http://10.1.20.7:7800`（VM 的内网网卡，**不是公网出口 IP**）
- systemd 服务死循环重启（restart counter 已到 35），关键错误：
  ```
  frp-easy[…]: 加载 frp_easy.toml 失败：appconf: write default: open /opt/frp-easy/frp_easy.toml: permission denied
  systemd[1]: frp-easy.service: Main process exited, code=exited, status=1/FAILURE
  ```

## 任务边界

本任务覆盖三件耦合的事：

1. **修复服务启动崩溃**：appconf 写默认配置时被拒。
2. **修正 IP 显示**：服务端场景必须能给出/引导到公网 IP；纯内网/客户端不应误导用户。
3. **安装期角色选择**：服务端 vs 客户端的语义只能在装机时决定（默认监听地址、是否开放外网、是否打印公网 IP 提示）。
   - 用户主张：服务端 → 监听 0.0.0.0、需要公网 IP；客户端 → 监听 127.0.0.1 最安全。
   - 一键安装的 `curl | sudo bash` 形态本身是非交互的，是否能"在安装中选"是设计决策点。

不在本任务范围：FRP 业务侧 `frpc.toml`/`frps.toml` 的 UI/默认值改动（那是另一个 feature surface）。
