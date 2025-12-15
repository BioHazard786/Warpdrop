import { use, useEffect, useRef } from "react";
import useWebSocket, { ReadyState } from "react-use-websocket";
import { IsSenderContext } from "@/context/is-sender-context";
import { logger } from "@/lib/logger";
import { type MessageOfType, MessageType, parseMessage } from "@/lib/messages";
import { createPeerConnection } from "@/lib/webrtc";
import { ReceiverStatus, useReceiverActions } from "@/store/use-receiver-store";
import { useRTCActions } from "@/store/use-rtc-store";
import { SenderStatus, useSenderActions } from "@/store/use-sender-store";

const WS_URL =
	typeof window !== "undefined"
		? (process.env.NEXT_PUBLIC_WS_URL ??
			`${window.location.protocol === "https:" ? "wss" : "ws"}://${window.location.host}/ws`)
		: null;

// useWebRTC: sets up WebSocket signaling + WebRTC peer connection lifecycle.
// Responsibilities:
// 1. Create/join rooms via signaling server.
// 2. Exchange SDP & ICE messages ("signal" type).
// 3. Manage RTCPeerConnection + cleanup via rtc store reset.
export function useWebRTC({
	shouldConnect = true,
	roomId,
}: {
	shouldConnect?: boolean;
	roomId?: string;
}) {
	const isSender = use(IsSenderContext);
	const peerConnectionRef = useRef<RTCPeerConnection | null>(null);
	const iceCandidateQueueRef = useRef<RTCIceCandidateInit[]>([]);
	const remoteDescriptionSetRef = useRef(false);
	const { resetWebRTC } = useRTCActions();
	const senderActions = useSenderActions();
	const receiverActions = useReceiverActions();

	const { sendJsonMessage, lastMessage, readyState } = useWebSocket(
		WS_URL,
		{
			onOpen: () => {
				logger(null, import.meta.url, "WebSocket Connected");

				if (isSender) {
					logger("sender", import.meta.url, "Sending create_room message");

					const message: MessageOfType<MessageType.CREATE_ROOM> = {
						type: MessageType.CREATE_ROOM,
						client_type: "web",
					};

					sendJsonMessage(message);
				} else if (!isSender && roomId) {
					logger("receiver", import.meta.url, "Sending join_room message");

					const message: MessageOfType<MessageType.JOIN_ROOM> = {
						type: MessageType.JOIN_ROOM,
						room_id: roomId,
						client_type: "web",
					};

					sendJsonMessage(message);
				}
			},
			onClose: (event) => {
				logger(null, import.meta.url, "WebSocket Disconnected", event);
			},
			onError: (event) => {
				logger(null, import.meta.url, "WebSocket Error", event);
				if (isSender) senderActions.setError("WebSocket connection error");
				else receiverActions.setError("WebSocket connection error");
			},
			shouldReconnect: () => true,
		},
		shouldConnect,
	);

	useEffect(() => {
		if (readyState === ReadyState.CONNECTING) {
			if (isSender) senderActions.setStatus(SenderStatus.WS_CONNECTING);
			else receiverActions.setStatus(ReceiverStatus.WS_CONNECTING);
		}
		if (readyState === ReadyState.OPEN) {
			if (isSender) senderActions.setStatus(SenderStatus.IDLE);
			else receiverActions.setStatus(ReceiverStatus.IDLE);
		}
	}, [
		readyState,
		isSender,
		receiverActions.setStatus,
		senderActions.setStatus,
	]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: handleSignal is defined below
	useEffect(() => {
		if (!lastMessage?.data) return;

		const message = parseMessage(JSON.parse(lastMessage.data));

		switch (message.type) {
			case MessageType.ROOM_CREATED:
				senderActions.setRoomId(message.room_id);
				logger(
					"sender",
					import.meta.url,
					"Room created with ID:",
					message.room_id,
				);
				break;

			case MessageType.JOIN_SUCCESS:
				logger(
					"receiver",
					import.meta.url,
					"Room joined with ID:",
					message.room_id,
				);
				break;

			case MessageType.PEER_JOINED: {
				logger("sender", import.meta.url, "Peer joined. Creating offer...");
				// Create new connection if none exists or if existing one is closed
				if (
					!peerConnectionRef.current ||
					peerConnectionRef.current.signalingState === "closed"
				) {
					peerConnectionRef.current = createPeerConnection(sendJsonMessage);
				}
				const pc = peerConnectionRef.current;

				pc.createOffer()
					.then((offer) => pc.setLocalDescription(offer))
					.then(() => {
						sendJsonMessage({
							type: "signal",
							payload: pc.localDescription,
						});
					})
					.catch((err) => {
						logger("sender", import.meta.url, "Error creating offer:", err);
						senderActions.setError(
							err instanceof Error ? err.message : "Failed to create offer",
						);
					});
				break;
			}

			case MessageType.PEER_LEFT:
				logger(null, import.meta.url, "Peer left the room.");
				break;

			case MessageType.SIGNAL:
				handleSignal(message.payload);
				break;

			case MessageType.ERROR:
				if (isSender)
					senderActions.setError(message.payload?.error ?? "Unknown error");
				else
					receiverActions.setError(message.payload?.error ?? "Unknown error");
				logger(null, import.meta.url, "Server error:", message.payload?.error);
				break;
		}
	}, [lastMessage, sendJsonMessage]);

	// Cleanup on unmount
	useEffect(() => {
		return () => {
			logger(null, import.meta.url, "Cleaning up on unmount");
			resetWebRTC();
			if (peerConnectionRef.current) {
				peerConnectionRef.current = null;
			}
		};
	}, [resetWebRTC]);

	const handleSignal = async (
		payload: MessageOfType<MessageType.SIGNAL>["payload"],
	) => {
		// Create new connection if none exists or if existing one is closed
		if (
			!peerConnectionRef.current ||
			peerConnectionRef.current.signalingState === "closed"
		) {
			logger(
				"receiver",
				import.meta.url,
				"Received signal. Creating peer connection...",
			);
			peerConnectionRef.current = createPeerConnection(sendJsonMessage);
			remoteDescriptionSetRef.current = false;
			iceCandidateQueueRef.current = [];
		}

		const pc = peerConnectionRef.current;

		if (payload.type && payload.sdp) {
			// Check if this is an SDP message (offer/answer)
			try {
				await pc.setRemoteDescription(new RTCSessionDescription(payload));
				remoteDescriptionSetRef.current = true;

				// Process any queued ICE candidates now that remote description is set
				while (iceCandidateQueueRef.current.length > 0) {
					const queuedCandidate = iceCandidateQueueRef.current.shift();
					if (queuedCandidate) {
						try {
							await pc.addIceCandidate(new RTCIceCandidate(queuedCandidate));
						} catch (err) {
							logger(
								null,
								import.meta.url,
								"Error adding queued ICE candidate:",
								err,
							);
						}
					}
				}

				// If we received an offer, create and send answer
				if (payload.type === "offer") {
					const answer = await pc.createAnswer();
					await pc.setLocalDescription(answer);
					sendJsonMessage({
						type: "signal",
						payload: pc.localDescription,
					});
				}

				// If we received an answer, send the file metadata (only for sender)
				else if (payload.type === "answer") {
					logger("sender", import.meta.url, "Send file metadata to receiver");
				}
			} catch (err) {
				logger(null, import.meta.url, "Error handling SDP:", err);
				const errMsg =
					err instanceof Error ? err.message : "Failed to handle SDP";
				if (isSender) senderActions.setError(errMsg);
				else receiverActions.setError(errMsg);
			}
		} else if (payload.ice_candidate) {
			// It's an ICE candidate
			try {
				// If remote description is not set yet, queue the candidate
				if (!remoteDescriptionSetRef.current) {
					logger(
						null,
						import.meta.url,
						"Remote description not set yet. Queueing ICE candidate.",
					);
					iceCandidateQueueRef.current.push(payload.ice_candidate);
					return;
				}

				await pc.addIceCandidate(new RTCIceCandidate(payload.ice_candidate));
			} catch (err) {
				// Ignore errors for ICE candidates - they're not critical
				logger(
					null,
					import.meta.url,
					"Warning: Error adding ICE candidate (non-critical):",
					err,
				);
				// Don't set error state for ICE candidate failures as the transfer still works
			}
		}
	};
}
