import { forwardRef } from "react"
import { cn } from "@/lib/utils"

/**
 * Wrapper that keeps wide tables usable on small screens:
 * the table scrolls horizontally inside its own container.
 */
export const TableScroll = forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
	({ className, ...props }, ref) => (
		<div ref={ref} className={cn("w-full overflow-x-auto", className)} {...props} />
	)
)
TableScroll.displayName = "TableScroll"

/** Consistent header-cell styling used across data tables. */
export const Th = forwardRef<HTMLTableCellElement, React.ThHTMLAttributes<HTMLTableCellElement>>(
	({ className, ...props }, ref) => (
		<th
			ref={ref}
			className={cn(
				"whitespace-nowrap border-b border-border bg-[var(--table-header)] px-4 py-2.5 text-left text-xs font-semibold text-muted-foreground",
				className
			)}
			{...props}
		/>
	)
)
Th.displayName = "Th"

/** Consistent body-cell styling used across data tables. */
export const Td = forwardRef<HTMLTableCellElement, React.TdHTMLAttributes<HTMLTableCellElement>>(
	({ className, ...props }, ref) => (
		<td
			ref={ref}
			className={cn(
				"whitespace-nowrap border-b border-border/60 px-4 py-2.5",
				className
			)}
			{...props}
		/>
	)
)
Td.displayName = "Td"
