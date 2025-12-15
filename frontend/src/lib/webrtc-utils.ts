import { decode, encode } from "@msgpack/msgpack";
import type { Message } from "@/lib/messages";

// --- Configuration ---
function getIceServers(): RTCIceServer[] {
	const stunServer =
		process.env.NEXT_PUBLIC_STUN_SERVER || "stun:stun.l.google.com:19302";

	const servers: RTCIceServer[] = [{ urls: stunServer }];

	const turnServer = process.env.NEXT_PUBLIC_TURN_SERVER;
	if (turnServer) {
		const turnUsername = process.env.NEXT_PUBLIC_TURN_USERNAME || "warpdrop";
		const turnPassword =
			process.env.NEXT_PUBLIC_TURN_PASSWORD || "warpdrop-secret";

		servers.push({
			urls: [
				`turn:${turnServer}:3478?transport=udp`,
				`turn:${turnServer}:3478?transport=tcp`,
				`turns:${turnServer}:5349?transport=tcp`,
			],
			username: turnUsername,
			credential: turnPassword,
		});
	}

	return servers;
}

export const PEER_CONNECTION_CONFIG: RTCConfiguration = {
	iceServers: getIceServers(),
};

// --- Dynamic Chunk Size Configuration ---
// Chunk sizes adapt based on measured transfer speed
export const MIN_CHUNK_SIZE = 4 * 1024; // 4 KB - for very slow connections
export const MAX_CHUNK_SIZE = 64 * 1024; // 64 KB - for fast connections
export const DEFAULT_CHUNK_SIZE = 16 * 1024; // 16 KB - starting size
export const HIGH_WATER_MARK = 2 * 1024 * 1024; // 2 MB
export const LOW_WATER_MARK = 512 * 1024; // 512 KB

// Speed thresholds for chunk size adjustment (in bytes per second)
const SPEED_THRESHOLDS = {
	VERY_SLOW: 50 * 1024, // < 50 KB/s
	SLOW: 200 * 1024, // < 200 KB/s
	MEDIUM: 500 * 1024, // < 500 KB/s
	FAST: 1 * 1024 * 1024, // < 1 MB/s
	// > 1 MB/s = VERY_FAST
};

/**
 * Calculate optimal chunk size based on current transfer speed.
 * Slower connections benefit from smaller chunks (less retransmission on failure),
 * while faster connections benefit from larger chunks (less overhead).
 *
 * @param currentSpeed - Current transfer speed in bytes per second
 * @param currentChunkSize - Current chunk size being used
 * @returns Optimal chunk size in bytes
 */
export function calculateDynamicChunkSize(
	currentSpeed: number,
	currentChunkSize: number = DEFAULT_CHUNK_SIZE,
): number {
	let targetChunkSize: number;

	if (currentSpeed <= 0) {
		// No speed data yet, use current or default
		return currentChunkSize;
	}

	if (currentSpeed < SPEED_THRESHOLDS.VERY_SLOW) {
		// Very slow connection (< 50 KB/s): use minimum chunk size
		targetChunkSize = MIN_CHUNK_SIZE;
	} else if (currentSpeed < SPEED_THRESHOLDS.SLOW) {
		// Slow connection (50-200 KB/s): use small chunks
		targetChunkSize = 8 * 1024; // 8 KB
	} else if (currentSpeed < SPEED_THRESHOLDS.MEDIUM) {
		// Medium-slow connection (200-500 KB/s): use medium-small chunks
		targetChunkSize = 16 * 1024; // 16 KB
	} else if (currentSpeed < SPEED_THRESHOLDS.FAST) {
		// Medium-fast connection (500 KB/s - 1 MB/s): use medium chunks
		targetChunkSize = 32 * 1024; // 32 KB
	} else {
		// Fast connection (> 1 MB/s): use large chunks
		targetChunkSize = MAX_CHUNK_SIZE; // 64 KB
	}

	// Smooth transitions: don't change chunk size too drastically at once
	// Move 25% toward the target to avoid oscillation
	const smoothedChunkSize = Math.round(
		currentChunkSize + (targetChunkSize - currentChunkSize) * 0.25,
	);

	// Clamp to valid range
	return Math.max(MIN_CHUNK_SIZE, Math.min(MAX_CHUNK_SIZE, smoothedChunkSize));
}

// --- Utility Functions ---
export function validateOffset(
	files: File[],
	fileName: string,
	offset: number,
): File {
	const validFile = files.find(
		(file) => file.name === fileName && offset <= file.size,
	);
	if (!validFile) {
		throw new Error("invalid file offset");
	}
	return validFile;
}

export function getZipFilename(): string {
	return `warpdrop-download-${Date.now()}.zip`;
}

export function packMessage(message: Message): Uint8Array<ArrayBuffer> {
	return new Uint8Array(encode(message));
}

export function unpackMessage(data: ArrayBuffer | Uint8Array): Message {
	return decode(data) as Message;
}
