// API layer for the SmartPing backend (/api/* endpoints).
// Mirrors the data shapes returned by src/http/api.go.

export interface AlertRule {
	Addr: string
	Name?: string
	Thdavgdelay?: string
	Thdchecksec?: string
	Thdloss?: string
	Thdoccnum?: string
}

export interface Config {
	Ver: string
	Port: number
	Name: string
	Addr: string
	Mode: Record<string, string>
	Base: Record<string, number>
	Topology: Record<string, string>
	Alert: Record<string, string>
	//PingTargets 是 ICMP 检测目标：目标地址 -> 显示名称
	PingTargets: Record<string, string>
	//TcpTargets 是 TCP 检测目标：host:port -> 显示名称
	TcpTargets: Record<string, string>
	AlertRules: AlertRule[]
	Chinamap: Record<string, { ctcc: string[]; cucc: string[]; cmcc: string[] }>
	Toollimit: number
	Authiplist: string
	Password: string
}

export interface PingData {
	lastcheck: string[]
	maxdelay: string[]
	mindelay: string[]
	avgdelay: string[]
	losspk: string[]
}

export interface TopoResult {
	[addr: string]: string
}

export interface MappingData {
	text: string
	subtext: string
	avgdelay: { ctcc: { name: string; value: number }[]; cucc: { name: string; value: number }[]; cmcc: { name: string; value: number }[] }
}

export interface AlertLog {
	Logtime: string
	Fromname: string
	Fromip: string
	Targetname: string
	Targetip: string
	Tracert: string
}

export interface ToolsRes {
	status: string
	error?: string
	ip: string
	ping: { SendPk: number; RevcPk: number; LossPk: number; MaxDelay: number; MinDelay: number; AvgDelay: number }
}

const TOKEN_KEY = "smartping_token"

/** Get stored JWT token */
export function getStoredToken(): string | null {
	try {
		return localStorage.getItem(TOKEN_KEY)
	} catch {
		return null
	}
}

/** Save JWT token and notify listeners */
export function setStoredToken(token: string): void {
	try {
		localStorage.setItem(TOKEN_KEY, token)
		window.dispatchEvent(new CustomEvent("smartping-auth-change", { detail: { authenticated: true } }))
	} catch {}
}

/** Clear stored JWT token and notify listeners */
export function clearStoredToken(): void {
	try {
		localStorage.removeItem(TOKEN_KEY)
		window.dispatchEvent(new CustomEvent("smartping-auth-change", { detail: { authenticated: false } }))
	} catch {}
}

/** Listen for auth state changes */
export function onAuthChange(callback: (authed: boolean) => void): () => void {
	const handler = (e: Event) => {
		const custom = e as CustomEvent<{ authenticated: boolean }>
		callback(custom.detail?.authenticated ?? !!getStoredToken())
	}
	window.addEventListener("smartping-auth-change", handler)
	return () => window.removeEventListener("smartping-auth-change", handler)
}

/** Build request headers including Authorization if token exists */
function buildHeaders(extra?: Record<string, string>): HeadersInit {
	const headers: Record<string, string> = { ...extra }
	const token = getStoredToken()
	if (token) {
		headers["Authorization"] = `Bearer ${token}`
	}
	return headers
}

/** fetch helper: throw on non-ok, parse json, auto-inject auth token */
export async function get<T>(url: string): Promise<T> {
	const res = await fetch(url, {
		headers: buildHeaders(),
		credentials: "same-origin",
	})
	if (res.status === 401) {
		// If unauthorized and we had a stored token, clear it
		if (getStoredToken()) {
			clearStoredToken()
		}
		throw new Error("401 Unauthorized: 需要管理员权限")
	}
	if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
	return (await res.json()) as T
}

