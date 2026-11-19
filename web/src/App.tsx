import { lazy, Suspense, useEffect, useState } from "react"
import { Navbar, type PageKey } from "@/components/navbar"
import { LoginDialog } from "@/components/login-dialog"
import { fetchAuthStatus, fetchConfig, getStoredToken, logout, onAuthChange } from "@/lib/api"

// route-level code splitting: chart-heavy pages load only when visited
const DashboardPage = lazy(() => import("@/components/dashboard").then((m) => ({ default: m.DashboardPage })))
const TopologyPage = lazy(() => import("@/components/topology").then((m) => ({ default: m.TopologyPage })))
const MappingPage = lazy(() => import("@/components/mapping").then((m) => ({ default: m.MappingPage })))
const ToolsPage = lazy(() => import("@/components/tools").then((m) => ({ default: m.ToolsPage })))
const AlertsPage = lazy(() => import("@/components/alerts").then((m) => ({ default: m.AlertsPage })))
const ConfigPage = lazy(() => import("@/components/config").then((m) => ({ default: m.ConfigPage })))

function parseHash(): PageKey {
	const h = window.location.hash.replace(/^#\/?/, "")
	switch (h) {
		case "ping":
		case "":
			return "ping"
		case "tcpping":
			return "tcpping"
		case "reverse":
			return "reverse"
		case "topology":
			return "topology"
		case "mapping":
			return "mapping"
		case "tools":
			return "tools"
		case "alerts":
			return "alerts"
		case "config":
			return "config"
		default:
			return "ping"
	}
}

export default function App() {
	const [page, setPage] = useState<PageKey>(parseHash)
	const [version, setVersion] = useState("")
	const [activeNode, setActiveNode] = useState("")
	const [authenticated, setAuthenticated] = useState(() => !!getStoredToken())
	const [loginOpen, setLoginOpen] = useState(false)

	useEffect(() => {
		const onHash = () => setPage(parseHash())
		window.addEventListener("hashchange", onHash)
		return () => window.removeEventListener("hashchange", onHash)
	}, [])

	useEffect(() => {
		window.scrollTo(0, 0)
	}, [page])

	// 动态从后端获取版本号与系统基础配置
	useEffect(() => {
		fetchConfig()
			.then((c) => {
				if (c?.Ver) setVersion(c.Ver)
			})
			.catch(() => {})
	}, [])

	// 检查鉴权状态并监听登录/登出事件
	useEffect(() => {
		fetchAuthStatus().then((res) => {
			setAuthenticated(res.authenticated)
		})
		const unsubscribe = onAuthChange((authed) => {
			setAuthenticated(authed)
		})
		return unsubscribe
	}, [])

	const handleLogout = async () => {
		await logout()
		setAuthenticated(false)
	}

	return (
		<div>
			<Navbar
				active={page}
				version={version}
				authenticated={authenticated}
				onLoginClick={() => setLoginOpen(true)}
				onLogoutClick={handleLogout}
			/>
			<main className="container min-h-[calc(100vh-15rem)] py-5">
				<Suspense
					fallback={
						<div className="flex flex-col items-center gap-3 py-16 text-sm text-muted-foreground">
							<span className="size-6 animate-spin rounded-full border-2 border-muted-foreground/25 border-t-muted-foreground" />
							页面加载中…
						</div>
					}
				>
					{page === "ping" && <DashboardPage mode="ping" activeNode={activeNode} onNodeChange={setActiveNode} />}
					{page === "tcpping" && <DashboardPage mode="tcpping" activeNode={activeNode} onNodeChange={setActiveNode} />}
					{page === "reverse" && <DashboardPage mode="reverse" activeNode={activeNode} onNodeChange={setActiveNode} />}
					{page === "topology" && <TopologyPage />}
					{page === "mapping" && <MappingPage />}
					{page === "tools" && <ToolsPage />}
					{page === "alerts" && <AlertsPage />}
					{page === "config" && (
						<ConfigPage authenticated={authenticated} onLoginClick={() => setLoginOpen(true)} />
					)}
				</Suspense>
			</main>
			<footer className="border-t border-border py-4">
				<div className="container flex items-center justify-between text-xs text-muted-foreground">
					<span>SmartPingPlus · 网络质量监控系统</span>
					{version && <div>Version: {version}</div>}
				</div>
			</footer>

			<LoginDialog
				open={loginOpen}
				onOpenChange={setLoginOpen}
				onSuccess={() => setAuthenticated(true)}
			/>
		</div>
	)
}
