import { browserName, browserVersion } from "react-device-detect";
import type { SendJsonMessage } from "react-use-websocket/dist/lib/types";
import {
	streamDownloadMultipleFiles,
	streamDownloadSingleFile,
} from "@/lib/download-utils";
import { logger } from "@/lib/logger";
import { type MessageOfType, MessageType, parseMessage } from "@/lib/messages";
import { calculateTransferStats } from "@/lib/transfer-stats-utils";
import {
	CHUNK_SIZE,
	getZipFilename,
	HIGH_WATER_MARK,
	LOW_WATER_MARK,
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
			const { status } = useSenderStore.getState();
			if (status !== SenderStatus.COMPLETED) {
				// If not completed, this could be receiver disconnecting early
				logger(
					"receiver",
					import.meta.url,
					"Data channel closed during transfer, status:",
					status,
				);
				resetWebRTC();
			}
			senderActions.setStatus(SenderStatus.IDLE);
			senderActions.removeConnectedDevice();
		} else {
			// On the receiver side, check if we were in a transfer state
			// If so, this is a normal closure after transfer
			const { status } = useReceiverStore.getState();
			if (status !== ReceiverStatus.COMPLETED) {
				// If not completed, this could be sender disconnecting early
				logger(
					"receiver",
					import.meta.url,
					"Data channel closed during transfer, status:",
					status,
				);
				resetWebRTC();
			}
		}
	};

	dataChannel.onerror = (err) => {
		logger(null, import.meta.url, "Data channel error:", err);

		const { isSender } = useRoleStore.getState();

		if (isSender) {
			const errMsg = err.error
				? `${err.error.name} - ${err.error.message}`
				: "Data channel error";

			const { status } = useSenderStore.getState();
			if (
				status === SenderStatus.COMPLETED ||
				status === SenderStatus.SENDING ||
				status === SenderStatus.IDLE
			) {
				// Transfer was in progress or completed - treat as normal close
				logger(
					"sender",
					import.meta.url,
					"Data channel error during/after transfer, treating as close",
				);
				resetWebRTC();
				return;
			}
			senderActions.setError(errMsg);
		} else {
			// On the receiver side, onerror can fire instead of onclose when the sender
			// closes the data channel (browser-specific behavior). Check if transfer
			// was completed or if this is a genuine error.
			const { status } = useReceiverStore.getState();
			if (
				status === ReceiverStatus.COMPLETED ||
				status === ReceiverStatus.RECEIVING_FILE
			) {
				// Transfer was in progress or completed - treat as normal close
				logger(
					"receiver",
					import.meta.url,
					"Data channel error during/after transfer, treating as close",
				);
				resetWebRTC();
				return;
			}
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
					senderActions.setStatus(SenderStatus.COMPLETED);
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
				deviceName: browserName,
				deviceVersion: browserVersion,
			},
		}),
	);
}

