import {
	browserName,
	browserVersion,
	mobileModel,
	mobileVendor,
	osName,
	osVersion,
} from "react-device-detect";
import type { SendJsonMessage } from "react-use-websocket/dist/lib/types";
import {
	streamDownloadMultipleFiles,
	streamDownloadSingleFile,
} from "@/lib/download-utils";
import { logger } from "@/lib/logger";
import { type MessageOfType, MessageType, parseMessage } from "@/lib/messages";
import {
	getZipFilename,
	HIGH_WATER_MARK,
	LOW_WATER_MARK,
	MAX_CHUNK_SIZE,
	PEER_CONNECTION_CONFIG,
	packMessage,
	unpackMessage,
	validateOffset,
} from "@/lib/webrtc-utils";
import useFileUploadStore from "@/store/use-file-upload-store";
import useReceiverStore, { ReceiverStatus } from "@/store/use-receiver-store";
import useRoleStore from "@/store/use-role-store";
import useRTCStore from "@/store/use-rtc-store";
import useSenderStore, { SenderStatus } from "@/store/use-sender-store";

// ==================================================================
// WebRTC Peer Connection and Data Channel Logic
// ==================================================================

// Creates and configures a new RTCPeerConnection with data channel setup
export function createPeerConnection(
	sendMessage: SendJsonMessage,
): RTCPeerConnection {
	const pc = new RTCPeerConnection(PEER_CONNECTION_CONFIG);
	const { setDataChannel, setPeerConnection } = useRTCStore.getState().actions;
	const { isSender } = useRoleStore.getState();
	const senderActions = useSenderStore.getState().actions;
	const receiverActions = useReceiverStore.getState().actions;

	// Initial status while gathering ICE and negotiating
	if (isSender) senderActions.setStatus(SenderStatus.CONNECTING);
	else receiverActions.setStatus(ReceiverStatus.CONNECTING);

	// Send ICE candidates to the other peer via signaling server
	pc.onicecandidate = (event) => {
		if (event.candidate) {
			sendMessage({
				type: "signal",
				payload: { ice_candidate: event.candidate },
			});
		}
	};

	// Sender creates data channel; receiver waits to receive it
	if (isSender) {
		const dataChannel = pc.createDataChannel("file-transfer");
		dataChannel.binaryType = "arraybuffer";
		setupDataChannelHandlers(dataChannel);
		setDataChannel(dataChannel);
	} else {
		pc.ondatachannel = (event) => {
			const dataChannel = event.channel;
			dataChannel.binaryType = "arraybuffer";
			setupDataChannelHandlers(dataChannel);
			setDataChannel(dataChannel);
		};
	}

	setPeerConnection(pc);
	return pc;
}

// Sets up all event handlers for the data channel (open, close, message, error)
export function setupDataChannelHandlers(dataChannel: RTCDataChannel) {
	// Once channel opens, sender shares file metadata, receiver shares device info
	const { isSender } = useRoleStore.getState();
	const { resetWebRTC } = useRTCStore.getState().actions;
	const senderActions = useSenderStore.getState().actions;
	const receiverActions = useReceiverStore.getState().actions;

	dataChannel.onopen = () => {
		if (isSender) {
			sendFilesMetaData();
			senderActions.setStatus(SenderStatus.READY);
		} else {
			sendDeviceInfo();
			receiverActions.setStatus(ReceiverStatus.READY);
		}

		logger(null, import.meta.url, "Data channel is open.");
	};

	// Clean up when channel closes
	dataChannel.onclose = () => {
		logger(null, import.meta.url, "Data channel is closed.");
		resetWebRTC();
		if (isSender) {
			senderActions.setStatus(SenderStatus.IDLE);
			senderActions.removeConnectedDevice();
		}
	};

	dataChannel.onerror = (err) => {
		logger(null, import.meta.url, "Data channel error:", err);

		const { isSender } = useRoleStore.getState();
		const errMsg = `${err.error.name} - ${err.error.message}`;

		if (isSender) {
			senderActions.setError(errMsg);
		} else {
			receiverActions.setError("Sender has aborted the connection.");
		}
	};

	// Route incoming messages based on type
	dataChannel.onmessage = (event) => {
		const { setFilesMetadata } = useReceiverStore.getState().actions;
		const { setConnectedDevice } = useSenderStore.getState().actions;
		const senderActions = useSenderStore.getState().actions;
		const receiverActions = useReceiverStore.getState().actions;
		const { isSender } = useRoleStore.getState();

		try {
			// Unpack and parse the binary message
			const message = parseMessage(unpackMessage(event.data));

			switch (message.type) {
				case MessageType.FILES_METADATA:
					setFilesMetadata(message.payload);
					logger(
						"receiver",
						import.meta.url,
						"Received files metadata:",
						message.payload,
					);
					break;

				case MessageType.DEVICE_INFO:
					setConnectedDevice(message.payload);
					logger(
						"sender",
						import.meta.url,
						"Received device info:",
						message.payload,
					);
					break;

				case MessageType.READY_TO_RECEIVE:
					sendFiles(message.payload);
					senderActions.setStatus(SenderStatus.SENDING);
					logger(
						"sender",
						import.meta.url,
						"Receiver is ready to receive files",
					);
					break;

				case MessageType.CHUNK:
					if (!isSender)
						receiverActions.setStatus(ReceiverStatus.RECEIVING_FILE);
					handleReceivedChunk(message.payload);
					break;

				case MessageType.DOWNLOADING_DONE:
					if (isSender) senderActions.setStatus(SenderStatus.COMPLETED);
					else receiverActions.setStatus(ReceiverStatus.COMPLETED);
					logger(
						"sender",
						import.meta.url,
						"Receiver has completed downloading all files",
					);
					break;
			}
		} catch (error) {
			logger(null, import.meta.url, "Error parsing message:", error);
			const errMsg = error instanceof Error ? error.message : String(error);
			if (isSender) {
				senderActions.setError(errMsg);
			} else {
				receiverActions.setError(errMsg);
			}
		}
	};
}

