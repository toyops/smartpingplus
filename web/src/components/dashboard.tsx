import { useCallback, useEffect, useRef, useState } from "react"
import {
	fetchConfig,
	fetchPing,
	fetchTcpping,
	type Config,
	type PingData,
} from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { StatusDot } from "@/components/ui/badge"
import { MiniChart, MiniStats, buildPoints } from "@/components/mini-chart"
import { cn } from "@/lib/utils"
import { DetailDialog } from "@/components/detail-dialog"

export type DashMode = "ping" | "reverse" | "tcpping"

interface Target {
	key: string
	name: string
	displayIp: string
	data?: PingData
}

function targetsFor(cfg: Config, mode: DashMode): { key: string; name: string; displayIp: string }[] {
	if (mode === "ping") {
		return Object.entries(cfg.PingTargets ?? {}).map(([ip, name]) => ({ key: ip, name: name || ip, displayIp: ip }))
	}
	if (mode === "tcpping") {
		return Object.entries(cfg.TcpTargets ?? {}).map(([t, name]) => {
			const i = t.lastIndexOf(":")
			const host = i > -1 && t.indexOf(":") === i ? t.substring(0, i) : t
			return { key: t, name: name || host, displayIp: t }
		})
	}
	// reverse: sources that ping this agent。
	// 反向数据存在于"其他检测节点（agent）"的 pinglog 中，需多 agent 部署 +
	// 来源节点将本机设为 Ping 目标后，通过 proxy 拉取。单机部署无来源节点，
	// 保持空列表（由界面提示说明）。
	const out: { key: string; name: string; displayIp: string }[] = []
	return out
}

/** proxy URL helper (reverse data lives on the source agent) */
const proxyUrl = (host: string, port?: number) => `http://${host}:${port}`

