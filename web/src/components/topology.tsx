import { useEffect, useState } from "react"
import { fetchConfig, fetchTopology, type Config } from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge, StatusDot } from "@/components/ui/badge"

interface NodeState {
	addr: string
	name: string
	from: boolean
	status: "up" | "down" | "pending"
}

export function TopologyPage() {
	const [cfg, setCfg] = useState<Config | null>(null)
	const [nodes, setNodes] = useState<NodeState[]>([])
	const [edges, setEdges] = useState<{ from: string; to: string; ok: boolean }[]>([])
	const [error, setError] = useState("")

	useEffect(() => {
		let cancelled = false
		fetchConfig()
			.then(async (c) => {
				if (cancelled) return
				setCfg(c)
				// nodes: PingTargets + TcpTargets hosts + AlertRules targets 去重
				const nameOf = (addr: string): string => {
					if (c.PingTargets?.[addr]) return c.PingTargets[addr]
					for (const [t, n] of Object.entries(c.TcpTargets ?? {})) {
						const host = t.indexOf(":") === t.lastIndexOf(":") && t.includes(":") ? t.substring(0, t.lastIndexOf(":")) : t
						if (host === addr) return n
					}
					return addr
				}
				const addrSet = new Set<string>([c.Addr])
				for (const addr of Object.keys(c.PingTargets ?? {})) addrSet.add(addr)
				for (const t of Object.keys(c.TcpTargets ?? {}))
					addrSet.add(t.indexOf(":") === t.lastIndexOf(":") && t.includes(":") ? t.substring(0, t.lastIndexOf(":")) : t)
				for (const r of c.AlertRules ?? []) addrSet.add(r.Addr)
				const ns: NodeState[] = Array.from(addrSet).map((addr) => ({
					addr,
					name: nameOf(addr),
					//检测源为本机（唯一来源）
					from: addr === c.Addr,
					status: "pending",
				}))
				// edges from AlertRules (targets monitored by the local agent)
				const es: { from: string; to: string; ok: boolean }[] = (c.AlertRules ?? []).map((r) => ({
					from: c.Addr,
					to: r.Addr,
					ok: false,
				}))
				setNodes(ns)
				setEdges(es)

				// probe each from-node for topology status
				const probes = ns.filter((n) => n.from)
				await Promise.all(
					probes.map(async (n) => {
						try {
							const topo = await fetchTopology(proxyUrl(n.addr, c.Port))
							setEdges((prev) =>
								prev.map((e) => (e.from === n.addr ? { ...e, ok: topo[e.to] === "true" } : e))
							)
							setNodes((prev) => prev.map((p) => (p.addr === n.addr ? { ...p, status: "up" } : p)))
						} catch {
							setNodes((prev) => prev.map((p) => (p.addr === n.addr ? { ...p, status: "down" } : p)))
						}
					})
				)
			})
			.catch((e) => !cancelled && setError(String(e.message || e)))
		return () => {
			cancelled = true
		}
	}, [])

	if (error) return <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
	if (!cfg) return <div className="py-10 text-center text-sm text-muted-foreground">加载中…</div>

	const fromNodes = nodes.filter((n) => n.from)
	const targetNodes = nodes.filter((n) => !n.from)

	return (
		<div className="space-y-4">
			<Card>
				<CardHeader>
					<CardTitle>Ping 拓扑</CardTitle>
					<CardDescription>节点间连通性：绿色=连通，红色=中断</CardDescription>
				</CardHeader>
				<CardContent>
					<TopoGraph nodes={nodes} edges={edges} />
				</CardContent>
			</Card>
			<div className="grid gap-4 lg:grid-cols-2">
				<Card>
					<CardHeader>
						<CardTitle>来源节点</CardTitle>
						<CardDescription>发起互 PING 检测的节点</CardDescription>
					</CardHeader>
					<CardContent className="grid gap-1">
						{fromNodes.map((n) => (
							<div key={n.addr} className="flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-muted">
								<StatusDot status={n.status === "pending" ? "paused" : n.status} />
								<span className="font-medium">{n.name}</span>
								<span className="text-xs text-muted-foreground">{n.addr}</span>
							</div>
						))}
						{!fromNodes.length && <div className="text-sm text-muted-foreground">暂无来源节点</div>}
						<div className="mt-3 flex flex-wrap gap-2">
							<Badge className="bg-green-500/15 text-green-600">
								<StatusDot status="up" /> 连通
							</Badge>
							<Badge className="bg-red-500/15 text-red-600">
								<StatusDot status="down" /> 中断
							</Badge>
							<Badge>
								<StatusDot status="paused" /> 待检测
							</Badge>
						</div>
					</CardContent>
				</Card>
				<Card>
					<CardHeader>
						<CardTitle>目标节点</CardTitle>
						<CardDescription>被检测的目标</CardDescription>
					</CardHeader>
					<CardContent className="grid gap-1">
						{targetNodes.map((n) => (
							<div key={n.addr} className="flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-muted">
								<StatusDot status="paused" />
								<span className="font-medium">{n.name}</span>
								<span className="text-xs text-muted-foreground">{n.addr}</span>
							</div>
						))}
						{!targetNodes.length && <div className="text-sm text-muted-foreground">暂无目标节点</div>}
					</CardContent>
				</Card>
			</div>
		</div>
	)
}

const proxyUrl = (host: string, port?: number) => `http://${host}:${port}`

/** Simple SVG circular topology graph */
function TopoGraph({ nodes, edges }: { nodes: NodeState[]; edges: { from: string; to: string; ok: boolean }[] }) {
	const size = 520
	const cx = size / 2
	const cy = size / 2
	const r = 180
	const active = nodes.filter((n) => n.from || edges.some((e) => e.from === n.addr || e.to === n.addr))

	if (active.length === 0) {
		return (
			<div className="flex h-[260px] items-center justify-center text-sm text-muted-foreground">暂无拓扑节点（来源节点未配置目标）</div>
		)
	}

	const pos = new Map<string, { x: number; y: number }>()
	active.forEach((n, i) => {
		const angle = (2 * Math.PI * i) / active.length - Math.PI / 2
		pos.set(n.addr, { x: cx + r * Math.cos(angle), y: cy + r * Math.sin(angle) })
	})

	return (
		<svg viewBox={`0 0 ${size} ${size}`} className="mx-auto h-auto w-full max-w-[600px]">
			{edges.map((e, i) => {
				const p1 = pos.get(e.from)
				const p2 = pos.get(e.to)
				if (!p1 || !p2) return null
				return (
					<line
						key={i}
						x1={p1.x}
						y1={p1.y}
						x2={p2.x}
						y2={p2.y}
						stroke={e.ok ? "var(--green-500)" : "var(--red-500)"}
						strokeWidth={2}
						strokeOpacity={0.8}
					/>
				)
			})}
			{active.map((n) => {
				const p = pos.get(n.addr)
				if (!p) return null
				const color = n.status === "up" ? "var(--green-500)" : n.status === "down" ? "var(--red-500)" : "var(--muted-foreground)"
				return (
					<g key={n.addr}>
						<circle cx={p.x} cy={p.y} r={22} fill={color} stroke="var(--card)" strokeWidth={2} />
						<text x={p.x} y={p.y + 4} textAnchor="middle" fontSize={11} fill="white" fontWeight={600}>
							{n.name.slice(0, 3)}
						</text>
						<text x={p.x} y={p.y + 38} textAnchor="middle" fontSize={10} fill="var(--muted-foreground)">
							{n.name}
						</text>
					</g>
				)
			})}
		</svg>
	)
}