// Sender: Send file names, sizes, and types to receiver
function sendFilesMetaData() {
	const { files } = useFileUploadStore.getState();
	const { dataChannel } = useRTCStore.getState();

	logger("sender", import.meta.url, "Sending files metadata");

	dataChannel?.send(
		packMessage({
			type: MessageType.FILES_METADATA,
			payload: files.map(({ file }) => ({
				name: file.name,
				size: file.size,
				type: file.type,
			})),
		}),
	);
}

// Receiver: Send browser/OS info to sender for display
function sendDeviceInfo() {
	const { dataChannel } = useRTCStore.getState();

	logger("receiver", import.meta.url, "Sending device info");

	dataChannel?.send(
		packMessage({
			type: MessageType.DEVICE_INFO,
			payload: {
				browserName,
				browserVersion,
				osName,
				osVersion,
				mobileVendor,
				mobileModel,
			},
		}),
	);
}

// Sender: Continuously stream file chunks using bufferedAmount-based backpressure
async function streamFileChunks(fileName: string, startOffset: number) {
	const { dataChannel } = useRTCStore.getState();
	const { files } = useFileUploadStore.getState();
	const senderActions = useSenderStore.getState().actions;

	const file = validateOffset(
		files.map(({ file }) => file),
		fileName,
		startOffset,
	);

	if (!dataChannel || dataChannel.readyState !== "open") {
		logger("sender", import.meta.url, "DataChannel not open; aborting send");
		return;
	}

	// Set up backpressure threshold
	dataChannel.bufferedAmountLowThreshold = LOW_WATER_MARK;

	let currentOffset = startOffset;

	// Helper to wait for buffer to drain
	const waitForBufferDrain = (): Promise<void> => {
		return new Promise((resolve) => {
			if (dataChannel.bufferedAmount <= LOW_WATER_MARK) {
				resolve();
			} else {
				dataChannel.onbufferedamountlow = () => {
					dataChannel.onbufferedamountlow = null;
					resolve();
				};
			}
		});
	};

	// Pump chunks continuously until file is complete
	while (currentOffset < file.size) {
		// Check if channel is still open
		if (dataChannel.readyState !== "open") {
			logger("sender", import.meta.url, "DataChannel closed during transfer");
			return;
		}

		// Wait for buffer to have space if needed
		if (dataChannel.bufferedAmount > HIGH_WATER_MARK) {
			await waitForBufferDrain();
		}

		// Calculate chunk boundaries
		const nextChunkEnd = Math.min(currentOffset + MAX_CHUNK_SIZE, file.size);
		const isLastChunk = nextChunkEnd >= file.size;

		// Read chunk from file as binary data
		const chunkBlob = file.slice(currentOffset, nextChunkEnd);
		const arrayBuffer = await chunkBlob.arrayBuffer();

		const chunkMessage: MessageOfType<MessageType.CHUNK> = {
			type: MessageType.CHUNK,
			payload: {
				fileName,
				offset: currentOffset,
				bytes: new Uint8Array(arrayBuffer),
				final: isLastChunk,
			},
		};

		try {
			const payloadBytes = packMessage(chunkMessage);
			dataChannel.send(payloadBytes);

			// Update progress based on bytes sent (not acknowledged)
			senderActions.setCurrentFileOffset(nextChunkEnd);
			senderActions.setCurrentFileProgress(nextChunkEnd / file.size);
			senderActions.setStatus(SenderStatus.SENDING);

			// logger(
			// 	"sender",
			// 	import.meta.url,
			// 	`Sent chunk for ${fileName} (${currentOffset}-${nextChunkEnd}) total size=${file.size} final=${isLastChunk}`,
			// );

			currentOffset = nextChunkEnd;
		} catch (err) {
			logger("sender", import.meta.url, "Error sending chunk", err);

			if (dataChannel.readyState === "open")
				senderActions.setError(
					err instanceof Error ? err.message : String(err),
				);
			return;
		}
	}

	// File transfer complete
	logger("sender", import.meta.url, `Finished sending file: ${fileName}`);

	const { completedFileCount } = useSenderStore.getState();
	senderActions.setCompletedFileCount(completedFileCount + 1);
	senderActions.setCurrentFileProgress(0);

	// Check if all files completed
	if (completedFileCount + 1 >= files.length) {
		// Don't set completed here - wait for DOWNLOADING_DONE from receiver
		logger(
			"sender",
			import.meta.url,
			"All files sent, waiting for receiver confirmation",
		);
	}
}

