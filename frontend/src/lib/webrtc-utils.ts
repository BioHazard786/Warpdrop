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

// Chunk sizes
export const CHUNK_SIZE = 60 * 1024; // 60 KB 
export const LOW_WATER_MARK = 512 * 1024; // 512 KB
export const HIGH_WATER_MARK = 2 * 1024 * 1024; // 2 MB


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
