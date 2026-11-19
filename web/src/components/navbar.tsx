import { Activity, Bell, Cable, LayoutGrid, Lock, LogIn, LogOut, Map, Network, Settings, ShieldCheck, Wrench, type LucideIcon } from "lucide-react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

export type PageKey = "ping" | "reverse" | "tcpping" | "topology" | "mapping" | "tools" | "alerts" | "config"

export const NAV_ITEMS: { key: PageKey; label: string; icon: LucideIcon; href: string; protected?: boolean }[] = [
	{ key: "ping", label: "正向 Ping", icon: Activity, href: "#/ping" },
	{ key: "tcpping", label: "正向 TcpPing", icon: Cable, href: "#/tcpping" },
	{ key: "reverse", label: "反向 Ping", icon: LayoutGrid, href: "#/reverse" },
	{ key: "topology", label: "Ping 拓扑", icon: Network, href: "#/topology" },
	{ key: "mapping", label: "全国延迟", icon: Map, href: "#/mapping" },
	{ key: "tools", label: "检测工具", icon: Wrench, href: "#/tools" },
	{ key: "alerts", label: "报警记录", icon: Bell, href: "#/alerts" },
	{ key: "config", label: "系统配置", icon: Settings, href: "#/config", protected: true },
]

export function Navbar({
	active,
	version,
	authenticated = false,
	onLoginClick,
	onLogoutClick,
}: {
	active: PageKey
	version?: string
	authenticated?: boolean
	onLoginClick?: () => void
	onLogoutClick?: () => void
}) {
	return (
		<>
			<header className="sticky top-0 z-40 border-b border-border bg-card/90 backdrop-blur">
				<div className="container flex h-14 items-center justify-between gap-3">
					<a href="#/ping" className="flex items-center gap-2 font-bold tracking-tight">
						<span className="grid size-7 place-items-center rounded-lg bg-gradient-to-br from-[var(--chart-1)] to-[var(--chart-4)] text-white text-sm shadow-xs">
							<Activity className="size-4" />
						</span>
						<span>SmartPingPlus</span>
					</a>

					<div className="flex items-center gap-2 sm:gap-3">
						{version && (
							<span className="hidden rounded-md border border-border/80 bg-muted/40 px-2 py-0.5 text-xs text-muted-foreground sm:inline tabular-nums">
								v{version}
							</span>
						)}

						{authenticated ? (
							<div className="flex items-center gap-1.5">
								<span className="inline-flex items-center gap-1 rounded-md bg-emerald-500/10 px-2 py-1 text-xs font-medium text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
									<ShieldCheck className="size-3.5" />
									<span className="hidden sm:inline">已认证</span>管理员
								</span>
								<Button
									variant="ghost"
									size="sm"
									onClick={onLogoutClick}
									className="h-8 gap-1 px-2 text-xs text-muted-foreground hover:text-destructive"
									title="退出管理员身份"
								>
									<LogOut className="size-3.5" />
									<span className="hidden sm:inline">退出</span>
								</Button>
							</div>
						) : (
							<Button
								variant="outline"
								size="sm"
								onClick={onLoginClick}
								className="h-8 gap-1.5 px-2.5 text-xs font-medium"
								title="管理员登录"
							>
								<LogIn className="size-3.5" />
								<span>登录</span>
							</Button>
						)}
					</div>
				</div>
			</header>
			<nav className="border-b border-border bg-background">
				<div className="container flex gap-1 overflow-x-auto scrollbar-hide">
					{NAV_ITEMS.map(({ key, label, icon: Icon, href, protected: isProtected }) => (
						<a
							key={key}
							href={href}
							className={cn(
								"inline-flex items-center gap-2 whitespace-nowrap border-b-2 px-3.5 py-3 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground",
								active === key && "border-[var(--primary)] text-foreground"
							)}
						>
							<Icon className="size-4" />
							{label}
							{isProtected && !authenticated && (
								<span title="需要管理员认证" className="inline-flex">
									<Lock className="size-3 text-muted-foreground/60" />
								</span>
							)}
						</a>
					))}
				</div>
			</nav>
		</>
	)
}
