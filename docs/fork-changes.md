# 与上游 XTLS/Xray-core 的差异

本仓库是 XTLS/Xray-core 的下游分支。协议、线路格式、分享链接与上游完全一致，
可以和上游节点互通；差别只有三处：**部分配置块被合并/搬家**、**新增 `xray lookup` 命令**、
**若干行为修复**。升级前只需改配置。

## 1. 破坏性配置变更（必须迁移）

| 旧写法 | 现状 | 新写法 |
|---|---|---|
| `"stats": {}` | 删除 | 不需要：有 `api` 块就自动统计 |
| `"metrics": {...}` | 删除 | 端口并入 `api.listen`，同端口提供 gRPC 与 `/debug/vars`、`/debug/pprof` |
| `"observatory": {...}` | 删除 | `api.observatory` |
| `"burstObservatory": {...}` | 删除 | `api.burstObservatory` |
| 顶层 `"fakeDns": {...}` | 删除 | `dns.fakeDns` |
| `"reverse": {...}` | 启动报错（与上游一致） | 用 VLESS Reverse Proxy |

**注意**：`stats` / `metrics` / `observatory` / `burstObservatory` / `fakeDns` 这几块现在
只打一行 warning 然后忽略，不会让启动失败。迁移前它们等于静默失效——fake DNS 失效会导致
域名被真实解析、分流规则按真实 IP 命中。

`api` 块本身也放宽了：`tag` 和 `listen` 至少填一个（以前必须填 `tag`）。
`listen` 仍支持 `127.0.0.1:10085`、`/path/to.sock`、`@abstract`。
`services` 里的 `StatsService` 可以不写，写了也不重复。

迁移示例：

```json
{
  "api": {
    "tag": "api",
    "listen": "127.0.0.1:10085",
    "services": ["HandlerService", "LoggerService", "RoutingService"],
    "observatory": { "subjectSelector": ["proxy"], "probeUrl": "https://www.google.com/generate_204" },
    "burstObservatory": { "subjectSelector": ["proxy"], "pingConfig": { "destination": "https://www.google.com/generate_204" } }
  },
  "dns": {
    "servers": ["fakedns", "1.1.1.1"],
    "fakeDns": {
      "pools": [
        { "ipPool": "198.18.0.0/15", "poolSize": 32768 },
        { "ipPool": "fc00::/18", "poolSize": 32768 }
      ]
    }
  }
}
```

`/debug/vars` 与 pprof 与 gRPC 共用同一个端口：HTTP/1.1 走调试端点，
`Content-Type: application/grpc` 的 HTTP/2 请求走 gRPC，控制端不用改。

## 2. 新增命令：`xray lookup`

不启动任何入站，只回答"这个域名/IP 会走哪条规则、哪个出站、哪个 DNS"。

```bash
xray lookup -c config.json example.com
xray lookup -c config.json -port 80 1.1.1.1
xray lookup -c config.json -network udp example.com:53
xray lookup -c config.json -inbound socks -protocol http example.com
xray lookup -c config.json -resolve example.com        # 顺带真的查一次 DNS
xray lookup -c config.json -json -f domains.txt        # 一行一个结果，便于脚本
```

输出包含：命中规则编号与条件、balancer 与其成员、出站（含 "tag 不存在 = 连接被关闭"、
"没匹配则走默认出站"）、freedom finalRules 的最终判决、DNS 命中规则与查询顺序、
静态 hosts 覆盖，以及 `note:` 提示（哪些规则 lookup 没法模拟：process / localOS /
sourcePort / localPort / vlessRoute；routing.script 或 dns.script 存在时结果可能不适用；
入站嗅探会在路由前改写目标）。

与真实运行一致的部分：配置加载方式（`-c` / `-confdir` / `-format` 照旧）、
规则求值、balancer 选择、freedom 隐式 finalRules。`-json` 模式把日志赶到 stderr，
stdout 只有 JSON。

## 3. TUN（仅 Android 行为变化）

- 没设 `xray.tun.fd` 时**自己创建设备**（`/dev/tun`，退回 `/dev/net/tun`），
  并设置 MTU、地址、link up、路由、ip rule；关闭时反向清理。
  以前这种情况会把 fd=0（stdin）当 TUN 用，等于不可用。
- 设了 `xray.tun.fd` 时行为照旧，但启动时校验：设备必须是 `IFF_NO_PI` 且不带
  `IFF_VNET_HDR`，否则立即报错（以前是静默收不到包）。
- 出站接口绑定会在默认路由被系统切换后自动重绑（换网、切换 Wi-Fi/蜂窝），
  解析不到时每秒重查一次；TUN 关闭后不再拦截进程内其它出站拨号。
