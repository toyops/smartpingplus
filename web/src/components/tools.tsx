import { useEffect, useState } from "react"
import { CheckCircle2, Loader2, Search, XCircle } from "lucide-react"
import { fetchConfig, fetchTools, type Config, type ToolsRes } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { TableScroll, Td, Th } from "@/components/ui/table"

interface Row {
	addr: string
	name: string
	checked: boolean
	state: "idle" | "loading" | "ok" | "error"
	result?: ToolsRes
	err?: string
}

export function ToolsPage() {
	const [cfg, setCfg] = useState<Config | null>(null)
	const [target, setTarget] = useState("")
	const [rows, setRows] = useState<Row[]>([])
	const [running, setRunning] = useState(false)

	useEffect(() => {
		fetchConfig().then((c) => {
			setCfg(c)
			setTarget(c.Addr)
			// 无 Network 节点表：检测源为本机
			setRows([{ addr: c.Addr, name: c.Name, checked: true, state: "idle" as const }])
		})
	}, [])

	const run = async () => {
		if (!target || running) return
		const picked = rows.filter((r) => r.checked)
		if (!picked.length) return
		setRunning(true)
		setRows((prev) =>
			prev.map((r) =>
				r.checked
					? { ...r, state: "loading", result: undefined, err: undefined }
					: r
			)
		)
		await Promise.all(
			picked.map(async (r) => {
				try {
					const res = await fetchTools(`${r.addr}:${cfg?.Port}`, target)
					setRows((prev) => prev.map((p) => (p.addr === r.addr ? { ...p, state: "ok", result: res } : p)))
				} catch (e) {
					setRows((prev) => prev.map((p) => (p.addr === r.addr ? { ...p, state: "error", err: String((e as Error).message) } : p)))
				}
			})
		)
		setRunning(false)
	}

	return (
		<div className="space-y-4">
			<Card>
				<CardHeader>
					<CardTitle>检测工具</CardTitle>
					<CardDescription>对各节点发起 ICMP Ping 检测</CardDescription>
				</CardHeader>
				<CardContent className="flex flex-wrap items-center gap-3">
					<Input value={target} onChange={(e) => setTarget(e.target.value)} placeholder="目标地址 / 域名" className="w-full min-w-0 sm:w-auto sm:flex-1 sm:max-w-xs" onKeyDown={(e) => e.key === "Enter" && run()} />
					<Button onClick={run} disabled={!target || running} className="w-full shrink-0 sm:w-auto">
						{running ? <Loader2 className="size-4 animate-spin" /> : <Search className="size-4" />} {running ? "检测中…" : "检测"}
					</Button>
				</CardContent>
			</Card>

			<Card>
				<CardContent className="p-0">
					<TableScroll>
						<table className="w-full min-w-[760px] text-sm">
							<thead>
								<tr>
									<Th className="text-left">
										<input type="checkbox" checked={rows.every((r) => r.checked) && rows.length > 0} onChange={(e) => setRows((prev) => prev.map((r) => ({ ...r, checked: e.target.checked })))} className="size-3.5" aria-label="全选" />
									</Th>
									<Th className="text-left">节点名称</Th>
									<Th className="text-left">解析 IP</Th>
									<Th className="text-right">发送</Th>
									<Th className="text-right">接收</Th>
									<Th className="text-right">丢弃</Th>
									<Th className="text-right">最大</Th>
									<Th className="text-right">最小</Th>
									<Th className="text-right">平均</Th>
									<Th className="text-center">状态</Th>
								</tr>
							</thead>
							<tbody>
								{rows.map((r) => (
									<tr key={r.addr} className="hover:bg-muted/40">
										<Td>
											<input type="checkbox" checked={r.checked} onChange={(e) => setRows((prev) => prev.map((p) => (p.addr === r.addr ? { ...p, checked: e.target.checked } : p)))} className="size-3.5" aria-label={`选择 ${r.name}`} />
										</Td>
										<Td className="font-medium">{r.name}</Td>
										<Td className="tabular-nums">{r.result?.ip ?? ""}</Td>
										<Td className="text-right tabular-nums">{r.result?.ping.SendPk ?? ""}</Td>
										<Td className="text-right tabular-nums">{r.result?.ping.RevcPk ?? ""}</Td>
										<Td className="text-right tabular-nums">{r.result ? `${r.result.ping.LossPk}%` : ""}</Td>
										<Td className="text-right tabular-nums">{r.result ? `${r.result.ping.MaxDelay.toFixed(2)}ms` : ""}</Td>
										<Td className="text-right tabular-nums">{r.result ? `${r.result.ping.MinDelay.toFixed(2)}ms` : ""}</Td>
										<Td className="text-right tabular-nums">{r.result ? `${r.result.ping.AvgDelay.toFixed(2)}ms` : ""}</Td>
										<Td className="text-center">
											{r.state === "loading" && <Loader2 className="mx-auto size-4 animate-spin text-muted-foreground" aria-label="检测中" />}
											{r.state === "ok" && <CheckCircle2 className="mx-auto size-4 text-green-500" aria-label="正常" />}
											{r.state === "error" && (
												<span title={r.err}>
													<XCircle className="mx-auto size-4 text-red-500" aria-label="失败" />
												</span>
											)}
										</Td>
									</tr>
								))}
								{!rows.length && (
									<tr>
										<td colSpan={10} className="px-4 py-6 text-center text-muted-foreground">
											暂无节点
										</td>
									</tr>
								)}
							</tbody>
						</table>
					</TableScroll>
				</CardContent>
			</Card>
		</div>
	)
}