// Sender: Stream file chunks with deduplicated logic and backpressure
async function streamFileChunks(fileName: string, startOffset: number) {
	const { dataChannel } = useRTCStore.getState();
	const { files } = useFileUploadStore.getState();
	const senderActions = useSenderStore.getState().actions;

	// Validate file existence
	const file = validateOffset(
		files.map(({ file }) => file),
		fileName,
		startOffset,
	);

	if (!dataChannel || dataChannel.readyState !== "open") {
		logger("sender", import.meta.url, "DataChannel not open; aborting");
		return;
	}

	dataChannel.bufferedAmountLowThreshold = LOW_WATER_MARK;
	const reader = file.stream().getReader();

	let buffer = new Uint8Array(0);
	let currentOffset = startOffset;
	let lastProgressTime = Date.now();
	let lastProgressOffset = currentOffset;

	// -- Helper: Wait for WebRTC buffer to clear --
	const waitForBufferDrain = (): Promise<void> => {
		if (dataChannel.bufferedAmount <= LOW_WATER_MARK) return Promise.resolve();
		return new Promise((resolve) => {
			dataChannel.onbufferedamountlow = () => {
				dataChannel.onbufferedamountlow = null;
				resolve();
			};
		});
	};

	// -- Helper: Package, Send, and Update Stats --
	const sendData = async (chunk: Uint8Array<ArrayBuffer>, isFinal: boolean) => {
		// 1. Backpressure Check
		if (dataChannel.bufferedAmount > HIGH_WATER_MARK) {
			await waitForBufferDrain();
		}

		if (dataChannel.readyState !== "open") {
			logger("sender", import.meta.url, "DataChannel closed during transfer");
			return;
		}

		// 2. Prepare Message
		const message: MessageOfType<MessageType.CHUNK> = {
			type: MessageType.CHUNK,
			payload: {
				fileName,
				offset: currentOffset,
				bytes: chunk,
				final: isFinal,
			},
		};

		// 3. Send & Update State
		dataChannel.send(packMessage(message));

		const nextOffset = currentOffset + chunk.length;
		senderActions.setCurrentFileOffset(nextOffset);
		senderActions.setCurrentFileProgress(nextOffset / file.size);
		senderActions.setStatus(SenderStatus.SENDING);

		// 4. Update Stats (Throttled to 1s)
		const now = Date.now();
		const timeDiff = (now - lastProgressTime) / 1000;
		if (timeDiff >= 1 || isFinal) {
			const stats = calculateTransferStats(
				nextOffset,
				file.size,
				lastProgressOffset,
				timeDiff,
			);
			senderActions.setTransferSpeed(stats.speed);
			senderActions.setEstimatedTimeRemaining(stats.eta);
			lastProgressTime = now;
			lastProgressOffset = nextOffset;
		}

		currentOffset = nextOffset;
	};

	try {
		while (true) {
			const { value, done } = await reader.read();
			if (done) break;

			// Merge new data into buffer
			const merged = new Uint8Array(buffer.length + value.length);
			merged.set(buffer);
			merged.set(value, buffer.length);
			buffer = merged;

			// Process full chunks
			while (buffer.length >= CHUNK_SIZE) {
				const chunk = buffer.subarray(0, CHUNK_SIZE);
				buffer = buffer.subarray(CHUNK_SIZE);

				// Check if this specific chunk finishes the file
				const isFinal = currentOffset + chunk.length >= file.size;
				await sendData(chunk, isFinal);

				if (isFinal) break; // Stop if we hit end of file size
			}
		}

		// Flush remaining bytes
		if (buffer.length > 0) {
			await sendData(buffer, true);
		}

		logger("sender", import.meta.url, `Sent file: ${fileName}`);

		// Completion updates
		const { completedFileCount } = useSenderStore.getState();
		senderActions.setCompletedFileCount(completedFileCount + 1);
		senderActions.setCurrentFileProgress(0); // Reset for next file

		if (completedFileCount + 1 >= files.length) {
			logger(
				"sender",
				import.meta.url,
				"All files sent, awaiting confirmation",
			);
		}
	} catch (err) {
		logger("sender", import.meta.url, "Transfer failed", err);
		if (dataChannel.readyState === "open") {
			senderActions.setError(err instanceof Error ? err.message : String(err));
		}
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

	receiverActions.setDownloadStarted(Date.now());

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
let lastReceiverProgressTime = Date.now();
let lastReceiverProgressBytes = 0;

// Receiver: Process incoming chunk (no ACK needed - using bufferedAmount backpressure)
function handleReceivedChunk(
	chunkPayload: MessageOfType<MessageType.CHUNK>["payload"],
) {
	const { fileStreamsByName, filesMetadata } = useReceiverStore.getState();

	const receiverActions = useReceiverStore.getState().actions;

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
	receiverActions.setBytesDownloaded(chunkSize);

	// Calculate and update transfer stats
	const now = Date.now();
	const timeDiffSeconds = (now - lastReceiverProgressTime) / 1000;
	if (timeDiffSeconds >= 1) {
		// Update stats every 1 seconds
		const totalSize = filesMetadata.reduce((sum, f) => sum + f.size, 0);
		const { speed, eta } = calculateTransferStats(
			totalReceivedChunks,
			totalSize,
			lastReceiverProgressBytes,
			timeDiffSeconds,
		);
		receiverActions.setTransferSpeed(speed);
		receiverActions.setEstimatedTimeRemaining(eta);
		lastReceiverProgressTime = now;
		lastReceiverProgressBytes = totalReceivedChunks;
	}

	// Update per-file progress
	const fileMetadata = filesMetadata.find(
		(f) => f.name === chunkPayload.fileName,
	);
	if (fileMetadata) {
		const fileProgress = chunkEndOffset / fileMetadata.size;
		receiverActions.setFileProgress(chunkPayload.fileName, fileProgress);
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
			receiverActions.setStatus(ReceiverStatus.RECEIVING_FILE); // will become completed when DONE message arrives
		}
	}
}
