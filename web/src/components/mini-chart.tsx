import { useMemo } from "react"
import { Area, AreaChart, CartesianGrid, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts"
import { DELAY_COLOR, LOSS_COLOR, cn, lossStatus, shortTime } from "@/lib/utils"
import type { PingData } from "@/lib/api"

type Point = { t: string; delay: number | null; loss: number | null }

/** Merge PingData arrays into chart points aligned by index. */
export function buildPoints(d: PingData | undefined): Point[] {
	if (!d) return []
	const n = Math.min(d.lastcheck.length, d.avgdelay.length, d.losspk.length)
	const pts: Point[] = []
	for (let i = 0; i < n; i++) {
		const avg = parseFloat(d.avgdelay[i])
		const loss = parseFloat(d.losspk[i])
		pts.push({
			t: d.lastcheck[i],
			delay: Number.isNaN(avg) ? null : avg,
			loss: Number.isNaN(loss) ? null : loss,
		})
	}
	return pts
}

export function MiniChart({
	data,
	height = 130,
	showAxis = false,
	className,
}: {
	data: PingData | undefined
	height?: number
	showAxis?: boolean
	className?: string
}) {
	const points = useMemo(() => buildPoints(data), [data])
	const last = points.length ? points[points.length - 1] : null
	const status = last ? lossStatus(last.loss ?? 0) : "up"

	const axisTick = { fontSize: 10, fill: "var(--muted-foreground)" }

	// 横轴刻度：每半小时（整点 / 半点）一个时间标记
	const halfHourTicks = useMemo(() => {
		const out: string[] = []
		for (const p of points) {
			const m = p.t.match(/(\d{2}):(\d{2})$/)
			if (m && (m[2] === "00" || m[2] === "30")) out.push(p.t)
		}
		return out
	}, [points])

	return (
		<div className={cn("relative w-full overflow-hidden", className)} style={{ height }}>
			<ResponsiveContainer width="100%" height="100%">
				<AreaChart data={points} margin={{ top: 6, right: 8, bottom: showAxis ? 6 : 0, left: 4 }}>
					<defs>
						{/* green area dominates the chart: latency trend is the primary visual */}
						<linearGradient id="gDelay" x1="0" y1="0" x2="0" y2="1">
							<stop offset="0%" stopColor={DELAY_COLOR} stopOpacity={0.55} />
							<stop offset="100%" stopColor={DELAY_COLOR} stopOpacity={0.06} />
						</linearGradient>
					</defs>
					<CartesianGrid vertical={false} stroke="hsl(30 8.11% 85.49% / .45)" strokeDasharray="3 3" />
					{showAxis && (
						<XAxis
							dataKey="t"
							tickFormatter={shortTime}
							tick={axisTick}
							axisLine={{ stroke: "var(--border)" }}
							tickLine={false}
							ticks={halfHourTicks.length > 1 ? halfHourTicks : undefined}
							minTickGap={24}
						/>
					)}
					{/* delay: area starts from 0 so the green body fills the chart */}
					<YAxis yAxisId="delay" hide={!showAxis} domain={[0, "auto"]} tick={axisTick} axisLine={false} tickLine={false} width={34} />
					<YAxis yAxisId="loss" hide={!showAxis} domain={[0, 100]} orientation="right" tick={axisTick} axisLine={false} tickLine={false} width={30} />
					<Tooltip
						contentStyle={{
							background: "var(--card)",
							border: "1px solid var(--border)",
							borderRadius: 12,
							fontSize: 12,
							boxShadow: "0 12px 30px -8px rgba(0,0,0,.25)",
							fontVariantNumeric: "tabular-nums",
						}}
						labelFormatter={(_, p) => (p?.[0]?.payload as Point | undefined)?.t ?? ""}
						formatter={(value, name) => [
							`${Number(value).toFixed(1)}${name === "丢包率" ? "%" : " ms"}`,
							name as string,
						]}
					/>
{/* latency trend: big green area */}
				<Area yAxisId="delay" type="monotone" dataKey="delay" name="平均延迟" stroke={DELAY_COLOR} strokeWidth={2} fill="url(#gDelay)" isAnimationActive={false} connectNulls />
				{/* packet loss: red spikes drawn as stepped area from baseline 0.
				    Only visible when loss > 0, fulfilling "red = abnormal only". */}
				<Area yAxisId="loss" type="stepBefore" dataKey="loss" name="丢包率" stroke="none" fill={LOSS_COLOR} fillOpacity={0.9} isAnimationActive={false} connectNulls={false} />
				{status !== "up" && last?.loss != null ? (
					<ReferenceLine yAxisId="loss" y={last.loss} stroke={LOSS_COLOR} strokeDasharray="3 3" strokeOpacity={0.55} />
				) : null}
				</AreaChart>
			</ResponsiveContainer>
		</div>
	)
}

/** Status summary row rendered below a mini card. */
export function MiniStats({ avg, loss }: { avg: number | null; loss: number | null }) {
	return (
		<div className="flex items-center gap-3 px-5 pb-4 text-xs text-muted-foreground tabular-nums">
			<span style={{ color: "hsl(140 52% 38%)" }}>延迟 {avg == null ? "--" : avg.toFixed(1)} ms</span>
			<span style={{ color: loss != null && loss > 0 ? LOSS_COLOR : "hsl(140 52% 38%)" }}>丢包 {loss == null ? "--" : loss}%</span>
		</div>
	)
}

export function statusDotClass(loss: number | null) {
	return lossStatus(loss ?? 0)
}