// Sender: Initiate file transfer when receiver signals ready
function sendFiles(
	fileInfo: MessageOfType<MessageType.READY_TO_RECEIVE>["payload"],
) {
	const {
		actions: { setCurrentFileName, setCurrentFileOffset },
	} = useSenderStore.getState();

	const targetFileName = fileInfo.fileName;
	const startOffset = fileInfo.offset;

	logger(
		"sender",
		import.meta.url,
		`Starting to send file: ${targetFileName} from offset: ${startOffset}`,
	);

	// Initialize state for new file transfer
	setCurrentFileName(targetFileName);
	setCurrentFileOffset(startOffset);
	// Stream file using bufferedAmount-based backpressure
	void streamFileChunks(targetFileName, startOffset);
}

// Receiver: Set up streams for downloading files and start download
export function initializeFileDownload() {
	console.time("download-time-taken");
	const { dataChannel } = useRTCStore.getState();
	const { filesMetadata } = useReceiverStore.getState();

	const receiverActions = useReceiverStore.getState().actions;

	if (!dataChannel || !filesMetadata) {
		logger(
			"receiver",
			import.meta.url,
			"Data channel or files metadata not available for receiving files",
		);
		return;
	}

	// Create ReadableStream for each file to pipe chunks to browser download
	const newFileStreamsByName: Record<
		string,
		{
			stream: ReadableStream<Uint8Array>;
			enqueue: (chunk: Uint8Array) => void;
			close: () => void;
			isClosed: boolean;
		}
	> = {};

	const readableStreams = filesMetadata.map((fileMetadata) => {
		let enqueueChunk: ((chunk: Uint8Array) => void) | null = null;
		let closeStream: (() => void) | null = null;
		let isClosed = false;

		// Create stream with controller for chunk queueing
		const stream = new ReadableStream<Uint8Array>({
			start(controller) {
				// Expose enqueue function to push chunks into stream
				enqueueChunk = (chunk: Uint8Array) => {
					try {
						if (!isClosed) {
							controller.enqueue(chunk);
						}
					} catch (err) {
						logger("receiver", import.meta.url, "Error enqueuing chunk:", err);
						isClosed = true;
					}
				};
				// Expose close function to signal end of stream
				closeStream = () => {
					try {
						if (!isClosed) {
							controller.close();
							isClosed = true;
						}
					} catch (err) {
						logger("receiver", import.meta.url, "Error closing stream:", err);
						isClosed = true;
					}
				};
			},
			cancel(reason) {
				logger(
					"receiver",
					import.meta.url,
					"Stream cancelled (possibly by download manager):",
					reason,
				);
				isClosed = true;
			},
		});

		if (!enqueueChunk || !closeStream)
			throw new Error("Failed to initialize stream controllers");

		newFileStreamsByName[fileMetadata.name] = {
			stream,
			enqueue: enqueueChunk,
			close: closeStream,
			isClosed: false,
		};
		return stream;
	});

	receiverActions.setFileStreamsByName(newFileStreamsByName);

	const downloadDescriptors = filesMetadata.map((fileMetadata, index) => ({
		name: fileMetadata.name.replace(/^\//, ""),
		size: fileMetadata.size,
		stream: () => readableStreams[index],
	}));

	// Start download: zip multiple files or download single file directly
	const downloadPromise =
		downloadDescriptors.length > 1
			? streamDownloadMultipleFiles(downloadDescriptors, getZipFilename())
			: streamDownloadSingleFile(
					downloadDescriptors[0],
					downloadDescriptors[0].name,
				);

	// Notify sender when all downloads complete
	downloadPromise
		.then(() => {
			console.timeEnd("download-time-taken");
			logger("receiver", import.meta.url, "All files downloaded successfully");
			const doneMessage: MessageOfType<MessageType.DOWNLOADING_DONE> = {
				type: MessageType.DOWNLOADING_DONE,
			};
			dataChannel.send(packMessage(doneMessage));
			receiverActions.setStatus(ReceiverStatus.COMPLETED);
		})
		.catch((err) =>
			logger("receiver", import.meta.url, "Download error:", err),
		);

	// Request first file to start the transfer
	requestNextFile();
}

// Receiver: Signal sender that we're ready to receive the next file
function requestNextFile() {
	const {
		currentFileIndex,
		filesMetadata,
		actions: { setCurrentFileIndex },
	} = useReceiverStore.getState();
	const { dataChannel } = useRTCStore.getState();
	if (currentFileIndex >= filesMetadata.length) return;

	logger(
		"receiver",
		import.meta.url,
		`Starting to receive file: ${filesMetadata[currentFileIndex].name}`,
	);

	const readyMessage: MessageOfType<MessageType.READY_TO_RECEIVE> = {
		type: MessageType.READY_TO_RECEIVE,
		payload: {
			fileName: filesMetadata[currentFileIndex].name,
			offset: 0,
		},
	};

	dataChannel?.send(packMessage(readyMessage));

	setCurrentFileIndex(currentFileIndex + 1);
}

let totalReceivedChunks = 0;

// Receiver: Process incoming chunk (no ACK needed - using bufferedAmount backpressure)
function handleReceivedChunk(
	chunkPayload: MessageOfType<MessageType.CHUNK>["payload"],
) {
	const { fileStreamsByName, filesMetadata } = useReceiverStore.getState();

	const { setBytesDownloaded, setStatus, setFileProgress } =
		useReceiverStore.getState().actions;

	const targetFileStream = fileStreamsByName[chunkPayload.fileName];

	if (!targetFileStream) {
		logger(
			"receiver",
			import.meta.url,
			"No stream found for",
			chunkPayload.fileName,
		);
		return;
	}

	const chunkData = chunkPayload.bytes;
	const chunkSize = chunkData.byteLength;
	const chunkEndOffset = chunkPayload.offset + chunkSize;

	// logger(
	// 	"receiver",
	// 	import.meta.url,
	// 	`Received chunk for file: ${chunkPayload.fileName} (${chunkPayload.offset} - ${chunkEndOffset}, final: ${chunkPayload.final})`,
	// );
	totalReceivedChunks = totalReceivedChunks + chunkSize;
	setBytesDownloaded(chunkSize);

	// Update per-file progress
	const fileMetadata = filesMetadata.find(
		(f) => f.name === chunkPayload.fileName,
	);
	if (fileMetadata) {
		const fileProgress = chunkEndOffset / fileMetadata.size;
		setFileProgress(chunkPayload.fileName, fileProgress);
	}

	// Push chunk data into the download stream
	if (!targetFileStream.isClosed) {
		targetFileStream.enqueue(chunkData);
	} else {
		logger(
			"receiver",
			import.meta.url,
			`Stream already closed for ${chunkPayload.fileName}, skipping enqueue`,
		);
	}

	// If this was the final chunk, close stream and request next file
	if (chunkPayload.final) {
		logger(
			"receiver",
			import.meta.url,
			`Final chunk received for file: ${chunkPayload.fileName}. Closing stream.`,
		);
		if (!targetFileStream.isClosed) {
			targetFileStream.close();
		}
		requestNextFile();
		const { currentFileIndex: idxAfter, filesMetadata: metaAfter } =
			useReceiverStore.getState();
		if (idxAfter >= metaAfter.length) {
			setStatus(ReceiverStatus.RECEIVING_FILE); // will become completed when DONE message arrives
		}
	}
}
