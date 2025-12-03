export function logger(
	role: "sender" | "receiver" | null,
	metaUrl: string,
	...args: any[]
) {
	const timestamp = new Date().toISOString().replace("T", " ").replace("Z", "");
	const file = metaUrl.split("/").pop() || "unknown";

	const roleColorSender = "color: #00bcd4; font-weight: bold;"; // cyan
	const roleColorReceiver = "color: #c084fc; font-weight: bold;"; // purple
	const fileColor = "color: #9ca3af;";
	const timestampColor = "color: #4ade80;";

	// No role → Common log (no role tag, just file + time)
	if (!role) {
		console.log(
			`%c[${file}] %c${timestamp}`,
			fileColor,
			timestampColor,
			...args,
		);
		return;
	}

	// With role → styled prefix
	const roleColor = role === "sender" ? roleColorSender : roleColorReceiver;

	console.log(
		`%c[${role.toUpperCase()}] %c[${file}] %c${timestamp}`,
		roleColor,
		fileColor,
		timestampColor,
		...args,
	);
}
