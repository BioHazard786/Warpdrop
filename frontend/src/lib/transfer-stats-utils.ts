/**
 * Format bytes per second to human-readable speed
 */
export function formatSpeed(bytesPerSecond: number): string {
	if (bytesPerSecond < 1024) {
		return `${bytesPerSecond.toFixed(0)} B/s`;
	}
	if (bytesPerSecond < 1024 * 1024) {
		return `${(bytesPerSecond / 1024).toFixed(1)} KB/s`;
	}
	if (bytesPerSecond < 1024 * 1024 * 1024) {
		return `${(bytesPerSecond / (1024 * 1024)).toFixed(1)} MB/s`;
	}
	return `${(bytesPerSecond / (1024 * 1024 * 1024)).toFixed(1)} GB/s`;
}

/**
 * Format seconds to human-readable ETA
 */
export function formatETA(seconds: number): string {
	if (seconds === 0 || !Number.isFinite(seconds)) {
		return "--";
	}

	if (seconds < 60) {
		return `${Math.round(seconds)}s`;
	}
	if (seconds < 3600) {
		const minutes = Math.floor(seconds / 60);
		const secs = Math.round(seconds % 60);
		return `${minutes}m ${secs}s`;
	}
	const hours = Math.floor(seconds / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);
	return `${hours}h ${minutes}m`;
}

/**
 * Calculate transfer stats (speed and ETA)
 * @param currentBytes - current bytes transferred
 * @param totalBytes - total bytes to transfer
 * @param previousBytes - previous bytes recorded
 * @param timeDiffSeconds - time elapsed in seconds
 * @returns object with formatted speed and eta
 */
export function calculateTransferStats(
	currentBytes: number,
	totalBytes: number,
	previousBytes: number,
	timeDiffSeconds: number,
): { speed: string; eta: string } {
	if (timeDiffSeconds <= 0 || totalBytes === 0) {
		return { speed: "0 B/s", eta: "--" };
	}

	const bytesDiff = currentBytes - previousBytes;
	const instantSpeed = bytesDiff / timeDiffSeconds;

	const speedStr = formatSpeed(instantSpeed);

	const remainingBytes = totalBytes - currentBytes;
	const etaSeconds = instantSpeed > 0 ? remainingBytes / instantSpeed : 0;
	const etaStr = formatETA(etaSeconds);

	return { speed: speedStr, eta: etaStr };
}
