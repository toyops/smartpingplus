import { useEffect, useState } from "react"
import * as Dialog from "@radix-ui/react-dialog"
import { ChevronDown, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { MiniChart } from "@/components/mini-chart"
import { TimeRangePicker, fmtTime } from "@/components/time-range-picker"
import type { PingData } from "@/lib/api"

export function DetailDialog({
	title,
	data,
	onClose,
	onRefresh,
}: {
	title: string
	data: PingData
	onClose: () => void
	onRefresh: (start: string, end: string) => Promise<PingData>
}) {
	const [current, setCurrent] = useState<PingData>(data)
	const [busy, setBusy] = useState(false)
	const [pickerOpen, setPickerOpen] = useState(false)
	const [rangeLabel, setRangeLabel] = useState("Last 3 hours")

	// 仅在打开弹窗时默认加载最近 3 小时；
	// 不能用 [data] 做依赖：onRefresh 更新 detail.data 会触发重跑，
	// 把用户刚选择的时间范围覆盖回 3 小时（点击不生效的根因）
	useEffect(() => {
		const now = new Date()
		const from = new Date(now.getTime() - 3 * 60 * 60 * 1000)
		onRefresh(fmtTime(from), fmtTime(now))
			.then(setCurrent)
			.catch(() => {})
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [])

	const applyRange = async (from: string, to: string, label: string) => {
		setPickerOpen(false)
		setRangeLabel(label)
		setBusy(true)
		try {
			const d = await onRefresh(from, to)
			setCurrent(d)
		} finally {
			setBusy(false)
		}
	}

	return (
		<Dialog.Root open onOpenChange={(o) => !o && onClose()}>
			<Dialog.Portal>
				<Dialog.Overlay className="fixed inset-0 z-50 bg-black/45" />
				<Dialog.Content className="fixed left-1/2 top-1/2 z-50 flex max-h-[92vh] w-[min(92vw,900px)] flex-col -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-card shadow-xl outline-none">
					<div className="flex items-center justify-between border-b border-border px-5 py-4">
						<Dialog.Title className="truncate pr-2 text-base font-semibold">{title}</Dialog.Title>
						<Dialog.Close asChild>
							<Button variant="ghost" size="icon" className="size-8">
								<X className="size-4" />
							</Button>
						</Dialog.Close>
					</div>
					<div className="relative border-b border-border px-5 py-3">
						<button
							onClick={() => setPickerOpen((o) => !o)}
							className="inline-flex h-9 items-center gap-2 rounded-md border border-border bg-muted/40 px-3 text-sm font-medium transition-colors hover:bg-muted/60"
						>
							{busy ? "查询中…" : rangeLabel}
							<ChevronDown className="size-4 text-muted-foreground" />
						</button>
						{pickerOpen && (
							<>
								{/* 点击面板外部关闭 */}
								<div className="fixed inset-0 z-40" onClick={() => setPickerOpen(false)} />
								<div className="absolute left-5 top-full z-50 mt-2 shadow-2xl">
									<TimeRangePicker onApply={applyRange} defaultLabel={rangeLabel} />
								</div>
							</>
						)}
					</div>
					<div className="min-h-0 flex-1 overflow-auto px-5 py-4">
						<MiniChart data={current} height={320} showAxis />
					</div>
				</Dialog.Content>
			</Dialog.Portal>
		</Dialog.Root>
	)
}
