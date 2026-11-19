import { useEffect, useState } from "react"
import { fetchConfig, fetchMapping, type Config, type MappingData } from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { StatusDot } from "@/components/ui/badge"

export function MappingPage() {
	const [cfg, setCfg] = useState<Config | null>(null)
	const [data, setData] = useState<MappingData | null>(null)
	const [error, setError] = useState("")

	useEffect(() => {
		let cancelled = false
		fetchConfig()
			.then(async (c) => {
				if (cancelled) return
				setCfg(c)
				const m = await fetchMapping()
				if (!cancelled) setData(m)
			})
			.catch((e) => !cancelled && setError(String(e.message || e)))
		return () => {
			cancelled = true
		}
	}, [])

	if (error) return <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
	if (!cfg || !data) return <div className="py-10 text-center text-sm text-muted-foreground">加载中…</div>

	const series: { name: string; color: string; rows: { name: string; value: number }[] }[] = [
		{ name: "电信", color: "var(--chart-1)", rows: data.avgdelay.ctcc },
		{ name: "联通", color: "var(--chart-2)", rows: data.avgdelay.cucc },
		{ name: "移动", color: "var(--chart-3)", rows: data.avgdelay.cmcc },
	]

	return (
		<div className="space-y-4">
			<Card>
				<CardHeader>
					<CardTitle>全国延迟</CardTitle>
					<CardDescription>
						{data.text} · {data.subtext}
					</CardDescription>
				</CardHeader>
				<CardContent>
					{/* Province table grouped by ISP */}
					<div className="grid gap-6 md:grid-cols-3">
						{series.map((s) => (
							<div key={s.name} className="rounded-lg border border-border/60">
								<div className="border-b border-border px-4 py-2 text-sm font-semibold" style={{ color: s.color }}>
									{s.name}
								</div>
								<div className="max-h-[360px] overflow-auto">
									{s.rows.length === 0 && <div className="p-4 text-center text-xs text-muted-foreground">暂无数据</div>}
									{s.rows.map((row) => (
										<div key={row.name} className="flex items-center gap-3 border-b border-border/40 px-4 py-2 text-sm">
											<span className="w-14 shrink-0">{row.name}</span>
											<div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
												<div className="h-full rounded-full" style={{ width: `${Math.min(100, (row.value / 200) * 100)}%`, background: s.color }} />
											</div>
											<span className="w-12 text-right tabular-nums">{row.value} ms</span>
										</div>
									))}
								</div>
							</div>
						))}
					</div>
				</CardContent>
			</Card>
			<Card>
				<CardHeader>
					<CardTitle>节点列表</CardTitle>
					<CardDescription>SmartPingPlus 节点</CardDescription>
				</CardHeader>
				<CardContent className="grid gap-1">
					<div className="flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-muted">
						<StatusDot status="up" />
						<span className="font-medium">{cfg.Name}</span>
						<span className="text-xs text-muted-foreground">{cfg.Addr}</span>
					</div>
				</CardContent>
			</Card>
		</div>
	)
}
