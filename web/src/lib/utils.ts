import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs))
}

/** Format a "2006-01-02 15:04" timestamp into a short time "15:04". */
export function shortTime(v?: string) {
	return v && v.length >= 16 ? v.substring(11, 16) : v ?? ""
}

/** Round to a fixed number of decimals. */
export function toFixedFloat(v: number, decimals = 2) {
	return Number(v.toFixed(decimals))
}

/** Status class for a loss value. */
export function lossStatus(loss: number) {
	if (loss >= 100) return "down"
	if (loss > 0) return "warn"
	return "up"
}

/** Beszel chart palette (light theme hex approximations for canvas-free SVG). */
export const CHART_COLORS = [
	"hsl(220 70% 50%)",
	"hsl(160 60% 45%)",
	"hsl(30 80% 55%)",
	"hsl(280 65% 60%)",
	"hsl(340 75% 55%)",
	"hsl(200 70% 55%)",
	"hsl(120 55% 45%)",
	"hsl(20 80% 55%)",
	"hsl(320 60% 55%)",
	"hsl(260 60% 60%)",
]
export const DELAY_COLOR = "#00CC66"
export const LOSS_COLOR = "#FF3B30" // red: only used for abnormal (packet loss)

/** Split a "host:port" target; returns host (and port, if present). */
export function splitHostPort(target: string) {
	const i = target.lastIndexOf(":")
	if (i > -1 && target.indexOf(":") === i) {
		return { host: target.substring(0, i), port: target.substring(i + 1) }
	}
	return { host: target, port: "" }
}