- Windows / Darwin / FreeBSD / Linux 的 TUN 实现无变化。

## 4. DNS

**新增 DNS over TLS**：

```json
{ "address": "tls://1.1.1.1" }          // 走当前出站/路由
{ "address": "tls+local://8.8.8.8" }    // 直连，绕过路由
```

默认端口 853，SNI 取 URL 主机名（IP 字面量不发 SNI，改验证 IP SAN）。这类服务器在
路由规则里算 `tls` 而不是 `dns`，写 `"protocol": ["dns"]` 不会误伤它。

**fakeDNS 修正**：

- 域名按大小写不敏感归一（RFC 4343），`Example.COM` 与 `example.com` 拿同一个假 IP。
- 不再分配池内前 4 个地址和最后一个地址（网络地址 / 广播地址），
  TUN 自身地址、TUN 的 DNS 地址通常正落在池首，以前会发出连不通的假 IP。
- 池太小（掩码位数剩余 < 3）、`poolSize` 大于池容量时直接启动报错，不再跑起来才死循环。
- fake DNS 池里包含 TUN 网关或 TUN 的 DNS 地址时启动报错，提示改池或改网关。
- TUN 没配 IPv6 网关时不发 AAAA 假 IP（发了也没路由）。
- 只启用了一个地址族却没给该族 pool 时，返回空答案而**不再回落到真实解析**——
  以前会静默绕过 fake IP 分流。

**抗污染/规范加固（无配置变化）**：DNS 应答必须匹配请求的 ID、问题名、类型、class、
opcode，否则丢弃；DoH 不跟随重定向（RFC 8484）；DoH 按 `Age` 头扣减 TTL；
DoQ 按声明的长度读取应答，解析失败不再把半截数据当结果。日志里 DNS 决策的
域名/IP 规则改成可直接粘回配置的格式（`domain:xxx`、`geoip:cn`、`!ip:...`）。

## 5. 嗅探

- 新增 **DNS**、**STUN** 识别。它们只用于路由匹配，不能写进 `destOverride`
  （`destOverride` 仍只接受 `http` / `tls` / `quic` / `fakedns`）。
  典型用法，拦掉内网 DNS 泄漏：
  ```json
  { "type": "field", "protocol": ["dns"], "outboundTag": "block" }
  ```
- HTTP 嗅探：支持 `PATCH` / `TRACE`；按 RFC 9112 从 absolute-form 目标或 `CONNECT`
  目标取 authority，且 scheme 必须是合法 URI scheme——路径里带
  `/r?u=http://other.com/` 不再能劫持目标；`\r\n\r\n` 结束头部时不再把手
  读进请求体。

## 6. 传输与性能

- VLESS / VMess 入站**先把响应头 flush 出去**，再等下行数据。客户端"等头再发 body"时
  不会互等。
- 同一批 UDP 帧合并进一个 buffer 再加密（VLESS 多长度帧、XUDP 各一处），
  少发一批 TLS record（每条 29 字节 + 一次系统调用）。不跨 WriteMultiBuffer 调用攒帧。
- XTLS Vision 的写路径加锁，mux 多会话并发写不再互相破坏 state；
  `IsCompleteRecord` 支持一条 TLS 记录跨多个 buffer（以前直接判 false，失去 splice）；
  寻找 TLS 记录头改用反向扫描，大 buffer 上快约 4 倍。
- ECH / DoH 的 `X-Padding` 自适应：对方 `SETTINGS_MAX_HEADER_LIST_SIZE` 太小时自动
  去掉 padding 重试一次；整次查询共用 30 秒总超时（不退化成两次 30 秒）。
- HTTPUpgrade 按 token 解析 `Connection` / `Upgrade`（`keep-alive, Upgrade` 现在能识别）。
- SOCKS5 客户端校验应答版本字节；SOCKS4 的 user id / 域名字符串上限显式化为
  `buf.Size-1`（行为不变，只是不再随缓冲区大小漂移）。
- REALITY 入站启动后会把目标探测结果打进日志：探测中/失败/收集不到 post-handshake
  记录都会明确 warning，不再像上游那样静默退回普通 Go TLS 行为。

## 7. 构建与发布

- CI 上传的构建产物名带 7 位短 commit（`Xray-linux-64-1f4c389`），目录里能直接区分
  是哪次提交构建的。
- `go.mod` 依赖与上游一致，没有额外依赖。

## 8. 升级步骤

1. 按第 1 节把 `stats` / `metrics` / `observatory` / `fakeDns` 迁进 `api` 与 `dns`。
2. `xray run -test -c config.json` 看有没有那几行 removal warning。
3. `xray lookup -c config.json <常用域名>` 对比迁移前后的出站是否符合预期。
4. 换二进制。
