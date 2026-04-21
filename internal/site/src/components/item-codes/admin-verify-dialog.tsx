import { useState } from "react"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { toast } from "@/components/ui/use-toast"
import { Trans } from "@lingui/react/macro"
import { pb } from "@/lib/api"
import { LoaderCircleIcon } from "lucide-react"

interface AdminVerifyDialogProps {
	open: boolean
	onOpenChange: (open: boolean) => void
	onVerified: () => void
}

export default function AdminVerifyDialog({ open, onOpenChange, onVerified }: AdminVerifyDialogProps) {
	const [email, setEmail] = useState("")
	const [password, setPassword] = useState("")
	const [verifying, setVerifying] = useState(false)

	const handleVerify = async () => {
		if (!email.trim() || !password.trim()) {
			toast({ title: "请输入邮箱和密码", variant: "destructive" })
			return
		}
		setVerifying(true)
		try {
			const res = (await pb.send("/api/collections/users/auth-with-password", {
				method: "POST",
				body: { identity: email.trim(), password },
			})) as { record?: { role?: string } }
			if (res?.record?.role === "admin") {
				onVerified()
				onOpenChange(false)
				setEmail("")
				setPassword("")
			} else {
				toast({ title: "该账号不是管理员", variant: "destructive" })
			}
		} catch (err: any) {
			toast({ title: err.message || "验证失败", variant: "destructive" })
		} finally {
			setVerifying(false)
		}
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>
						<Trans>管理员验证</Trans>
					</DialogTitle>
					<DialogDescription>
						<Trans>请输入管理员账号和密码以继续操作。</Trans>
					</DialogDescription>
				</DialogHeader>
				<div className="space-y-4 py-2">
					<div className="space-y-2">
						<Label>
							<Trans>邮箱</Trans>
						</Label>
						<Input
							type="email"
							value={email}
							onChange={(e) => setEmail(e.target.value)}
							placeholder="admin@example.com"
						/>
					</div>
					<div className="space-y-2">
						<Label>
							<Trans>密码</Trans>
						</Label>
						<Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
					</div>
				</div>
				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						<Trans>取消</Trans>
					</Button>
					<Button onClick={() => void handleVerify()} disabled={verifying}>
						{verifying && <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />}
						<Trans>验证</Trans>
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
}
