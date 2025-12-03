import { z } from "zod";

export enum MessageType {
	FILES_METADATA = "files_metadata",
	DEVICE_INFO = "device_info",
	READY_TO_RECEIVE = "ready_to_receive",
	CHUNK = "chunk",
	ERROR = "error",
	CREATE_ROOM = "create_room",
	JOIN_ROOM = "join_room",
	ROOM_CREATED = "room_created",
	JOIN_SUCCESS = "join_success",
	PEER_JOINED = "peer_joined",
	PEER_LEFT = "peer_left",
	SIGNAL = "signal",
	DOWNLOADING_DONE = "downloading_done",
	CHUNK_ACKNOWLEDGEMENT = "chunk_acknowledgement",
}

export const DeviceInfoMessage = z.object({
	type: z.literal(MessageType.DEVICE_INFO),
	payload: z.object({
		browserName: z.string(),
		browserVersion: z.string(),
		osName: z.string(),
		osVersion: z.string(),
		mobileVendor: z.string(),
		mobileModel: z.string(),
	}),
});

export const FilesMetadataMessage = z.object({
	type: z.literal(MessageType.FILES_METADATA),
	payload: z.array(
		z.object({
			name: z.string(),
			size: z.number(),
			type: z.string(),
		}),
	),
});

export const ReadyToReceiveMessage = z.object({
	type: z.literal(MessageType.READY_TO_RECEIVE),
	payload: z.object({
		fileName: z.string(),
		offset: z.number(),
	}),
});

export const ChunkMessage = z.object({
	type: z.literal(MessageType.CHUNK),
	payload: z.object({
		fileName: z.string(),
		offset: z.number(),
		bytes: z.instanceof(Uint8Array<ArrayBuffer>),
		final: z.boolean(),
	}),
});

export const ChunkAcknowledgmentMessage = z.object({
	type: z.literal(MessageType.CHUNK_ACKNOWLEDGEMENT),
	payload: z.object({
		fileName: z.string(),
		offset: z.number(),
		bytesReceived: z.number(),
	}),
});

export const CreateRoomMessage = z.object({
	type: z.literal(MessageType.CREATE_ROOM),
});

export const JoinRoomMessage = z.object({
	type: z.literal(MessageType.JOIN_ROOM),
	room_id: z.string(),
});

export const RoomCreatedMessage = z.object({
	type: z.literal(MessageType.ROOM_CREATED),
	room_id: z.string(),
});

export const JoinSuccessMessage = z.object({
	type: z.literal(MessageType.JOIN_SUCCESS),
	room_id: z.string(),
});

export const PeerJoinedMessage = z.object({
	type: z.literal(MessageType.PEER_JOINED),
});

export const PeerLeftMessage = z.object({
	type: z.literal(MessageType.PEER_LEFT),
});

export const SignalMessage = z.object({
	type: z.literal(MessageType.SIGNAL),
	payload: z.any(),
});

export const ErrorMessage = z.object({
	type: z.literal("error"),
	payload: z.object({
		error: z.string(),
	}),
});

export const DownloadingDoneMessage = z.object({
	type: z.literal(MessageType.DOWNLOADING_DONE),
});

export const Message = z.discriminatedUnion("type", [
	DeviceInfoMessage,
	FilesMetadataMessage,
	ReadyToReceiveMessage,
	ChunkMessage,
	ErrorMessage,
	CreateRoomMessage,
	JoinRoomMessage,
	RoomCreatedMessage,
	JoinSuccessMessage,
	PeerJoinedMessage,
	PeerLeftMessage,
	SignalMessage,
	DownloadingDoneMessage,
	ChunkAcknowledgmentMessage,
]);

export type Message = z.infer<typeof Message>;
export type MessageOfType<T extends MessageType> = Extract<
	Message,
	{ type: T }
>;

export function parseMessage(data: unknown): Message {
	return Message.parse(data);
}