export function DashboardPage({ mode, activeNode, onNodeChange }: { mode: DashMode; activeNode: string; onNodeChange: (n: string) => void }) {
	const [cfg, setCfg] = useState<Config | null>(null)
	const [targets, setTargets] = useState<Target[]>([])
	const targetsRef = useRef<Target[]>(targets)
	targetsRef.current = targets
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState("")
	const [lastUpdate, setLastUpdate] = useState<Date | null>(null)
	const [detail, setDetail] = useState<{ title: string; key: string; data: PingData } | null>(null)

	useEffect(() => {
		let cancelled = false
		setLoading(true)
		setLastUpdate(null)
		fetchConfig(activeNode || undefined)
			.then((c) => {
				if (cancelled) return
				setCfg(c)
				const initialTargets = targetsFor(c, mode).map((t) => ({ ...t }))
				setTargets(initialTargets)
				targetsRef.current = initialTargets
				setError("")
			})
			.catch((e) => !cancelled && setError(String(e.message || e)))
			.finally(() => !cancelled && setLoading(false))
		return () => {
			cancelled = true
		}
	}, [activeNode, mode])

	/** fetch data for one target (mode/proxy aware) */
	const fetchOne = useCallback(
		async (key: string, start?: string, end?: string): Promise<PingData> => {
			if (mode === "tcpping") {
				return fetchTcpping(key, start, end, activeNode || undefined)
			}
			if (mode === "reverse") {
				// data lives on the SOURCE agent (key); target is the current agent's IP
				const targetIp = cfg?.Addr ?? key
				return fetchPing(targetIp, start, end, proxyUrl(key, cfg?.Port))
			}
			return fetchPing(key, start, end, activeNode || undefined)
		},
		[mode, activeNode, cfg]
	)

	// poll target data; interval driven by cfg.Base.Refresh (minutes, min 1)
	// 每次轮询同时刷新 config 中的目标列表：配置页新增/删除节点后，本页自动同步新实例
	useEffect(() => {
		if (!cfg) return
		let cancelled = false
		const reload = async () => {
			let list: { key: string; name: string; displayIp: string }[] | null = null
			try {
				const c = await fetchConfig(activeNode || undefined)
				if (cancelled) return
				list = targetsFor(c, mode)
				setTargets(list.map((t) => ({ ...t })))
			} catch {
				/* config 拉取失败时沿用当前列表 */
			}
			const current = list ?? targetsRef.current
			current.forEach((t) => {
				fetchOne(t.key)
					.then((d) => {
						if (cancelled) return
						setTargets((prev) => prev.map((p) => (p.key === t.key ? { ...p, data: d } : p)))
						setLastUpdate(new Date())
					})
					.catch(() => !cancelled && setTargets((prev) => prev.map((p) => (p.key === t.key ? { ...p, data: undefined } : p))))
			})
		}
		reload()
		const mins = Math.max(1, cfg.Base?.Refresh ?? 1)
		const id = window.setInterval(reload, mins * 60 * 1000)
		return () => {
			cancelled = true
			window.clearInterval(id)
		}
	}, [cfg, activeNode, mode, fetchOne])

	const title = mode === "ping" ? "正向 Ping" : mode === "tcpping" ? "正向 TcpPing" : "反向 Ping"
	const desc =
		mode === "ping"
			? "本机对各目标的平均延迟与丢包率"
			: mode === "tcpping"
				? "本机对各 TCP 目标 (host:port) 的延迟与丢包率"
				: "各来源节点对本机的延迟与丢包率"

	return (
		<div className="space-y-4">
			<Card className="px-5 py-5 sm:px-6">
				<div className="flex flex-wrap items-start justify-between gap-3">
					<div className="min-w-0 flex-1">
						<h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
						<div className="mt-2 flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
							<span className="flex items-center gap-2">
								<StatusDot status="up" />
								<span className="truncate">{cfg ? cfg.Name : "加载中…"}</span>
							</span>
							<span className="h-4 w-px bg-primary/30" />
							<span>{targets.length} 个目标</span>
						</div>
					</div>
					{cfg && (
						<div className="flex w-full items-center gap-2 sm:w-auto">
							<span className="shrink-0 text-xs text-muted-foreground">节点</span>
							<select
								value={activeNode}
								onChange={(e) => onNodeChange(e.target.value)}
								className="h-9 min-w-0 flex-1 truncate rounded-md border border-input bg-card px-3 text-sm outline-none focus:ring-2 focus:ring-ring sm:flex-initial sm:max-w-[12rem]"
							>
								<option value="">{cfg.Name} (本机)</option>
							{/* 无 Network 节点表：本机为唯一检测源，多 agent 场景在此扩展 */}
							</select>
						</div>
					)}
				</div>
			</Card>

			{error && (
				<div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
					<span>加载失败：{error}</span>
					<button
						onClick={() => {
							setLoading(true)
							setError("")
							fetchConfig(activeNode || undefined)
								.then((c) => {
									setCfg(c)
									setTargets(targetsFor(c, mode).map((t) => ({ ...t })))
								})
								.catch((e) => setError(String(e.message || e)))
								.finally(() => setLoading(false))
						}}
						className="rounded-md border border-destructive/40 px-3 py-1 text-xs font-medium hover:bg-destructive/10"
					>
						重试
					</button>
				</div>
			)}
			{loading && (
				<div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
					{[0, 1, 2].map((i) => (
						<Card key={i} className="p-5">
							<div className="h-4 w-2/3 animate-pulse rounded bg-muted" />
							<div className="mt-3 h-3 w-1/2 animate-pulse rounded bg-muted" />
							<div className="mt-5 h-[130px] animate-pulse rounded bg-muted/70" />
						</Card>
					))}
				</div>
			)}
			{!loading && !targets.length && (
				<div className="py-10 text-center text-sm text-muted-foreground">
					{mode === "tcpping" && "暂无 TcpPing 目标（可在系统配置中添加）"}
					{mode === "ping" && "暂无 Ping 目标（可在系统配置中添加）"}
					{mode === "reverse" &&
						"反向 Ping 需要多 agent 部署：其他检测节点将本机设为 Ping 目标后，此处展示它们对本机的检测数据。单机部署无数据。"}
				</div>
			)}
			{!loading && lastUpdate && (
				<p className="text-right text-xs text-muted-foreground">
					最后更新：{lastUpdate.toLocaleTimeString("zh-CN", { hour12: false })}
					（每 {Math.max(1, cfg?.Base?.Refresh ?? 1)} 分钟自动刷新）
				</p>
			)}

			<div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
				{targets.map((t) => {
					const pts = buildPoints(t.data)
					const last = pts.length ? pts[pts.length - 1] : null
					const loss = last?.loss ?? null
					const avg = last?.delay ?? null
					return (
						<Card
							key={t.key}
							className={cn("cursor-pointer pb-1 transition-shadow hover:shadow-md")}
							onClick={() => t.data && setDetail({ title: `${cfg?.Name ?? ""} -> ${t.name}`, key: t.key, data: t.data })}
						>
							<CardHeader className="px-5 pt-4">
								<div className="flex items-center gap-2">
									<StatusDot status={loss === null ? "up" : loss >= 100 ? "down" : loss > 0 ? "warn" : "up"} />
									<CardTitle className="truncate text-sm font-semibold">
										{cfg?.Name ?? ""} -&gt; {t.name}
									</CardTitle>
								</div>
								<CardDescription className="truncate">{t.displayIp}</CardDescription>
							</CardHeader>
							<CardContent className="px-2">
								<MiniChart data={t.data} height={170} showAxis />
							</CardContent>
							<MiniStats avg={avg} loss={loss} />
						</Card>
					)
				})}
			</div>

			{detail && (
				<DetailDialog
					title={detail.title}
					data={detail.data}
					onClose={() => setDetail(null)}
					onRefresh={async (start, end) => {
						const d = await fetchOne(detail.key, start, end)
						setDetail((prev) => (prev ? { ...prev, data: d } : prev))
						return d
					}}
				/>
			)}
		</div>
	)
}
