import { useEffect } from "react";
import { ReceiverStatus } from "@/store/use-receiver-store";
import { SenderStatus } from "@/store/use-sender-store";

export function useCloseConfirmation({
	status,
}: {
	status: SenderStatus | ReceiverStatus;
}) {
	const senderBusyStatuses: SenderStatus[] = [
		SenderStatus.CONNECTING,
		SenderStatus.WS_CONNECTING,
		SenderStatus.READY,
		SenderStatus.SENDING,
	];

	const receiverBusyStatuses: ReceiverStatus[] = [
		ReceiverStatus.WS_CONNECTING,
		ReceiverStatus.CONNECTING,
		ReceiverStatus.READY,
		ReceiverStatus.RECEIVING_FILE,
	];

	const shouldWarnOnClose =
		senderBusyStatuses.includes(status as SenderStatus) ||
		receiverBusyStatuses.includes(status as ReceiverStatus);

	useEffect(() => {
		if (!shouldWarnOnClose) {
			return;
		}

		const handleBeforeUnload = (e: BeforeUnloadEvent) => {
			e.preventDefault();
			// For legacy support
			e.returnValue = "";
		};

		window.addEventListener("beforeunload", handleBeforeUnload);

		return () => {
			window.removeEventListener("beforeunload", handleBeforeUnload);
		};
	}, [shouldWarnOnClose]);
}
