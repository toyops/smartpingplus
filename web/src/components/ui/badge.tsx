import { forwardRef } from "react"
import { cn } from "@/lib/utils"

export const Badge = forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(({ className, ...props }, ref) => (
	<div ref={ref} className={cn("inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium bg-secondary text-secondary-foreground", className)} {...props} />
))
Badge.displayName = "Badge"

/** Status dot with ping animation for "up" */
export function StatusDot({ status, className }: { status: "up" | "down" | "warn" | "paused"; className?: string }) {
	const cls = {
		up: "bg-green-500",
		down: "bg-red-500",
		warn: "bg-yellow-500",
		paused: "bg-primary/40",
	}[status]
	return (
		<span className={cn("relative inline-flex h-3 w-3 shrink-0", className)}>
			{status === "up" && (
				<span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75" style={{ animationDuration: "1.5s" }} />
			)}
			<span className={cn("relative inline-flex h-3 w-3 rounded-full", cls)} />
		</span>
	)
}
