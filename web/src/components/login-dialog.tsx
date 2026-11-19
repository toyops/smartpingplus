import { useState } from "react"
import * as Dialog from "@radix-ui/react-dialog"
import { Eye, EyeOff, KeyRound, Loader2, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { login } from "@/lib/api"

export function LoginDialog({
	open,
	onOpenChange,
	onSuccess,
}: {
	open: boolean
	onOpenChange: (open: boolean) => void
	onSuccess?: () => void
}) {
	const [password, setPassword] = useState("")
	const [showPassword, setShowPassword] = useState(false)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState("")

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault()
		if (!password.trim()) {
			setError("请输入管理员密码")
			return
		}
		setLoading(true)
		setError("")
		try {
			const res = await login(password)
			if (res.status === true || res.status === "true") {
				setPassword("")
				onOpenChange(false)
				onSuccess?.()
			} else {
				setError(res.info || res.error || "密码错误，请重试")
			}
		} catch (err) {
			setError((err as Error).message || "登录请求失败")
		} finally {
			setLoading(false)
		}
	}

	return (
		<Dialog.Root open={open} onOpenChange={onOpenChange}>
			<Dialog.Portal>
				<Dialog.Overlay className="fixed inset-0 z-50 bg-black/45 backdrop-blur-xs" />
				<Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[min(92vw,420px)] -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-card p-6 shadow-xl outline-none">
					<div className="flex items-start justify-between">
						<div className="flex items-center gap-2 text-foreground">
							<div className="grid size-8 place-items-center rounded-lg bg-primary/10 text-primary">
								<KeyRound className="size-4" />
							</div>
							<div>
								<Dialog.Title className="text-base font-semibold">管理员登录</Dialog.Title>
								<Dialog.Description className="text-xs text-muted-foreground">
									验证管理员密码后可编辑系统与监控配置
								</Dialog.Description>
							</div>
						</div>
						<Dialog.Close asChild>
							<Button variant="ghost" size="icon" className="size-8 text-muted-foreground hover:text-foreground">
								<X className="size-4" />
							</Button>
						</Dialog.Close>
					</div>

					<form onSubmit={handleSubmit} className="mt-5 space-y-4">
						<div className="space-y-1.5">
							<label className="text-xs font-medium text-foreground">管理员密码</label>
							<div className="relative">
								<Input
									type={showPassword ? "text" : "password"}
									autoFocus
									placeholder="输入管理员密码…"
									value={password}
									onChange={(e) => {
										setPassword(e.target.value)
										if (error) setError("")
									}}
									className="pr-10"
								/>
								<button
									type="button"
									onClick={() => setShowPassword((p) => !p)}
									className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
									tabIndex={-1}
								>
									{showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
								</button>
							</div>
						</div>

						{error && (
							<div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
								{error}
							</div>
						)}

						<div className="flex items-center justify-end gap-2 pt-1">
							<Button
								type="button"
								variant="outline"
								onClick={() => onOpenChange(false)}
								disabled={loading}
							>
								取消
							</Button>
							<Button type="submit" disabled={loading || !password.trim()}>
								{loading && <Loader2 className="mr-1.5 size-4 animate-spin" />}
								{loading ? "验证中…" : "登录"}
							</Button>
						</div>
					</form>
				</Dialog.Content>
			</Dialog.Portal>
		</Dialog.Root>
	)
}
