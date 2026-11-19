import { useEffect, useState } from "react"
import { Lock, LogIn, Plus, Save, ShieldCheck, Trash2 } from "lucide-react"
import { fetchConfig, post, type Config } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { StatusDot } from "@/components/ui/badge"
import { TableScroll, Td, Th } from "@/components/ui/table"

// 与后端 src/http/http.go ValidIP4 保持一致（仅 IPv4，用于 Ping 目标 / Network 条目）
const IP4_RE = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/
const isValidIp4 = (s: string) => IP4_RE.test(s.trim())
const HOST_RE = /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/

/** 拆分 host:port（仅最后一个冒号视为端口分隔，兼容 IPv4 / 域名） */
function splitHostPort(t: string): { host: string; port?: string } {
	const i = t.lastIndexOf(":")
	if (i > -1 && t.indexOf(":") === i) return { host: t.substring(0, i), port: t.substring(i + 1) }
	return { host: t }
}

export function ConfigPage({
	authenticated = false,
	onLoginClick,
}: {
	authenticated?: boolean
	onLoginClick?: () => void
}) {
	const [cfg, setCfg] = useState<Config | null>(null)
	const [saving, setSaving] = useState(false)
	const [msg, setMsg] = useState("")
	const [newPassword, setNewPassword] = useState("")
	const [newPing, setNewPing] = useState("")
	const [newPingName, setNewPingName] = useState("")
	const [newTcpping, setNewTcpping] = useState("")
	const [newTcppingName, setNewTcppingName] = useState("")

	// editable fields
	const [base, setBase] = useState({ Timeout: 0, Refresh: 0, Archive: 0 })
	const [topo, setTopo] = useState({ Tsound: "", Tline: "", Tsymbolsize: "" })
	const [alert, setAlert] = useState({ EmailHost: "", SendEmailAccount: "", SendEmailPassword: "", RevcEmailList: "" })
	const [authiplist, setAuthiplist] = useState("")
	const [toollimit, setToollimit] = useState(0)

	useEffect(() => {
		if (!authenticated) return
		fetchConfig()
			.then((c) => {
				setCfg(c)
				setBase({ Timeout: c.Base.Timeout, Refresh: c.Base.Refresh, Archive: c.Base.Archive })
				setTopo({ Tsound: c.Topology.Tsound, Tline: c.Topology.Tline, Tsymbolsize: c.Topology.Tsymbolsize })
				setAlert({
					EmailHost: c.Alert.EmailHost,
					SendEmailAccount: c.Alert.SendEmailAccount,
					SendEmailPassword: c.Alert.SendEmailPassword,
					RevcEmailList: c.Alert.RevcEmailList,
				})
				setAuthiplist(c.Authiplist)
				setToollimit(c.Toollimit)
			})
			.catch((err) => {
				setMsg(`获取配置失败 (${(err as Error).message})`)
			})
	}, [authenticated])

	const save = async () => {
		if (!cfg) return
		setSaving(true)
		setMsg("")
		const next: Config = JSON.parse(JSON.stringify(cfg))
		next.Base.Timeout = base.Timeout
		next.Base.Refresh = base.Refresh
		next.Base.Archive = base.Archive
		next.Topology = { ...topo }
		next.Alert = { ...alert }
		next.Authiplist = authiplist
		next.Toollimit = toollimit
		next.Mode = { ...next.Mode, Type: "local" }
		if (newPassword.trim()) {
			next.Password = newPassword.trim()
		}
		try {
			const r = await post<{ status: string; info?: string }>("/api/saveconfig.json", {
				config: JSON.stringify(next),
			})
			setMsg(r.status === "true" ? "系统配置保存成功！" : `保存失败 (${r.info})`)
			if (r.status === "true") {
				setNewPassword("")
				setTimeout(() => window.location.reload(), 800)
			}
		} catch (e) {
			setMsg(`保存失败 (${(e as Error).message})`)
		} finally {
			setSaving(false)
		}
	}

	// ---------- Ping / TcpPing 目标管理 ----------
	const addPingTarget = () => {
		if (!cfg) return
		const ip = newPing.trim()
		if (!ip) return
		if (!isValidIp4(ip)) {
			setMsg("Ping 节点必须是合法 IPv4 地址")
			return
		}
		const next: Config = JSON.parse(JSON.stringify(cfg))
		next.PingTargets = next.PingTargets ?? {}
		next.PingTargets[ip] = newPingName.trim() || ip
		setCfg(next)
		setNewPing("")
		setNewPingName("")
		setMsg("")
	}

	const removePingTarget = (ip: string) => {
		if (!cfg) return
		const next: Config = JSON.parse(JSON.stringify(cfg))
		next.PingTargets = { ...next.PingTargets }
		delete next.PingTargets[ip]
		setCfg(next)
	}

	const addTcppingTarget = () => {
		if (!cfg) return
		const t = newTcpping.trim()
		if (!t) return
		const { host } = splitHostPort(t)
		if (!isValidIp4(host) && !HOST_RE.test(host)) {
			setMsg("TcpPing 目标格式应为 host 或 host:port")
			return
		}
		const next: Config = JSON.parse(JSON.stringify(cfg))
		next.TcpTargets = next.TcpTargets ?? {}
		next.TcpTargets[t] = newTcppingName.trim() || host
		setCfg(next)
		setNewTcpping("")
		setNewTcppingName("")
		setMsg("")
	}

	const removeTcppingTarget = (t: string) => {
		if (!cfg) return
		const next: Config = JSON.parse(JSON.stringify(cfg))
		next.TcpTargets = { ...next.TcpTargets }
		delete next.TcpTargets[t]
		setCfg(next)
	}

	// 未认证时显示访问控制提示卡片
	if (!authenticated) {
		return (
			<div className="flex min-h-[50vh] items-center justify-center py-12">
				<Card className="w-full max-w-md p-6 text-center">
					<div className="mx-auto grid size-12 place-items-center rounded-full bg-primary/10 text-primary">
						<Lock className="size-6" />
					</div>
					<CardTitle className="mt-4 text-xl">系统配置受限访问</CardTitle>
					<CardDescription className="mt-2 text-sm">
						系统配置涉及检测目标、告警参数和监控拓扑设置，需要管理员身份认证后方可查看和修改。
					</CardDescription>
					<div className="mt-6 flex justify-center">
						<Button onClick={onLoginClick} className="gap-2">
							<LogIn className="size-4" /> 管理员登录
						</Button>
					</div>
				</Card>
			</div>
		)
	}

	if (!cfg) return <div className="py-10 text-center text-sm text-muted-foreground">加载配置中…</div>

	const Field = ({
		label,
		value,
		onChange,
		type = "text",
		placeholder,
	}: {
		label: string
		value: string | number
		onChange: (v: string) => void
		type?: string
		placeholder?: string
	}) => (
		<div className="space-y-1.5">
			<label className="text-xs font-medium text-muted-foreground">{label}</label>
			<Input type={type} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} />
		</div>
	)

	return (
		<div className="space-y-4">
			<Card className="px-5 py-5 sm:px-6">
				<div className="flex flex-wrap items-center justify-between gap-3">
					<div className="min-w-0 flex-1">
						<h1 className="text-2xl font-semibold tracking-tight">系统配置</h1>
						<div className="mt-2 flex items-center gap-3 text-sm text-muted-foreground">
							<StatusDot status="up" />
							<span className="truncate">{cfg.Name} · 本地模式</span>
							<span className="inline-flex items-center gap-1 rounded bg-emerald-500/10 px-2 py-0.5 text-xs text-emerald-600 font-medium border border-emerald-500/20">
								<ShieldCheck className="size-3.5" /> 已验证管理员
							</span>
						</div>
					</div>
					<div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
						<Button onClick={save} disabled={saving} className="w-full shrink-0 sm:w-auto gap-1.5">
							<Save className="size-4" /> {saving ? "保存中…" : "保存配置"}
						</Button>
					</div>
				</div>
				{msg && (
					<div
						className={`mt-3 rounded-lg border px-3 py-2 text-sm ${
							msg.includes("成功")
								? "border-green-500/40 bg-green-500/10 text-green-600"
								: "border-destructive/40 bg-destructive/10 text-destructive"
						}`}
					>
						{msg}
					</div>
				)}
			</Card>

			<div className="grid gap-4 lg:grid-cols-3">
				<Card>
					<CardHeader>
						<CardTitle>基础配置</CardTitle>
						<CardDescription>Ping 与数据存档</CardDescription>
					</CardHeader>
					<CardContent className="grid gap-3">
						<Field
							label="接口超时 (秒)"
							type="number"
							value={base.Timeout}
							onChange={(v) => setBase((p) => ({ ...p, Timeout: Number(v) }))}
						/>
						<Field
							label="页面刷新 (分钟)"
							type="number"
							value={base.Refresh}
							onChange={(v) => setBase((p) => ({ ...p, Refresh: Number(v) }))}
						/>
						<Field
							label="数据存档 (天)"
							type="number"
							value={base.Archive}
							onChange={(v) => setBase((p) => ({ ...p, Archive: Number(v) }))}
						/>
						<Field
							label="检测工具限定频率 (秒)"
							type="number"
							value={toollimit}
							onChange={(v) => setToollimit(Number(v))}
						/>
					</CardContent>
				</Card>

				<Card>
					<CardHeader>
						<CardTitle>Ping 拓扑</CardTitle>
						<CardDescription>拓扑图与报警声音</CardDescription>
					</CardHeader>
					<CardContent className="grid gap-3">
						<Field label="报警声音" value={topo.Tsound} onChange={(v) => setTopo((p) => ({ ...p, Tsound: v }))} />
						<Field label="连线粗细" value={topo.Tline} onChange={(v) => setTopo((p) => ({ ...p, Tline: v }))} />
						<Field label="形状大小" value={topo.Tsymbolsize} onChange={(v) => setTopo((p) => ({ ...p, Tsymbolsize: v }))} />
						<Field
							label="修改管理员密码 (留空则不修改)"
							type="password"
							placeholder="输入新密码以修改…"
							value={newPassword}
							onChange={setNewPassword}
						/>
					</CardContent>
				</Card>

				<Card>
					<CardHeader>
						<CardTitle>报警邮件 & 授权</CardTitle>
						<CardDescription>SMTP 与 IP 白名单配置</CardDescription>
					</CardHeader>
					<CardContent className="grid gap-3">
						<Field label="邮件服务器" value={alert.EmailHost} onChange={(v) => setAlert((p) => ({ ...p, EmailHost: v }))} />
						<Field label="发件邮箱" value={alert.SendEmailAccount} onChange={(v) => setAlert((p) => ({ ...p, SendEmailAccount: v }))} />
						<Field
							label="发件邮箱密码"
							type="password"
							value={alert.SendEmailPassword}
							onChange={(v) => setAlert((p) => ({ ...p, SendEmailPassword: v }))}
						/>
						<Field
							label="收件邮箱列表 (分号隔开)"
							value={alert.RevcEmailList}
							onChange={(v) => setAlert((p) => ({ ...p, RevcEmailList: v }))}
						/>
						<Field label="授权 IP 列表 (逗号隔开)" value={authiplist} onChange={setAuthiplist} />
					</CardContent>
				</Card>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Ping 节点测试网络</CardTitle>
					<CardDescription>采集节点列表 · 管理本机 Ping 节点，修改后点击"保存配置"生效</CardDescription>
				</CardHeader>
				<CardContent className="p-0">
					<TableScroll>
						<table className="w-full min-w-[560px] text-sm">
							<thead>
								<tr>
									<Th className="text-left">节点名称</Th>
									<Th className="text-left">节点 IP</Th>
									<Th className="text-left">Ping 目标</Th>
								</tr>
							</thead>
							<tbody>
								{cfg && (
									<tr key={cfg.Addr} className="hover:bg-muted/40">
										<Td>
											<span className="flex items-center gap-2 font-medium">
												<StatusDot status="up" />
												{cfg.Name}
												<span className="text-xs text-muted-foreground">(本机)</span>
											</span>
										</Td>
										<Td className="tabular-nums">{cfg.Addr}</Td>
										<Td>
											<div className="flex flex-wrap gap-1">
												{Object.entries(cfg.PingTargets ?? {}).map(([ip, name]) => {
													const label = name && name !== ip ? `${name} (${ip})` : ip
													return (
														<span
															key={ip}
															title={ip}
															className="inline-flex items-center gap-1 rounded-md border bg-muted/40 px-2 py-0.5 text-xs tabular-nums"
														>
															{label}
															<button
																onClick={() => removePingTarget(ip)}
																className="text-muted-foreground transition-colors hover:text-destructive"
																title="删除该目标"
															>
																<Trash2 className="size-3" />
															</button>
														</span>
													)
												})}
												{!Object.keys(cfg.PingTargets ?? {}).length && (
													<span className="text-xs text-muted-foreground">暂无目标</span>
												)}
											</div>
											<div className="mt-2 flex flex-wrap gap-2">
												<Input
													placeholder="目标名称（可选）"
													value={newPingName}
													onChange={(e) => setNewPingName(e.target.value)}
													onKeyDown={(e) => e.key === "Enter" && addPingTarget()}
													className="h-8 w-36 text-xs"
												/>
												<Input
													placeholder="目标 IPv4 地址"
													value={newPing}
													onChange={(e) => setNewPing(e.target.value)}
													onKeyDown={(e) => e.key === "Enter" && addPingTarget()}
													className="h-8 w-44 text-xs tabular-nums"
												/>
												<Button size="sm" variant="outline" onClick={addPingTarget} className="h-8 text-xs">
													<Plus className="size-3.5" /> 添加 Ping 目标
												</Button>
											</div>
										</Td>
									</tr>
								)}
							</tbody>
						</table>
					</TableScroll>

					<div className="space-y-6 border-t p-4">
						<div className="space-y-3">
							<div className="flex items-center justify-between">
								<h3 className="text-sm font-semibold">本机 TcpPing 节点</h3>
								<span className="text-xs text-muted-foreground">{Object.keys(cfg.TcpTargets ?? {}).length} 个</span>
							</div>
							<div className="flex flex-wrap gap-2">
								{Object.entries(cfg.TcpTargets ?? {}).map(([t, name]) => {
									const { host } = splitHostPort(t)
									const label = name && name !== host ? `${name} (${t})` : t
									return (
										<span key={t} title={t} className="inline-flex items-center gap-1.5 rounded-md border bg-muted/40 px-2 py-1 text-xs tabular-nums">
											{label}
											<button onClick={() => removeTcppingTarget(t)} className="text-muted-foreground transition-colors hover:text-destructive" title="删除该 TcpPing 节点">
												<Trash2 className="size-3.5" />
											</button>
										</span>
									)
								})}
								{!Object.keys(cfg.TcpTargets ?? {}).length && <span className="text-xs text-muted-foreground">暂无 TcpPing 节点</span>}
							</div>
							<div className="flex flex-wrap gap-2">
								<Input
									placeholder="名称（可选，如 DNS）"
									value={newTcppingName}
									onChange={(e) => setNewTcppingName(e.target.value)}
									onKeyDown={(e) => e.key === "Enter" && addTcppingTarget()}
									className="h-9 w-32 flex-1"
								/>
								<Input
									placeholder="host 或 host:port，如 8.8.8.8:53"
									value={newTcpping}
									onChange={(e) => setNewTcpping(e.target.value)}
									onKeyDown={(e) => e.key === "Enter" && addTcppingTarget()}
									className="h-9 w-48 flex-1"
								/>
								<Button variant="outline" onClick={addTcppingTarget} className="shrink-0">
									<Plus className="size-4" /> 添加 TcpPing 节点
								</Button>
							</div>
						</div>
					</div>
				</CardContent>
			</Card>
		</div>
	)
}
