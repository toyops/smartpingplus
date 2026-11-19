import { useEffect, useMemo, useRef, useState } from "react"
import { CalendarDays, Clipboard, Copy, Search } from "lucide-react"

// ---------- 时间工具 ----------
export function fmtTime(d: Date): string {
	const p = (n: number) => String(n).padStart(2, "0")
	return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

/** 解析 "now" / "now-1h" / "2026-08-12 14:00" 等表达式为后端可用的 "YYYY-MM-DD HH:mm" */
export function parseExpr(expr: string, now = new Date()): string {
	const e = expr.trim()
	if (!e || e === "now") return fmtTime(now)
	const m = e.match(/^now-(\d+)([mhd])$/i)
	if (m) {
		const n = parseInt(m[1], 10)
		const d = new Date(now)
		if (m[2].toLowerCase() === "m") d.setMinutes(d.getMinutes() - n)
		else if (m[2].toLowerCase() === "h") d.setHours(d.getHours() - n)
		else d.setDate(d.getDate() - n)
		return fmtTime(d)
	}
	return e.replace("T", " ")
}

interface QuickRange {
	label: string
	from: string
	to: string
}

const QUICK_RANGES: QuickRange[] = [
	{ label: "Last 30 minutes", from: "now-30m", to: "now" },
	{ label: "Last 1 hour", from: "now-1h", to: "now" },
	{ label: "Last 3 hours", from: "now-3h", to: "now" },
	{ label: "Last 6 hours", from: "now-6h", to: "now" },
	{ label: "Last 12 hours", from: "now-12h", to: "now" },
	{ label: "Last 24 hours", from: "now-24h", to: "now" },
	{ label: "Last 2 days", from: "now-2d", to: "now" },
	{ label: "Last 7 days", from: "now-7d", to: "now" },
	{ label: "Last 30 days", from: "now-30d", to: "now" },
	{ label: "Last 90 days", from: "now-90d", to: "now" },
	{ label: "Today", from: "today", to: "now" },
	{ label: "Yesterday", from: "yesterday", to: "now" },
	{ label: "This week", from: "thisweek", to: "now" },
	{ label: "This month", from: "thismonth", to: "now" },
]

/** 快捷项 → 具体起止时间（特殊项按日历语义计算） */
function quickToExact(q: QuickRange, now: Date): { from: string; to: string } {
	const dayStart = (d: Date) => {
		const x = new Date(d)
		x.setHours(0, 0, 0, 0)
		return x
	}
	switch (q.from) {
		case "today":
			return { from: fmtTime(dayStart(now)), to: fmtTime(now) }
		case "yesterday": {
			const s = dayStart(now)
			s.setDate(s.getDate() - 1)
			return { from: fmtTime(s), to: fmtTime(dayStart(now)) }
		}
		case "thisweek": {
			const s = dayStart(now)
			s.setDate(s.getDate() - ((s.getDay() + 6) % 7)) // 周一起点
			return { from: fmtTime(s), to: fmtTime(now) }
		}
		case "thismonth": {
			const s = dayStart(now)
			s.setDate(1)
			return { from: fmtTime(s), to: fmtTime(now) }
		}
		default:
			return { from: parseExpr(q.from, now), to: parseExpr(q.to, now) }
	}
}

const RECENT_KEY = "smartping.recentRanges"
const MAX_RECENT = 5

interface Recent {
	label: string
	from: string
	to: string
}

// 与项目 ui/input.tsx 保持一致的主题输入框样式
const inputCls =
	"h-9 w-full rounded-md border border-input bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring"

// ---------- 组件 ----------
export function TimeRangePicker({
	onApply,
	defaultLabel = "Last 3 hours",
}: {
	onApply: (from: string, to: string, label: string) => void
	defaultLabel?: string
}) {
	const [absFrom, setAbsFrom] = useState("now-3h")
	const [absTo, setAbsTo] = useState("now")
	const [quick, setQuick] = useState(defaultLabel)
	const [search, setSearch] = useState("")
	const [recent, setRecent] = useState<Recent[]>([])
	const [tip, setTip] = useState("")
	const fromPickRef = useRef<HTMLInputElement>(null)
	const toPickRef = useRef<HTMLInputElement>(null)

	useEffect(() => {
		try {
			setRecent(JSON.parse(localStorage.getItem(RECENT_KEY) || "[]"))
		} catch {
			/* ignore */
		}
	}, [])

	const filtered = useMemo(
		() => QUICK_RANGES.filter((q) => q.label.toLowerCase().includes(search.trim().toLowerCase())),
		[search]
	)

	const pushRecent = (label: string, from: string, to: string) => {
		const next = [{ label, from, to }, ...recent.filter((r) => r.from !== from || r.to !== to)].slice(0, MAX_RECENT)
		setRecent(next)
		try {
			localStorage.setItem(RECENT_KEY, JSON.stringify(next))
		} catch {
			/* ignore */
		}
	}

	// 快捷项：立即应用
	const pickQuick = (q: QuickRange) => {
		setQuick(q.label)
		const now = new Date()
		const { from, to } = quickToExact(q, now)
		setAbsFrom(q.from)
		setAbsTo(q.to)
		onApply(from, to, q.label)
		pushRecent(q.label, from, to)
	}

	// 绝对范围 Apply
	const applyAbsolute = () => {
		const now = new Date()
		const from = parseExpr(absFrom, now)
		const to = parseExpr(absTo, now)
		setQuick("")
		onApply(from, to, `${absFrom} to ${absTo}`)
		pushRecent(`${absFrom} to ${absTo}`, from, to)
		setTip("已应用")
		setTimeout(() => setTip(""), 1500)
	}

	const copyRange = async () => {
		try {
			await navigator.clipboard.writeText(`${absFrom} to ${absTo}`)
			setTip("已复制")
			setTimeout(() => setTip(""), 1500)
		} catch {
			/* ignore */
		}
	}

	const pasteRange = async () => {
		try {
			const t = (await navigator.clipboard.readText()).trim()
			const i = t.toLowerCase().indexOf(" to ")
			if (i > -1) {
				setAbsFrom(t.slice(0, i).trim())
				setAbsTo(t.slice(i + 4).trim())
			} else {
				setTip("剪贴板内容格式应为: from to to")
				setTimeout(() => setTip(""), 2000)
			}
		} catch {
			/* ignore */
		}
	}

	const applyRecent = (r: Recent) => {
		setQuick("")
		setAbsFrom(r.from)
		setAbsTo(r.to)
		onApply(r.from, r.to, r.label)
		pushRecent(r.label, r.from, r.to)
	}

	const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || "Browser Time"
	const offsetH = -new Date().getTimezoneOffset() / 60
	const offsetStr = `UTC${offsetH >= 0 ? "+" : "-"}${String(Math.abs(offsetH)).padStart(2, "0")}:00`
	const tzLabel = tz === "Asia/Shanghai" ? "China, CST" : tz

	return (
		<div className="w-[min(92vw,560px)] rounded-lg border border-border bg-card p-4 text-foreground shadow-2xl">
			<div className="grid gap-4 sm:grid-cols-2">
				{/* 左侧：绝对时间范围 */}
				<div className="min-w-0">
					<h4 className="mb-2 text-xs font-semibold">Absolute time range</h4>

					<div className="space-y-2">
						<div>
							<label className="mb-1 block text-[11px] text-muted-foreground">From</label>
							<div className="flex items-center gap-1">
								<input value={absFrom} onChange={(e) => setAbsFrom(e.target.value)} className={inputCls} spellCheck={false} />
								<button
									type="button"
									onClick={() => {
										try {
											fromPickRef.current?.showPicker()
										} catch {
											fromPickRef.current?.focus()
										}
									}}
									className="grid size-9 shrink-0 place-items-center rounded-md border border-input bg-muted/40 text-muted-foreground hover:bg-muted/60 hover:text-foreground"
									title="选择时间"
								>
									<CalendarDays className="size-3.5" />
								</button>
								<input
									ref={fromPickRef}
									type="datetime-local"
									aria-hidden
									tabIndex={-1}
									className="sr-only"
									onChange={(e) => setAbsFrom(e.target.value.replace("T", " "))}
								/>
							</div>
						</div>
						<div>
							<label className="mb-1 block text-[11px] text-muted-foreground">To</label>
							<div className="flex items-center gap-1">
								<input value={absTo} onChange={(e) => setAbsTo(e.target.value)} className={inputCls} spellCheck={false} />
								<button
									type="button"
									onClick={() => {
										try {
											toPickRef.current?.showPicker()
										} catch {
											toPickRef.current?.focus()
										}
									}}
									className="grid size-9 shrink-0 place-items-center rounded-md border border-input bg-muted/40 text-muted-foreground hover:bg-muted/60 hover:text-foreground"
									title="选择时间"
								>
									<CalendarDays className="size-3.5" />
								</button>
								<input
									ref={toPickRef}
									type="datetime-local"
									aria-hidden
									tabIndex={-1}
									className="sr-only"
									onChange={(e) => setAbsTo(e.target.value.replace("T", " "))}
								/>
							</div>
						</div>

						<div className="flex items-center gap-1 pt-0.5">
							<button type="button" onClick={copyRange} className="grid size-9 place-items-center rounded-md border border-input bg-muted/40 text-muted-foreground hover:bg-muted/60 hover:text-foreground" title="复制时间范围">
								<Copy className="size-3.5" />
							</button>
							<button type="button" onClick={pasteRange} className="grid size-9 place-items-center rounded-md border border-input bg-muted/40 text-muted-foreground hover:bg-muted/60 hover:text-foreground" title="从剪贴板导入">
								<Clipboard className="size-3.5" />
							</button>
							{tip && <span className="ml-1 text-[11px] text-amber-600">{tip}</span>}
						</div>

						<button
							type="button"
							onClick={applyAbsolute}
							className="h-9 w-full rounded-md bg-primary text-sm font-medium text-primary-foreground hover:bg-primary/90"
						>
							Apply time range
						</button>
					</div>

					<div className="mt-3 border-t border-border pt-2">
						<h4 className="mb-1.5 text-[11px] font-semibold text-muted-foreground">Recently used absolute ranges</h4>
						{recent.length === 0 ? (
							<p className="text-[11px] text-muted-foreground/70">暂无最近使用记录</p>
						) : (
							<ul className="space-y-0.5">
								{recent.map((r) => (
									<li key={r.label + r.from + r.to}>
										<button
											type="button"
											onClick={() => applyRecent(r)}
											className="w-full truncate rounded px-1.5 py-1 text-left text-[11px] text-muted-foreground hover:bg-accent/60 hover:text-foreground"
											title={`${r.from} to ${r.to}`}
										>
											{r.label}
										</button>
									</li>
								))}
							</ul>
						)}
					</div>
				</div>

				{/* 右侧：快捷时间范围 */}
				<div className="flex min-h-0 flex-col">
					<div className="relative mb-2">
						<Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
						<input
							value={search}
							onChange={(e) => setSearch(e.target.value)}
							placeholder="Search quick ranges"
							className="h-9 w-full rounded-md border border-input bg-background pl-7 pr-3 text-sm text-foreground placeholder:text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring"
						/>
					</div>
					<div className="max-h-[340px] min-h-0 flex-1 overflow-y-auto pr-1">
						{filtered.map((q) => (
							<button
								key={q.label}
								type="button"
								onClick={() => pickQuick(q)}
								className={
									"block w-full truncate rounded border-l-2 px-2 py-1.5 text-left text-xs transition-colors " +
									(q.label === quick
										? "border-primary bg-accent font-medium text-accent-foreground"
										: "border-transparent text-foreground hover:bg-accent/60")
								}
							>
								{q.label}
							</button>
						))}
						{filtered.length === 0 && <p className="px-2 py-1 text-[11px] text-muted-foreground">无匹配的快捷范围</p>}
					</div>
				</div>
			</div>

			{/* 底部：时区信息栏 */}
			<div className="mt-3 flex items-center justify-between gap-2 border-t border-border pt-2 text-[11px] text-muted-foreground">
				<span>
					Browser Time · <span className="text-foreground">{tzLabel}</span>
				</span>
				<span className="tabular-nums">{offsetStr}</span>
				<span className="flex items-center gap-1">
					<button type="button" className="text-foreground hover:text-primary">
						⌄ Change time settings
					</button>
				</span>
			</div>
		</div>
	)
}
