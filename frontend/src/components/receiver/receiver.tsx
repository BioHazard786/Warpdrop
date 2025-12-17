/** biome-ignore-all lint/suspicious/noArrayIndexKey: BS */
"use client";

import { notFound } from "next/navigation";
import { use, useEffect, useState } from "react";
import Hero from "@/components/hero";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { IsSenderContext } from "@/context/is-sender-context";
import { useCloseConfirmation } from "@/hooks/use-close-conformation";
import { useWebRTC } from "@/hooks/use-webrtc";
import { logger } from "@/lib/logger";
import { initializeFileDownload } from "@/lib/webrtc";
import useReceiverStore, {
	ReceiverStatus,
	useReceiverActions,
} from "@/store/use-receiver-store";
import { useRoleActions } from "@/store/use-role-store";
import FileTable from "./file-table";
import StatsTable from "./stats-table";
import { formatBytes } from "@/lib/utils";

export default function Receiver({ roomId }: { roomId: string }) {
	const [shouldConnect, setShouldConnect] = useState(true);
	const [hasStarted, setHasStarted] = useState(false);
	const { setIsSender } = useRoleActions();
	const filesMetadata = useReceiverStore.use.filesMetadata();
	const bytesDownloaded = useReceiverStore.use.bytesDownloaded();
	const currentFileIndex = useReceiverStore.use.currentFileIndex();
	const transferSpeed = useReceiverStore.use.transferSpeed();
	const estimatedTimeRemaining = useReceiverStore.use.estimatedTimeRemaining();
	const { setRoomId } = useReceiverActions();
	const status = useReceiverStore.use.status();
	const error = useReceiverStore.use.error();

	const totalBytes = filesMetadata.reduce((sum, file) => sum + file.size, 0);
	const overallProgress =
		totalBytes > 0 ? (bytesDownloaded / totalBytes) * 100 : 0;

	setIsSender(use(IsSenderContext));

	useWebRTC({ shouldConnect, roomId });

	useCloseConfirmation({ status });

	const handleReceive = () => {
		logger("receiver", import.meta.url, "Starting file download process");
		setHasStarted(true);
		initializeFileDownload();
	};

	useEffect(() => {
		setRoomId(roomId);
	}, [roomId, setRoomId]);

	useEffect(() => {
		if (status === ReceiverStatus.COMPLETED) {
			setShouldConnect(false);
			logger(
				"receiver",
				import.meta.url,
				"Transfer complete, connection closed.",
			);
		}
	}, [status]);

	if (error && error === "Room not found") {
		return notFound();
	}

	return (
		<main className="flex flex-col items-center justify-center min-h-screen p-8 space-y-8 max-w-xl min-w-sm md:min-w-md lg:min-w-lg mx-auto">
			<Hero />
			{/* Status and error messages */}
			{error && (
				<div className="text-destructive text-sm font-medium w-full">
					Error: {error}
				</div>
			)}
			{!error &&
				(status === ReceiverStatus.CONNECTING ||
					status === ReceiverStatus.WS_CONNECTING) && (
					<div className="text-muted-foreground text-sm text-center w-full">
						{status === ReceiverStatus.WS_CONNECTING &&
							"Establishing WebSocket connection..."}
						{status === ReceiverStatus.CONNECTING &&
							"Establishing WebRTC connection..."}
					</div>
				)}

			<FileTable />

			{hasStarted && status === ReceiverStatus.COMPLETED && (
				<StatsTable />
			)}
			
			{hasStarted && (
				<div className="w-full space-y-2">
					<div className="flex justify-between text-sm">
						<span className="text-muted-foreground">
							{status === ReceiverStatus.COMPLETED
								? "Download completed"
								: "Downloading..."}
						</span>
						<span className="font-medium">
							{transferSpeed} • ETA {estimatedTimeRemaining}
						</span>
					</div>
					<Progress value={overallProgress} className="h-5 rounded-sm" />
					<div className="flex justify-between text-xs text-muted-foreground">
						<span>
							{formatBytes(bytesDownloaded)} /{" "}
							{formatBytes(totalBytes)}
						</span>
						<span>
							{currentFileIndex} / {filesMetadata.length} files
						</span>
					</div>
				</div>
			)}
			{!hasStarted && status === ReceiverStatus.READY && (
				<Button className="w-full" onClick={handleReceive}>
					Download
				</Button>
			)}
		</main>
	);
}
