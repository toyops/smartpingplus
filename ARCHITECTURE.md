# SmartPingPlus 架构文档

> SmartPingPlus 是基于原版 [SmartPing](https://github.com/iquanzhan/smartping) 重构的**网络链路质量监控工具**：
> Go 单体后端 + React SPA 前端，前端产物经 `go:embed` 嵌入二进制，实现**单文件自包含部署**。
> 持续探测 ICMP 延迟/丢包与 TCP 端口可达性，提供拓扑图、全国延迟热力图、告警记录与在线诊断工具。

---

## 1. 总体架构

```
                        ┌─────────────────────────────────────────────┐
                        │              浏览器 (React SPA)              │
                        │  Dashboard / Topology / Mapping / Tools /   │
                        │  Alerts / Config   (hash 路由, 按页懒加载)    │
                        └──────────────────┬──────────────────────────┘
                                           │ /api/*.json (fetch)
┌──────────────────────────────────────────▼──────────────────────────────────┐
│                          smartpingplus (Go 单二进制)                         │
│                                                                             │
│  ┌───────────────┐   ┌────────────────┐   ┌───────────────────────────┐    │
│  │ src/smartping.go│ │  src/http      │   │  cron 调度 (jakecoffman)   │    │
│  │ 入口: 加载配置 → │ │  api.go        │   │  每 60s: Ping/TcpPing/     │    │
│  │ 启动 cron →    │ │  index.go      │   │          Mapping           │    │
│  │ 启动 HTTP      │ │  static.go     │   │  每小时: ClearArchive      │    │
│  └───────┬───────┘   │  (go:embed 前端)│   └────────────┬──────────────┘    │
│          │           └────────┬───────┘                │                   │
│          ▼                    │                        ▼                   │
│  ┌───────────────┐            │              ┌──────────────────┐          │
│  │ src/g         │            │              │ src/funcs        │          │
│  │ 配置解析/落盘   │◄───────────┘              │ ping.go    探测+入库        │
│  │ SQLite 连接    │                           │ tcpping.go 探测+入库        │
│  │ 全局状态/鉴权表 │                           │ mapping.go 运营商地图探测    │
│  └───────┬───────┘                           │ alert.go   告警判定+邮件     │
│          │                                   │ archive.go 数据滚动清理      │
│          │                                   │ cloud.go   云端受控模式      │
│          │                                   └────────┬─────────┘          │
│          ▼                                            ▼                    │
│  ┌──────────────────────────────┐        ┌──────────────────────────┐      │
│  │ SQLite (modernc.org/sqlite)  │        │ src/nettools             │      │
│  │ db/database.db               │        │ ping.go     原生 raw ICMP  │      │
│  │ pinglog / tcppinglog /       │◄───────│ cmdping.go  系统 ping 回退 │      │
│  │ mappinglog / alertlog        │        │ tcpping.go  TCP 握手测速   │      │
│  └──────────────────────────────┘        │ mtr.go      MTR 路由追踪   │      │
│                                          └──────────────────────────┘      │
└─────────────────────────────────────────────────────────────────────────────┘
```

**核心思路**：一个 Go 进程内同时承担三个角色——

1. **采集器**：cron 定时并发探测所有目标，结果按分钟粒度写入 SQLite；
2. **Web 服务**：标准库 `net/http` 提供 JSON API + 内嵌前端静态资源；
3. **告警引擎**：基于滑动窗口阈值判定目标异常，边沿触发邮件告警（附 MTR 路由信息）。

---

## 2. 技术栈

| 层 | 技术 | 说明 |
|----|------|------|
| 后端语言 | Go 1.25 | 无 Web 框架，标准 `net/http` + `DefaultServeMux` |
| 数据库 | `modernc.org/sqlite` | **纯 Go 实现，无 CGO**，便于交叉编译 |
| 定时调度 | `github.com/jakecoffman/cron` | 6 字段表达式（含秒） |
| 日志 | `github.com/cihub/seelog` | 由 `conf/seelog.xml` 配置，输出 `logs/` |
| ICMP | `golang.org/x/net/icmp` | raw socket，不可用时回退系统 `ping` 命令 |
| 服务端绘图 | `github.com/wcharczuk/go-chart` | `/api/graph.png` 生成延迟 PNG（供外链嵌入） |
| 配置 | `github.com/BurntSushi/toml` | TOML 解析与编码写回 |
| 前端 | React 19 + Vite 7 + TypeScript | SPA，hash 路由 |
| UI | Tailwind CSS 4 + Radix UI + lucide-react | shadcn 风格组件（`web/src/components/ui/`） |
| 图表 | recharts | 延迟面积图（绿色为主、红色表丢包） |

---

## 3. 目录结构

```
smartpingplus/
├── conf/                        # 运行时配置（可变，需持久化）
│   ├── config.toml             # 通用配置：端口/节点名/密码/告警邮箱/监控模式/全国地图目标
│   ├── config-base.toml        # 通用配置出厂模板（config.toml 缺失时自动拷贝）
│   ├── targets.toml            # 抓取配置：[PingTargets] [TcpTargets] [[AlertRules]]
│   └── seelog.xml              # 日志配置
├── db/
│   ├── database.db             # 运行时 SQLite（自动生成）
│   └── database-base.db        # 出厂空库模板
├── src/                         # Go 后端源码
│   ├── smartping.go            # 程序入口（main，flag -v 显示版本）
│   ├── g/                      # 全局层：配置解析/落盘、DB 连接、全局状态、鉴权表
│   │   ├── config.go
│   │   └── struct.go           # Config/TargetsConfig/PingSt/AlertLog 等结构体
│   ├── funcs/                  # 业务层：探测编排、入库、告警、归档、云端模式
│   │   ├── ping.go / tcpping.go / mapping.go
│   │   ├── alert.go / archive.go / cloud.go
│   ├── nettools/               # 探测层：底层网络探测原语
│   │   ├── ping.go             # 原生 raw ICMP
│   │   ├── cmdping.go          # 系统 ping 命令回退 + IcmpAvailable 探测
│   │   ├── tcpping.go          # TCP 握手计时（RunTcpPing/ParseTcpTarget）
│   │   └── mtr.go              # MTR 逐跳统计
│   └── http/                   # HTTP 层
│       ├── http.go             # StartHttp / 鉴权函数 / RenderJson / IP 校验
│       ├── api.go              # 全部 /api/*.json 路由与处理逻辑
│       ├── index.go            # "/" 静态资源入口
│       ├── static.go           # go:embed 嵌入 static/（前端产物）
│       └── static/             # 前端 vite build 输出（构建产物，勿手改）
├── web/                         # React 前端源码
│   ├── vite.config.ts          # 构建输出 → ../src/http/static；dev 代理 /api → :8899
│   └── src/
│       ├── App.tsx             # hash 路由 + 按页 lazy 加载
│       ├── lib/api.ts          # 类型化 API 封装（含 proxy 多节点取数）
│       └── components/         # 8 个页面组件 + ui/ 基础组件
├── bin/                         # 编译产物（smartpingplus 可执行文件、Windows nssm）
├── dist/                        # 发布包模板（Dockerfile + deploy.sh + 二进制 + 配置）
├── control / control.cmd        # Linux/Windows 控制/构建脚本
├── alert.mp3                    # 告警提示音（运行时磁盘读取，不嵌入二进制）
└── logs/                        # seelog 日志输出
```

---

## 4. 后端架构

### 4.1 启动流程（`src/smartping.go`）

```
main()
 ├─ flag -v → 打印版本退出
 ├─ g.ParseConfig(ver)
 │   ├─ Root = 可执行文件所在目录的上一级（部署布局假定 bin/ 在根目录下）
 │   ├─ conf/config.toml 缺失 → 从 config-base.toml 拷贝生成
 │   ├─ 初始化 seelog（conf/seelog.xml）
 │   ├─ 读取 config.toml → g.Cfg（Name 缺省取主机名，Addr 缺省 127.0.0.1）
 │   ├─ loadTargets() 加载抓取配置（见 §6 配置体系）
 │   ├─ db/database.db 缺失 → 从 database-base.db 拷贝生成
 │   ├─ sql.Open("sqlite", ...)，SetMaxOpenConns(1)
 │   ├─ initDbSchema()：CREATE TABLE IF NOT EXISTS tcppinglog（+索引），兼容旧库升级
 │   └─ saveAuth()：重建用户白名单 / Agent 授权两张 map
 ├─ go funcs.ClearArchive()      # 启动即清理一次过期数据
 ├─ cron 调度
 │   ├─ "*/60 * * * * *"（每 60 秒）
 │   │    ├─ go funcs.Ping()
 │   │    ├─ go funcs.TcpPing()
 │   │    ├─ go funcs.Mapping()
 │   │    └─ cloud 模式时：go funcs.StartCloudMonitor()
 │   └─ "0 0 * * * *"（每小时整点）→ funcs.ClearArchive()
 └─ http.StartHttp()             # 阻塞监听 :Port（默认 8899）
```

### 4.2 模块职责

| 包 | 职责 | 关键点 |
|----|------|--------|
| `src/g` | 配置、DB、全局状态 | `Cfg`（配置单例）、`Db`、`AlertStatus`（告警状态机）、`AuthUserIpMap`/`AuthAgentIpMap`（鉴权表）、`ToolLimit`（工具限流表）、`DLock`（DB 写互斥） |
| `src/funcs` | 业务编排 | 探测一轮 → 聚合 → 写库 → 触发告警判定 |
| `src/nettools` | 探测原语 | 与业务无关的 ping/tcpping/mtr 实现 |
| `src/http` | API + 静态资源 | 标准库路由，无中间件框架 |

### 4.3 探测流水线

**ICMP Ping（`funcs/ping.go`）**——每 60 秒一轮：

1. 为每个 `PingTargets` 目标启动一个 goroutine（`sync.WaitGroup` 汇合）；
2. 每目标连续探测 **20 次**，每次以 3 秒为节拍（`sleep(3s - 探测耗时)` 补齐）；
3. 每次探测：优先原生 raw ICMP（`nettools.RunPing`）；`IcmpAvailable()` 为 false 时回退系统 `ping` 命令（`nettools.RunCmdPing`）；
4. 聚合 min/max/avg 延迟与丢包率，写一行 `pinglog`（`logtime` 为分钟粒度字符串）；
5. 全部目标完成后 `go StartAlert()` 触发告警判定。

**TCP Ping（`funcs/tcpping.go`）**——与 ICMP 完全对称：目标为 `host:port`，每次探测 `net.DialTimeout` 计时握手 RTT，写入 `tcppinglog`（`target` 列保留 `host:port`，同主机多端口可独立追踪）。

**全国地图 Mapping（`funcs/mapping.go`）**：

1. 遍历 `Cfg.Chinamap`（省份 → 运营商(ctcc/cucc/cmcc) → IP 列表），每省份×运营商一个 goroutine；
2. 每个 IP 探测 3 次，聚合时丢弃约 1/4 的全丢样本（抗毛刺），算出该省份×运营商的平均延迟；
3. 整张地图序列化为 JSON，`REPLACE INTO mappinglog`（每分钟一行快照）。

### 4.4 探测层实现（`src/nettools`）

| 文件 | 能力 | 说明 |
|------|------|------|
| `ping.go` | 原生 ICMP | `ip4:icmp` raw socket + TTL 控制，构造/解析 Echo 报文；socket 创建失败必须回填 Error（否则 0 延迟会被误判为成功） |
| `cmdping.go` | 系统 ping 回退 | `IcmpAvailable()` 用 `sync.Once` 探测一次：**Windows 强制回退**（raw ICMP 对回环/局域网有兼容性问题）；Unix 下尝试创建 raw socket 判断权限。解析输出用「数字+ms」正则，不依赖本地化前缀（规避中文 Windows GBK 编码问题） |
| `tcpping.go` | TCP ping | `RunTcpPing` = DialTimeout 计时；`ParseTcpTarget` 解析 `host:port`（缺端口默认 80） |
| `mtr.go` | MTR 路由追踪 | 逐 TTL 发 ICMP，聚合每跳 Loss/Last/Avg/Best/Wrst/StDev；连续 N 跳超时提前终止 |

### 4.5 告警机制（`funcs/alert.go`）

**判定**（`CheckAlertStatus`）：对每条 `AlertRules` 规则，查询 `pinglog` 中最近 `Thdchecksec` 秒窗口内、满足 `avgdelay > Thdavgdelay || losspk > Thdloss` 的记录数 `cnt`；**`cnt ≤ Thdoccnum`（容忍次数）返回 true = 正常**，超过则返回 false = 异常。

> 注意返回值语义与直觉相反：`true` 表示"正常"。`/api/topology.json` 直接把该值映射为连线 ok（绿/红），前端 `topology.tsx` 以 `=== "true"` 判定连通。

**触发**（`StartAlert`，每轮 Ping/TcpPing 后异步执行）——**边沿触发状态机**，避免持续异常期间重复轰炸：

```
AlertStatus[addr] 仅记录上轮状态（无 key / true / false）
触发条件: (!haskey && 异常) || (异常 && 上轮为正常)
触发动作: RunMtr 采集路由 → 写 alertlog → 异步发告警邮件(附 MTR HTML 表格)
持续异常: 不重复触发；恢复正常会更新状态，再次异常才会重新告警
```

**邮件**：`net/smtp` + PlainAuth，HTML 格式；SMTP 四项配置齐全才发送。

### 4.6 数据存储

SQLite（`db/database.db`），**4 张表**：

| 表 | 粒度 | 列 |
|----|------|-----|
| `pinglog` | 每目标×每分钟 1 行 | logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk |
| `tcppinglog` | 同上（target 为 host:port） | 同上（启动时 `initDbSchema` 补建，带 target+logtime 索引） |
| `mappinglog` | 每分钟 1 行 | logtime, mapjson（整张地图 JSON 快照） |
| `alertlog` | 每次告警 1 行 | logtime, targetip, targetname, tracert（MTR JSON） |

**并发策略**：

- 连接池 `SetMaxOpenConns(1)` —— SQLite 单写者模型，从根上避免 `SQLITE_BUSY`；
- 写操作额外用 `g.DLock` 互斥锁包裹（部分读路径也加锁）。

**数据滚动**（`funcs/archive.go`）：删除 `Base.Archive` 天前的数据；启动时执行一次 + 每小时整点执行。

**时间处理约定**：所有 `logtime` 为 `YYYY-MM-DD HH:MM` 格式字符串；查询接口把入参时间减 8 小时折算（硬编码东八区），Docker 镜像也设置 `TZ=Asia/Shanghai`。

### 4.7 HTTP API（`src/http/api.go`）

所有 API 前置两级鉴权（不通过返回 401）：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/config.json` | GET | 当前完整配置；`Password` 恒脱敏，非 Agent 来源时 SMTP 密码用 `samepasswordasbefore` 占位 |
| `/api/ping.json?ip=&starttime=&endtime=` | GET | ICMP 目标时序；按分钟生成连续时间轴并把查到的行对齐填入（缺失补 0），默认最近 2 小时 |
| `/api/tcpping.json?ip=&starttime=&endtime=` | GET | 同上，查 `tcppinglog` |
| `/api/topology.json` | GET | `{目标Addr: "true"/"false"}` 告警状态（true=正常） |
| `/api/alert.json?date=` | GET | 返回 `[有告警的日期列表, 当日告警数组]` 二元 JSON |
| `/api/mapping.json?d=` | GET | 指定分钟的地图快照（默认 1 分钟前） |
| `/api/tools.json?t=&mode=&port=` | GET | 在线诊断：5 次探测聚合；`mode=tcp` 走 TCP 握手；受 `Toollimit` 秒/IP 限流 |
| `/api/saveconfig.json` | POST | 需 `password` 参数校验；对节点名/IP/Base/Topology/目标合法性/告警规则阈值逐项校验后整体替换 `g.Cfg` 并拆分写回两个 TOML |
| `/api/sendmailtest.json` | GET | 发送测试告警邮件 |
| `/api/graph.png?g=<api-url>` | GET | 服务端取数后用 go-chart 画 300×130 延迟/丢包 PNG（供旧版外链嵌入） |
| `/api/proxy.json?g=<url>&t=<timeout>` | GET | **服务端代理取数**：多节点部署时，前端经本机代理访问其他节点的 API |
| `/` | GET | 内嵌前端静态资源（`go:embed`）；`/alert.mp3` 例外，从磁盘读取 |

**鉴权模型**（`http.go`）：

- `AuthUserIp`：`Authiplist`（逗号分隔 IP 白名单）——**为空时全放行**；
- `AuthAgentIp`：`AuthAgentIpMap` = 所有检测目标的主机 IP 集合，用于"互 PING 的对端节点"身份认定（多节点组网时互相拉数据免白名单）；
- 大多数 API 的判定为 `AuthUserIp || AuthAgentIp(drt=true)`（两者任一通过）。

### 4.8 运行模式

| 模式 | 行为 |
|------|------|
| `local`（默认） | 单机独立采集与展示 |
| `cloud` | 每分钟 `SaveCloudConfig(Endpoint)`：GET 中心端的 `/api/config.json`，用返回值**覆盖本地配置**（保留本机 Name/Addr/Ver/Port/Password/Endpoint/Type），记录 `LastSuccTime`，再 `SaveConfig()` 落盘——即"受控节点"从中心端同步抓取目标 |

---

## 5. 前端架构（`web/`）

### 5.1 构建链路与嵌入

```
web (Vite + React + TS)
  npm run build ──► ../src/http/static   (vite.config.ts outDir, emptyOutDir)
                        │
                        ▼
              src/http/static.go: //go:embed static
                        │
                        ▼
              go build ──► 单二进制（自包含前端）
```

- 修改前端后必须 `npm run build` **再重新编译 Go**（embed 在编译期生效）；
- 开发模式：`npm run dev`（:5173），`/api` 代理到本地 8899 后端，热更新无需重编译；
- `alert.mp3` 刻意不嵌入（体积 + 可自定义），运行时从可执行文件旁磁盘读取。

### 5.2 路由与页面

`App.tsx` 用 **hash 路由**（`#/ping` 等）+ `React.lazy` 按页代码分割（图表页仅在访问时加载）：

| 路由 | 组件 | 功能 |
|------|------|------|
| `#/ping` | `DashboardPage mode="ping"` | 正向 ICMP：每目标一张卡片（迷你图 + 详情弹窗） |
| `#/tcpping` | `DashboardPage mode="tcpping"` | 正向 TCP 探测面板（同一组件复用） |
| `#/reverse` | `DashboardPage mode="reverse"` | 反向 Ping：数据存于**源节点**，需多节点部署 + proxy 拉取；单机显示空态提示 |
| `#/topology` | `TopologyPage` | 以本机为中心的拓扑；节点 = Ping/Tcp 目标 + 告警规则目标去重，连线状态取 `/api/topology.json` |
| `#/mapping` | `MappingPage` | 三大运营商全国延迟热力图 |
| `#/tools` | `ToolsPage` | 在线 Ping / TCP 诊断 |
| `#/alerts` | `AlertsPage` | 按日期浏览历史告警 |
| `#/config` | `ConfigPage` | 可视化编辑两份 TOML 配置，保存走 `/api/saveconfig.json` |

### 5.3 API 层（`web/src/lib/api.ts`）

- 所有后端响应都有对应 TypeScript interface，单一 `get<T>()` 封装（非 2xx 抛错）；
- **多节点取数统一走 `/api/proxy.json`**：`fetchXxx(..., proxy)` 传入源节点地址时，请求会被改写为 `proxy.json?g=http://<node>/api/xxx.json`，由服务端代理转发（规避浏览器跨域）。

---

## 6. 配置体系（三文件模型）

```
                   ┌──────────────────────┐
                   │  内存 g.Cfg (Config)  │   唯一事实来源（API 契约 = 内存结构）
                   └──────────┬───────────┘
                 解析时合并 ▲   │ SaveConfig() 拆分落盘
        ┌──────────────────┼──┴───────────────────┐
        ▼                  ▼                      ▼
 conf/config.toml   conf/targets.toml      （config-base.toml /
 通用配置：端口/名称/   抓取配置：              database-base.db 仅作出厂模板，
 密码/模式/Base/       PingTargets/            缺失时自动拷贝生成运行时文件）
 拓扑样式/告警邮箱/     TcpTargets/
 全国地图目标          AlertRules
```

**加载策略**（`g.loadTargets`）：

1. `targets.toml` 存在 → 直接读入 `g.Cfg`；
2. 不存在但 `config.toml` 内嵌了旧版抓取配置 → **迁移**到 `targets.toml`，并重写 `config.toml` 清理旧字段；
3. 都没有 → 使用内置默认目标（AliDNS + 本机）落盘 `targets.toml`。

**拆分实现技巧**：`Config` 结构体中抓取字段带 `toml:"-"` 标签——编码 `config.toml` 时自动跳过这些字段，解码时也不污染，从而让"内存合并结构"与"双文件落盘"共存（`g/struct.go`）。

**写回路径**：前端 Config 页 → `/api/saveconfig.json`（密码校验 + 逐项合法性校验）→ `g.Cfg = nconfig` → `SaveConfig()` → `saveBaseConfig()`（写 config.toml，带说明注释头）+ `SaveTargets()`（写 targets.toml）。

---

## 7. 构建与部署

### 7.1 构建

```bash
# 1. 前端
cd web && npm install && npm run build     # 输出 src/http/static/

# 2. 后端（无 CGO）
CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -o bin/smartpingplus src/smartping.go
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/smartpingplus src/smartping.go
```

- `control`（Linux 脚本）/ `control.cmd`（Windows）：`build|run|start|stop|restart|status|version|pack`；`pack` 打 tar.gz 发布包（排除运行时生成的 config.toml/targets.toml/database.db）；
- Windows 服务化使用 `bin/nssm.exe`。

### 7.2 Docker（`dist/`）

- 基础镜像 `alpine:3.20`，安装 `iputils`（容器内非 root，raw ICMP 不可用，自动回退系统 ping）与 `tzdata`（`TZ=Asia/Shanghai`）；
- `deploy.sh`：`build + run | run | stop | logs`；容器名 `smartpingplus`，`--restart=always`，暴露 8899；
- 持久化挂载：`conf/`、`db/`、`logs/`（升级替换镜像不丢数据）。

### 7.3 部署目录布局（程序对路径的假定）

`g.GetRoot()` 取**可执行文件所在目录的上一级**为项目根，因此标准布局为：

```
<root>/
├── bin/smartpingplus    # 可执行文件
├── conf/  db/  logs/    # 程序按 <root>/ 相对路径访问
└── alert.mp3
```

---

## 8. 关键设计决策与已知注意点

### 设计决策

1. **单二进制自包含**：前端 `go:embed` 嵌入 + 纯 Go SQLite（无 CGO），交叉编译零依赖交付；
2. **配置双文件拆分**：通用配置与抓取配置分离，Web 保存互不污染，且自动完成旧版单文件迁移；
3. **探测能力降级链**：原生 raw ICMP → 系统 ping 命令，Windows 强制走后者（兼容性实测结论），命令输出解析只认「数字+ms」（规避 GBK 本地化）；
4. **SQLite 单连接**：`MaxOpenConns(1)` + 写互斥锁，用吞吐换正确性（本场景写频率低，足够）；
5. **告警边沿触发**：状态机保证"恢复→再异常"才再次告警，持续异常不重复发送；
6. **多节点组网靠 proxy**：前端不直接跨域访问其他节点，统一经本机 `/api/proxy.json` 服务端转发。

### 已知注意点（阅读/二次开发时留意）

1. **SQL 均为字符串拼接**（`api.go`/`funcs/*`），未参数化，查询参数直接进入 SQL 文本；受 IP 白名单 + 目标合法性校验缓解，但属于技术债；
2. **时区硬编码东八区**：查询接口 `-8h` 折算、Docker `TZ=Asia/Shanghai`，非东八区部署会出现时间轴偏移；
3. **采集周期硬编码**：cron 固定每 60 秒一轮（每目标 20 次 × 3s 节拍恰好填满一分钟）；`Base.Refresh` 配置项仅在保存时校验合法性，**并未参与调度**；
4. `CheckAlertStatus` 返回 `true` 表示"正常"（与函数名直觉相反），`topology.json` 与告警状态机均依赖该语义；
5. 归档清理实际是**每小时**执行一次（cron `0 0 * * * *`），而非每天；
6. `net/http` 使用 `DefaultServeMux` 全局注册，API 与静态资源共用鉴权入口，无 middleware 分层。

---

## 9. 一次完整数据流的时序示例

以「用户在 Dashboard 查看某 ICMP 目标最近 2 小时延迟」为例：

```
[每 60s] cron ─► funcs.Ping()
              └► 每目标 goroutine: 20×(raw ICMP 或 系统 ping, 3s 节拍)
                   └► 聚合 → INSERT pinglog (1 行/分钟)
                        └► go StartAlert(): 滑窗阈值判定 → (边沿触发) MTR + alertlog + 邮件

[浏览器] DashboardPage
  └► fetchConfig()            GET /api/config.json        → 拿到 PingTargets 列表
  └► 逐目标 fetchPing(ip)     GET /api/ping.json?ip=...   → 分钟粒度时序数组(缺失补 0)
       └► MiniChart(recharts 面积图) 渲染，点击卡片弹出 DetailDialog 看大图
```