/** POST helper: form-urlencoded body with auto-injected auth token */
export async function post<T>(url: string, data: Record<string, string>): Promise<T> {
	const res = await fetch(url, {
		method: "POST",
		headers: buildHeaders({ "Content-Type": "application/x-www-form-urlencoded" }),
		body: new URLSearchParams(data).toString(),
		credentials: "same-origin",
	})
	if (res.status === 401) {
		if (getStoredToken()) {
			clearStoredToken()
		}
		throw new Error("401 Unauthorized: 需要管理员权限")
	}
	if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
	return (await res.json()) as T
}

export interface LoginResponse {
	status: boolean | string
	token?: string
	info?: string
	error?: string
	expires_in?: number
}

/** Admin login */
export async function login(password: string): Promise<LoginResponse> {
	const res = await fetch("/api/login.json", {
		method: "POST",
		headers: { "Content-Type": "application/x-www-form-urlencoded" },
		body: new URLSearchParams({ password }).toString(),
		credentials: "same-origin",
	})
	const data = (await res.json()) as LoginResponse
	const ok = data.status === true || data.status === "true"
	if (ok && data.token) {
		setStoredToken(data.token)
	}
	return data
}

/** Admin logout */
export async function logout(): Promise<void> {
	try {
		await fetch("/api/logout.json", {
			headers: buildHeaders(),
			credentials: "same-origin",
		})
	} catch {
		// ignore network errors during logout
	} finally {
		clearStoredToken()
	}
}

/** Query server authentication status */
export async function fetchAuthStatus(): Promise<{ authenticated: boolean; username?: string }> {
	try {
		const res = await get<{ authenticated: boolean; username?: string }>("/api/auth/status.json")
		if (!res.authenticated && getStoredToken()) {
			clearStoredToken()
		}
		return res
	} catch {
		return { authenticated: false }
	}
}

export function apiUrl(path: string, params?: Record<string, string>) {
	const u = new URL(path, window.location.origin)
	if (params) {
		for (const [k, v] of Object.entries(params)) u.searchParams.set(k, v)
	}
	return u.toString()
}

export const fetchConfig = (proxy?: string) =>
	get<Config>(proxy ? apiUrl("/api/proxy.json", { g: proxy + "/api/config.json" }) : "/api/config.json")

export const fetchPing = (ip: string, starttime?: string, endtime?: string, proxy?: string) => {
	const p: Record<string, string> = { ip }
	if (starttime && endtime) {
		p.starttime = starttime
		p.endtime = endtime
	}
	return get<PingData>(proxy ? apiUrl("/api/proxy.json", { g: proxy + "/api/ping.json?" + new URLSearchParams(p).toString() }) : apiUrl("/api/ping.json", p))
}

export const fetchTcpping = (ip: string, starttime?: string, endtime?: string, proxy?: string) => {
	const p: Record<string, string> = { ip }
	if (starttime && endtime) {
		p.starttime = starttime
		p.endtime = endtime
	}
	return get<PingData>(proxy ? apiUrl("/api/proxy.json", { g: proxy + "/api/tcpping.json?" + new URLSearchParams(p).toString() }) : apiUrl("/api/tcpping.json", p))
}

export const fetchTopology = (proxy: string) => get<TopoResult>(apiUrl("/api/proxy.json", { g: proxy + "/api/topology.json" }))

export const fetchMapping = (proxy?: string, date?: string) =>
	get<MappingData>(proxy ? apiUrl("/api/proxy.json", { g: proxy + "/api/mapping.json" + (date ? "?d=" + date : "") }) : apiUrl("/api/mapping.json", date ? { d: date } : undefined))

export const fetchAlerts = (proxy: string, date?: string) =>
	get<[string[], AlertLog[]]>(apiUrl("/api/proxy.json", { g: proxy + "/api/alert.json" + (date ? "?date=" + date : "") }))

export const fetchTools = (proxyAddr: string, target: string) =>
	get<ToolsRes>(apiUrl("/api/proxy.json", { g: "http://" + proxyAddr + "/api/tools.json?t=" + encodeURIComponent(target) }))
