import { decode, encode } from "@msgpack/msgpack";
import type { Message } from "@/lib/messages";

// --- Configuration ---
export const PEER_CONNECTION_CONFIG: RTCConfiguration = {
	iceServers: [
		{ urls: "stun:stun.l.google.com:19302" },
		{ urls: "stun:stun1.l.google.com:19302" },
	],
};

// Keep individual send sizes conservative to avoid DC closure from oversize frames
export const MAX_CHUNK_SIZE = 60 * 1024; // ~60 KB
export const HIGH_WATER_MARK = 2 * 1024 * 1024; // 2 MB
export const LOW_WATER_MARK = 512 * 1024; // 512 KB

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
