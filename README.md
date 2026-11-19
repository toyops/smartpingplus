# SmartPingPlus

> 基于 [SmartPing](https://github.com/iquanzhan/smartping) 重构的网络质量监控工具：Go 后端 + React 前端，单二进制自包含部署。

SmartPingPlus 用于持续监测网络链路质量（ICMP 延迟/丢包、TCP 端口可达性），
并以拓扑图、全国延迟热力图、告警记录等形式直观呈现。相比原版 SmartPing，
本项目的改进重点是：

- **前后端分离重构**：前端使用 React 19 + Vite + TypeScript + Tailwind 4 + recharts，
  参考 Beszel 设计语言重新设计了 UI；后端保持 Go 单体服务。
- **配置拆分**：通用配置（`config.toml`）与抓取配置（`targets.toml`）分离，互不污染。
- **单二进制交付**：前端构建产物经 `go:embed` 嵌入 Go 二进制，部署无需携带 `html/` 目录。
- **纯 Go SQLite**：使用 `modernc.org/sqlite`，无需 CGO，便于交叉编译（Linux amd64 / macOS arm64）。
- **视觉语义**：延迟走势以绿色面积图为主，红色仅用于丢包异常，并做了响应式与移动端适配。

---

## 特性

- **多目标探测**：ICMP Ping（`PingTargets`）与 TCP 端口握手（`TcpTargets`）两类检测目标。
- **拓扑视图**：以本机为中心的网络拓扑，按告警规则高亮异常节点。
- **全国延迟地图**：对三大运营商（电信/联通/移动）各地区探测点采集延迟，绘制全国热力图。
- **告警**：支持邮件告警与浏览器声音告警（`alert.mp3`，可从磁盘替换自定义声音）。
- **在线诊断工具**：内置 Ping / TCP 探测工具，用于临时手动诊断。
- **历史归档**：探测数据写入 SQLite 并按配置天数滚动归档。
- **权限控制**：基于访问 IP 白名单（`Authiplist`）与目标 IP 授权，保护 API 与控制台。

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.25、标准 `net/http`、`modernc.org/sqlite`、`seelog` 日志、`jakecoffman/cron` 定时、`go-chart` 绘图、`golang.org/x/net` |
| 前端 | React 19、Vite 7、TypeScript、Tailwind CSS 4、recharts、Radix UI、lucide-react |
| 打包 | `go:embed` 将 `web/` 构建产物嵌入二进制；`alert.mp3` 例外，运行时从磁盘读取 |
| 部署 | Docker（alpine 3.20，无 CGO，`CGO_ENABLED=0`） |

---

## 目录结构

```
smartpingplus/
├── conf/                      # 配置文件（运行时可变）
│   ├── config.toml           # 通用配置（服务参数），Web 保存写回此处
│   ├── config-base.toml      # 通用配置出厂模板（config.toml 缺失时自动拷贝）
│   ├── targets.toml          # 抓取配置：检测目标与告警规则
│   └── seelog.xml            # 日志配置
├── db/                       # SQLite 数据库
│   ├── database.db           # 运行时数据（自动生成）
│   └── database-base.db      # 出厂空库模板
├── src/                      # Go 后端源码
│   ├── smartping.go          # 程序入口（flag -v 显示版本）
│   ├── g/                    # 配置解析、数据库、全局状态
│   ├── funcs/                # ping / tcpping / mapping / alert / archive 逻辑
│   ├── nettools/             # 底层 ping / tcpping / mtr 实现
│   ├── http/                 # HTTP 路由、API、内嵌静态资源
│   │   └── static/           # 前端构建产物（go:embed 嵌入）
│   └── ...
├── web/                      # React + Vite 前端源码
│   └── src/components/       # dashboard / topology / mapping / tools / config / alerts 等页面
├── bin/                      # 编译产物（smartpingplus 可执行文件）
├── dist/                     # 发布包模板（含 Dockerfile、deploy.sh、二进制）
├── control / control.cmd     # Linux / Windows 控制脚本（build/run/start/stop/...）
├── Dockerfile / deploy.sh    # 容器化部署
├── go.mod / go.sum
└── LICENSE
```

---

## 监控模式

通过 `config.toml` 的 `[Mode]` 节配置运行模式：

- **local（默认）**：单机独立运行，仅采集本机视角的数据。
- **cloud**：作为受控节点接入中心端（`Endpoint` 指向中心 SmartPing 服务），由中心统一调度与展示。

---

## 快速开始（从源码构建）

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
```

> 也可用内置脚本：`bash control build`（Linux）或 `control.cmd build`（Windows）。

### 3. 运行

```bash
./bin/smartpingplus          # 默认监听 8899 端口
# 或
bash control start           # 后台启动（写入 var/app.pid）
```

首次启动时若 `conf/config.toml`、`conf/targets.toml` 或 `db/database.db` 缺失，
会自动从对应的 `-base` 模板生成。浏览器访问 `http://<本机IP>:8899`，
默认访问密码为 `smartping`（见 `config.toml` 的 `Password`）。

---

## 配置说明

配置文件位于 `conf/`，三者关系如下：

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
Password = "smartping"   # Web 访问密码
Toollimit = 0            # 诊断工具频率限制（秒），0 不限

[Mode]
  Type = "local"         # local | cloud
  Endpoint = ""          # cloud 模式下的中心地址

[Base]
  Archive = 10           # 数据归档天数
  Refresh = 1            # 采集刷新间隔（分钟）
  Timeout = 5            # 单次探测超时（秒）

[Topology]
  Tline = "1"            # 拓扑连线粗细
  Tsound = "/alert.mp3"  # 告警声音路径
  Tsymbolsize = "70"     # 拓扑节点大小

[Alert]
  EmailHost = ""         # 告警邮件 SMTP 服务器
  RevcEmailList = ""     # 收件邮箱列表
  SendEmailAccount = ""  # 发件账号
  SendEmailPassword = "" # 发件密码
```

抓取配置示例（`targets.toml`）：

```toml
[PingTargets]
  "119.29.29.29" = "AliDNS"
  "127.0.0.1" = "本机"

[TcpTargets]
  "127.0.0.1:8899" = "本机"
  "cloud.tencent.com:443" = "腾讯云"

[[AlertRules]]            # 触发告警的阈值规则
  Addr = "119.29.29.29"
  Name = "AliDNS"
  Thdavgdelay = "200"     # 平均延迟阈值(ms)
  Thdchecksec = "900"     # 判定周期(秒)
  Thdloss = "30"          # 丢包率阈值(%)
  Thdoccnum = "3"         # 连续触发次数
```

---

## Web 控制台

控制台页面（`web/src/components/`）：

- **Dashboard**：各检测目标的实时延迟/丢包卡片与趋势图（绿色面积图为主）。
- **Topology**：以本机为中心的网络拓扑，异常节点红色高亮。
- **Mapping**：全国三大运营商延迟热力图。
- **Tools**：在线 Ping / TCP 探测工具。
- **Config**：可视化编辑通用配置与抓取配置，保存后写回 TOML 文件。
- **Alerts**：历史告警记录列表。

---

## HTTP API

所有 API 均受 IP 白名单 / 目标授权保护，未授权返回 `401`。

| 端点 | 说明 |
|------|------|
| `/api/config.json` | 读取当前配置（密码脱敏） |
| `/api/ping.json?ip=<addr>&starttime=&endtime=` | 某 ICMP 目标的延迟/丢包时序 |
| `/api/tcpping.json?ip=<host:port>&starttime=&endtime=` | 某 TCP 目标的延迟/丢包时序 |
| `/api/topology.json` | 各目标告警状态（true/false） |
| `/api/alert.json?date=` | 告警记录（按日期） |
| `/api/mapping.json?d=` | 全国延迟地图数据 |
| `/api/tools.json?t=<host>&mode=tcp&port=` | 在线诊断工具（ICMP / TCP） |
| `/api/saveconfig.json` | 保存配置（需密码） |
| `/api/sendmailtest.json` | 发送测试告警邮件 |
| `/api/graph.png?g=<api-url>` | 生成延迟/丢包 PNG 图（供旧版外链嵌入） |
| `/api/proxy.json?g=<url>` | 服务端代理取数 |

---

## 部署（Docker）

发布包在 `dist/`，含 `Dockerfile`、`deploy.sh`、二进制与配置模板。

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
- 镜像基于 `alpine:3.20`，安装了 `iputils`（ICMP 回退）与 `tzdata`（东八区）。
- **告警声音不嵌入镜像**，需将自定义 `alert.mp3` 放到挂载的 `/app/alert.mp3`（已在 Dockerfile 中预置默认文件）。

---

## 数据持久化

- 探测数据：`db/database.db`（SQLite），按 `Base.Archive` 天数归档。
- 配置：`conf/config.toml`、`conf/targets.toml`。
- 日志：`logs/`（由 `seelog.xml` 定义）。
- 升级时只需替换二进制与前端产物，保留上述目录即可。

---

## 开发模式

前端开发服务器（热更新）默认代理 `/api` 到本地 `8899` 后端：

```bash
cd web
npm install
npm run dev        # http://localhost:5173，代理 /api -> http://127.0.0.1:8899
```

需先在本地启动一个后端（`bash control start` 或 `./bin/smartpingplus`）。
修改前端后执行 `npm run build`，再重新编译 Go 二进制（`go:embed` 重新嵌入）。

---

## 许可证

遵循原 SmartPing 项目的许可证，详见 `LICENSE`。
