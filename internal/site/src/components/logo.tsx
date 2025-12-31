export function Logo({ className, variant = "wordmark" }: { className?: string; variant?: "wordmark" | "icon" }) {
	return (
		<img
			src={variant === "icon" ? "/static/icon.svg" : "/static/logo.svg"}
			className={className}
			alt="Aether"
		/>
	)
}
