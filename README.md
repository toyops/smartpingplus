# SmartPingPlus

> 基于 [SmartPing](https://github.com/iquanzhan/smartping) 重构的网络质量监控工具：Go 后端 + React 前端，单二进制自包含部署。

SmartPingPlus 持续监测网络链路质量（ICMP 延迟/丢包、TCP 端口握手 RTT），
以目标卡片、趋势图、拓扑视图、分省延迟对比、告警记录等形式呈现，
并在告警触发时自动执行 MTR 路由取证、发送邮件通知。

## 与原版 SmartPing 的主要差异

- **前后端分离重构**：前端 React 19 + Vite 7 + TypeScript + Tailwind CSS 4 + recharts，
  参考 Beszel 设计语言重新设计 UI，hash 路由 + 按页懒加载；后端保持 Go 单体服务。
- **TCP 端口探测**：新增 `TcpTargets` 目标类型（`host:port` TCP 握手 RTT），
  独立数据表、独立 API 与专属页面，与 ICMP 探测完全对称。
- **管理员登录体系**：bcrypt 密码哈希（兼容旧明文并自动升级）+ JWT（HS256）会话，
  HttpOnly Cookie / Bearer / query 参数三种携带方式，7 天有效期；系统配置页需登录后访问。
- **告警自动 MTR 取证**：目标状态翻转为异常时自动对该地址执行 MTR（逐跳丢包/延迟统计），
  结果存入告警记录并渲染进告警邮件。
- **配置拆分**：通用配置（`config.toml`）与抓取配置（`targets.toml`）分离，互不污染；
  旧版内嵌在 config.toml 中的目标配置可自动迁移。
- **单二进制交付**：前端构建产物经 `go:embed` 嵌入 Go 二进制，部署无需携带静态目录；
  仅 `alert.mp3` 告警声音保留在磁盘，便于自定义替换。
- **纯 Go SQLite**：`modernc.org/sqlite`，无需 CGO，交叉编译友好（Linux amd64 / Windows / macOS arm64）；
  WAL 模式 + 单连接 + 全参数化 SQL，避免锁冲突与注入。
- **无特权运行**：raw ICMP 不可用时（Windows / 非 root）自动回退系统 `ping` 命令，
  普通用户权限即可运行（Docker 镜像内置 iputils 即为此准备）。
- **安全加固**：跨节点代理带 SSRF 白名单校验（拦截云元数据、内网地址、非法协议），
  统一安全响应头，SQL 参数化，优雅停机。

## 特性

- **多目标探测**：ICMP Ping（`PingTargets`）与 TCP 端口握手（`TcpTargets`）两类检测目标，
  TCP 目标支持域名与同主机多端口分别统计。
- **目标卡片与趋势图**：每目标一张卡片（状态点 + 迷你时序图 + 延迟/丢包摘要），
  点击弹出详情图，支持 Grafana 风格时间范围选择（快捷范围、`now-1h` 表达式、日历、最近使用）。
- **拓扑视图**：以本机为中心的环形拓扑，节点与连线按告警状态高亮。
- **全国延迟对比**：对三大运营商（电信/联通/移动）各省份探测点采集延迟，分省条形对比。
- **告警**：阈值规则（延迟/丢包/判定周期/连续次数）触发邮件告警 + 浏览器声音告警（`alert.mp3` 可从磁盘替换）；
  触发时自动 MTR 取证。
- **在线诊断工具**：对指定节点发起临时 ICMP / TCP 检测，支持按 IP 限频。
- **历史归档**：探测数据写入 SQLite 并按配置天数滚动清理。
- **多节点**：支持 local / cloud 模式；多 agent 部署时可经中心端代理查看反向链路与跨节点数据。

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.25、标准 `net/http`、`modernc.org/sqlite`、`golang.org/x/crypto`（bcrypt）、`seelog` 日志、`jakecoffman/cron` 秒级定时、`go-chart` 服务端绘图、`golang.org/x/net`（raw ICMP） |
| 前端 | React 19、Vite 7、TypeScript 5.9、Tailwind CSS 4、Radix UI、recharts、lucide-react |
| 打包 | `go:embed` 将前端产物嵌入二进制；`alert.mp3` 例外，运行时从磁盘读取 |
| 部署 | Docker（alpine 3.20 + iputils + tzdata）、`control` / `control.cmd` 脚本、nssm Windows 服务 |

## 目录结构

```
smartpingplus/
├── conf/                      # 配置文件
│   ├── config.toml           # 通用配置（服务参数），Web「保存配置」写回
│   ├── config-base.toml      # 通用配置出厂模板（config.toml 缺失时自动拷贝）
│   ├── targets.toml          # 抓取配置：检测目标与告警规则
│   └── seelog.xml            # 日志配置
├── db/                       # SQLite 数据库
│   ├── database.db           # 运行时数据（首启自动生成）
│   └── database-base.db      # 出厂空库模板
├── src/                      # Go 后端源码
│   ├── smartping.go          # 入口：cron 调度、优雅停机
│   ├── g/                    # 配置解析、数据库初始化、全局状态
│   ├── funcs/                # ping / tcpping / mapping / alert / archive 业务逻辑
│   ├── nettools/             # 底层探测原语：raw ICMP / 命令行 ping 回退 / TCP 握手 / MTR
│   └── http/                 # 路由、API、认证、SSRF 校验、内嵌静态资源
│       └── static/           # 前端构建产物（go:embed 嵌入）
├── web/                      # React + Vite 前端源码
│   └── src/components/       # 各页面组件
├── bin/                      # 编译产物
├── dist/                     # Linux 发布包（Dockerfile、deploy.sh、二进制）
├── control / control.cmd     # Linux / Windows 控制脚本（build/run/start/stop/...）
└── alert.mp3                 # 告警声音（不嵌入二进制，可自由替换）
```

## 快速开始（从源码构建）

环境要求：Go ≥ 1.25、Node.js ≥ 20.19（Vite 7 要求）。

### 1. 构建前端

```bash
cd web
npm install
npm run build      # 输出到 ../src/http/static（被 Go 后端嵌入）
```

### 2. 构建后端

```bash
# Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/smartpingplus src/smartping.go

# macOS arm64
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/smartpingplus src/smartping.go

# 或使用脚本：bash control build（Linux）/ control.cmd build（Windows）
```

### 3. 运行

```bash
./bin/smartpingplus          # 默认监听 8899 端口
# 或
bash control start           # 后台启动（写入 var/app.pid）
```

首次启动时若 `conf/config.toml`、`conf/targets.toml` 或 `db/database.db` 缺失，
会自动从模板生成（检测目标默认 AliDNS + 本机）。浏览器访问 `http://<本机IP>:8899`。

> **默认管理员密码为 `smartping`**（见 `config.toml` 的 `Password`），首次登录后请在
> 「系统配置」页修改。保存配置时明文密码会自动升级为 bcrypt 哈希。

> raw ICMP 需要 root / CAP_NET_RAW 权限；无特权时自动回退系统 `ping` 命令，功能不受影响。

## 配置说明

| 文件 | 作用 | 写回方 |
|------|------|--------|
| `config.toml` | 通用配置（端口、节点名、采集频率、拓扑样式、告警邮箱、监控模式等） | Web「保存配置」 |
| `config-base.toml` | 通用配置出厂模板，`config.toml` 缺失时自动复制 | 手动 |
| `targets.toml` | 抓取配置：`[PingTargets]`、`[TcpTargets]`、告警规则 `[[AlertRules]]` | Web「保存配置」 |

主要通用配置项（`config.toml`）：

```toml
Ver = "0.9.1"
Port = 8899
Name = "本机"            # 本节点显示名称
Addr = "127.0.0.1"       # 本节点 IP
Password = "smartping"   # 管理员密码（保存时自动 bcrypt 哈希）
Toollimit = 0            # 诊断工具频率限制（秒），0 不限
Authiplist = ""          # 用户 IP 白名单（逗号分隔，空 = 所有 IP 可访问）

[Mode]
  Type = "local"         # local | cloud（cloud 时由 Endpoint 中心端统一下发配置）
  Endpoint = ""

[Base]
  Archive = 10           # 数据归档天数
  Refresh = 1            # 页面刷新间隔（分钟）
  Timeout = 5            # 单次探测超时（秒）

[Topology]
  Tline = "1"            # 拓扑连线粗细
  Tsound = "/alert.mp3"  # 告警声音路径
  Tsymbolsize = "70"     # 拓扑节点大小

[Alert]
  EmailHost = ""         # 告警邮件 SMTP 服务器
  RevcEmailList = ""     # 收件邮箱列表（分号分隔）
  SendEmailAccount = ""  # 发件账号
  SendEmailPassword = "" # 发件密码

[Chinamap]               # 全国延迟探测目标矩阵：省份 × 运营商(cmcc/ctcc/cucc) × IP
```

抓取配置示例（`targets.toml`）：

```toml
[PingTargets]
  "119.29.29.29" = "AliDNS"
  "127.0.0.1" = "本机"

[TcpTargets]
  "127.0.0.1:8899" = "本机"
  "cloud.tencent.com:443" = "腾讯云"   # 支持域名，缺省端口 80

[[AlertRules]]            # 触发告警的阈值规则
  Addr = "119.29.29.29"
  Name = "AliDNS"
  Thdavgdelay = "200"     # 平均延迟阈值(ms)
  Thdchecksec = "900"     # 判定周期(秒)
  Thdloss = "30"          # 丢包率阈值(%)
  Thdoccnum = "3"         # 连续触发次数
```

## Web 控制台

| 页面 | 说明 |
|------|------|
| 正向 Ping | 本机 → 各 ICMP 目标的实时卡片墙与趋势图 |
| 正向 TcpPing | 本机 → 各 TCP 目标（host:port）的握手延迟/失败率 |
| 反向 Ping | 其他节点 → 本机（需多 agent 部署，经中心端代理取数） |
| Ping 拓扑 | 以本机为中心的环形拓扑，异常链路红色高亮 |
| 全国延迟 | 电信/联通/移动分省延迟条形对比 |
| 检测工具 | 对指定节点临时发起 ICMP / TCP 检测 |
| 报警记录 | 按日期归档的告警历史 |
| 系统配置 | 🔒 需登录：通用配置、Ping/TCP 目标、告警规则、邮箱、密码 |

图表交互：点击任意目标卡片弹出详情图，支持 Grafana 风格时间范围选择
（30m ~ 90d 快捷范围、`now-1h` 类表达式、日历选择、最近使用记录）。
视觉语义：延迟走势以绿色面积图为主，红色仅用于丢包异常。

## 安全机制

- **三层访问控制**：用户 IP 白名单（`Authiplist`，空 = 放行）→ 检测目标 IP 授权（agent 互访）
  → 管理员密码 / JWT（修改配置等写操作）。
- **密码存储**：bcrypt 哈希，保存配置时旧明文自动升级。
- **JWT 会话**：HS256 签名，密钥持久化于 `conf/.jwt_secret`（0600 权限），
  Token 经 HttpOnly Cookie 下发，7 天有效。
- **SSRF 防护**：`/api/proxy.json` 仅允许代理白名单内 agent 的白名单 API 路径，
  拦截云元数据地址（169.254.169.254）、link-local、非法协议与重定向。
- **SQL 注入防御**：全部查询参数化，表名白名单校验。
- **安全响应头**：`X-Content-Type-Options` / `X-Frame-Options` / `X-XSS-Protection` / `Referrer-Policy`。

## HTTP API

读接口受 IP 白名单保护，写接口额外要求 JWT 或密码。

| 端点 | 说明 |
|------|------|
| `/api/config.json` | 读取当前配置（密码脱敏） |
| `/api/ping.json?ip=<addr>&starttime=&endtime=` | 某 ICMP 目标的延迟/丢包时序 |
| `/api/tcpping.json?ip=<host:port>&starttime=&endtime=` | 某 TCP 目标的延迟/丢包时序 |
| `/api/topology.json` | 各目标告警状态（true/false） |
| `/api/alert.json?date=` | 告警记录（按日期，含 MTR 取证数据） |
| `/api/mapping.json?d=` | 全国延迟数据 |
| `/api/tools.json?t=<host>&mode=tcp&port=` | 在线诊断（ICMP / TCP） |
| `/api/login.json` | 管理员登录（POST 密码，签发 JWT Cookie） |
| `/api/auth/status.json` | 查询当前认证状态 |
| `/api/logout.json` | 登出（清除 Cookie） |
| `/api/saveconfig.json` | 保存配置（需 JWT 或密码） |
| `/api/sendmailtest.json` | 发送测试告警邮件 |
| `/api/graph.png?g=<api-url>` | 服务端渲染延迟/丢包 PNG（供外链嵌入） |
| `/api/proxy.json?g=<url>` | 跨节点代理取数（SSRF 白名单校验） |

## 部署（Docker）

发布包在 `dist/`，含 `Dockerfile`、`deploy.sh`、Linux 二进制与配置模板。

```bash
# 1. 将 dist/ 上传到 Linux 服务器
# 2. 构建并运行容器
cd dist
bash deploy.sh              # build + run（默认）
# bash deploy.sh run        # 仅运行（镜像已存在）
# bash deploy.sh stop       # 停止并删除容器
# bash deploy.sh logs       # 跟随日志
```

- 默认监听 **8899** 端口，容器名 `smartpingplus`，`--restart=always`。
- 持久化目录：`conf/`、`db/`、`logs/` 挂载到宿主机，升级时保留数据。
- 镜像基于 `alpine:3.20`，安装 `iputils`（ICMP 回退）与 `tzdata`（Asia/Shanghai）。
- Windows 可通过 `control.cmd` 配合 nssm 注册为系统服务。

## 数据持久化

- 探测数据：`db/database.db`（SQLite，WAL 模式），按 `Base.Archive` 天滚动清理。
- 表：`pinglog` / `tcppinglog` / `mappinglog` / `alertlog`（含 MTR 取证 JSON）。
- 日志：`logs/`（由 `seelog.xml` 定义，按级别滚动）。
- 升级时只需替换二进制（前端已内嵌），保留 `conf/`、`db/`、`logs/` 即可。

## 开发模式

前端开发服务器（热更新）默认代理 `/api` 到本地 8899 后端：

```bash
cd web
npm install
npm run dev        # http://localhost:5173，代理 /api -> http://127.0.0.1:8899
```

需先在本地启动后端（`./bin/smartpingplus` 或 `bash control run`）。
修改前端后执行 `npm run build`，再重新编译 Go 二进制（`go:embed` 重新嵌入）。

运行测试（认证、SSRF 校验、SQL 注入防御、并发安全、存储层）：

```bash
go test ./src/...
```

## 持续集成与发布

仓库使用 GitHub Actions（`.github/workflows/`）：

- **CI**（`ci.yml`）：push / PR 时自动构建前端、编译并测试后端。
- **Release**（`release.yml`）：推送 `v*` 标签时自动构建前端 + 三平台后端
  （Linux amd64 / Windows amd64 / macOS arm64），打包为 tar.gz / zip
  （含配置模板、出厂数据库、`alert.mp3`、管理脚本），附 SHA256 校验和，
  并创建 GitHub Release（自动生成变更说明）：

  ```bash
  git tag v0.9.1 && git push origin v0.9.1   # 即可触发发布
  ```

## 许可证

遵循原 SmartPing 项目的许可证，详见 `LICENSE`。
