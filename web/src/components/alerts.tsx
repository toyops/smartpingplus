import { useEffect, useRef, useState } from "react"
import { fetchAlerts, fetchConfig, type AlertLog, type Config } from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge, StatusDot } from "@/components/ui/badge"
import { TableScroll, Td, Th } from "@/components/ui/table"

export function AlertsPage() {
	const [cfg, setCfg] = useState<Config | null>(null)
	const [dates, setDates] = useState<string[]>([])
	const [logs, setLogs] = useState<AlertLog[]>([])
	const [selected, setSelected] = useState("")
	const [nodes, setNodes] = useState<{ addr: string; name: string }[]>([])

	const reqSeq = useRef(0)

	useEffect(() => {
		let cancelled = false
		fetchConfig()
			.then(async (c) => {
				if (cancelled) return
				setCfg(c)
				const fromNodes =
					(c.AlertRules ?? []).length > 0 ? [{ addr: c.Addr, name: c.Name }] : []
				setNodes(fromNodes)
				await loadAlerts(c, fromNodes, "", cancelled)
			})
			.catch(() => undefined)
		return () => {
			cancelled = true
		}
	}, [])

	async function loadAlerts(c: Config, fromNodes: { addr: string; name: string }[], date: string, cancelled: boolean) {
		const seq = ++reqSeq.current
		const allDates = new Set<string>()
		const allLogs: AlertLog[] = []
		await Promise.all(
			fromNodes.map(async (n) => {
				try {
					const [ds, ls] = await fetchAlerts(proxyUrl(n.addr, c.Port), date || undefined)
					ds.forEach((d) => allDates.add(d))
					allLogs.push(...ls)
				} catch {
					/* skip unreachable node */
				}
			})
		)
		// drop stale responses when the user switched dates while we were loading
		if (cancelled || seq !== reqSeq.current) return
		setDates(Array.from(allDates).sort().reverse())
		setLogs(allLogs.sort((a, b) => (a.Logtime < b.Logtime ? 1 : -1)))
	}

	const pickDate = (d: string) => {
		setSelected(d)
		if (cfg && nodes.length) loadAlerts(cfg, nodes, d, false)
	}

	return (
		<div className="space-y-4">
			<div className="grid gap-4 lg:grid-cols-4">
				<Card>
					<CardHeader>
						<CardTitle>报警存档</CardTitle>
						<CardDescription>按日期查看</CardDescription>
					</CardHeader>
					<CardContent className="grid gap-1">
						{dates.map((d) => (
							<div
								key={d}
								className={`flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm hover:bg-muted ${selected === d ? "bg-muted font-medium" : ""}`}
								onClick={() => pickDate(d)}
							>
								<StatusDot status="up" />
								{d}
							</div>
						))}
						{!dates.length && <div className="text-sm text-muted-foreground">暂无存档</div>}
					</CardContent>
				</Card>

				<Card className="lg:col-span-3">
					<CardHeader>
						<CardTitle>报警历史</CardTitle>
						<CardDescription>{selected ? `日期: ${selected}` : "全部记录"}</CardDescription>
					</CardHeader>
					<CardContent className="p-0">
						<TableScroll>
							<table className="w-full min-w-[640px] text-sm">
								<thead>
									<tr>
										<Th className="text-left">报警日期</Th>
										<Th className="text-left">源节点</Th>
										<Th className="text-left">源 IP</Th>
										<Th className="text-left">目标节点</Th>
										<Th className="text-left">目标 IP</Th>
									</tr>
								</thead>
								<tbody>
									{logs.map((l, i) => (
										<tr key={`${l.Logtime}-${i}`} className="hover:bg-muted/40">
											<Td className="tabular-nums">{l.Logtime}</Td>
											<Td>{l.Fromname}</Td>
											<Td className="tabular-nums">{l.Fromip}</Td>
											<Td>{l.Targetname}</Td>
											<Td className="tabular-nums">{l.Targetip}</Td>
										</tr>
									))}
									{!logs.length && (
										<tr>
											<td colSpan={5} className="px-4 py-6 text-center text-muted-foreground">
												暂无报警记录
											</td>
										</tr>
									)}
								</tbody>
							</table>
						</TableScroll>
					</CardContent>
				</Card>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>节点列表</CardTitle>
					<CardDescription>来源节点</CardDescription>
				</CardHeader>
				<CardContent className="grid gap-1">
					{nodes.map((n) => (
						<div key={n.addr} className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm hover:bg-muted">
							<StatusDot status="up" />
							<span className="font-medium">{n.name}</span>
							<span className="text-xs text-muted-foreground">{n.addr}</span>
						</div>
					))}
					{!nodes.length && <div className="text-sm text-muted-foreground">暂无来源节点</div>}
				</CardContent>
			</Card>
		</div>
	)
}

const proxyUrl = (host: string, port?: number) => `http://${host}:${port}`
