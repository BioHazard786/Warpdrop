/** biome-ignore-all lint/suspicious/noArrayIndexKey: BS */
/** biome-ignore-all lint/suspicious/noShadowRestrictedNames: BS */
"use client";

import { CircleStop, Infinity } from "lucide-react";
import { use, useState } from "react";
import Hero from "@/components/hero";
import Dropzone from "@/components/sender/dropzone";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Separator } from "@/components/ui/separator";
import { IsSenderContext } from "@/context/is-sender-context";
import { useWebRTC } from "@/hooks/use-webrtc";
import { createSharableLink } from "@/lib/utils";
import useFileUploadStore, {
	useFileUploadActions,
} from "@/store/use-file-upload-store";
import { useRoleActions } from "@/store/use-role-store";
import useSenderStore, {
	SenderStatus,
	useSenderActions,
} from "@/store/use-sender-store";
import CopyLink from "./copy-to-clipboard";
import SenderFileTable from "./file-table";
import QRCode from "./qr-code";

export default function Sender() {
	const [shouldConnect, setShouldConnect] = useState(false);
	const [hasStarted, setHasStarted] = useState(false);
	const files = useFileUploadStore.use.files();
	const clearFiles = useFileUploadActions().clearFiles;
	const connectedDevices = useSenderStore.use.connectedDevices();
	const currentFileName = useSenderStore.use.currentFileName();
	const currentFileProgress = useSenderStore.use.currentFileProgress();
	const completedFileCount = useSenderStore.use.completedFileCount();
	const roomId = useSenderStore.use.roomId();
	const { reset } = useSenderActions();
	const { setIsSender } = useRoleActions();
	const status = useSenderStore.use.status();
	const error = useSenderStore.use.error();

	setIsSender(use(IsSenderContext));

	useWebRTC({ shouldConnect });

	const handleStart = () => {
		setHasStarted(true);
		setShouldConnect(true);
	};

	const handleCancel = () => {
		clearFiles();
		setHasStarted(false);
		setShouldConnect(false);
		reset();
	};

	const handleStopUpload = () => {
		setHasStarted(false);
		setShouldConnect(false);
		reset();
	};

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
				(status === SenderStatus.CONNECTING ||
					status === SenderStatus.WS_CONNECTING) && (
					<div className="text-muted-foreground text-sm text-center w-full">
						{status === SenderStatus.WS_CONNECTING &&
							"Establishing WebSocket connection..."}
						{status === SenderStatus.CONNECTING &&
							"Establishing WebRTC connection..."}
					</div>
				)}

			{/* Show dropzone and start button before starting */}
			{!hasStarted && (
				<>
					<Dropzone />
					{files.length > 0 && (
						<div className="flex gap-3 w-full">
							<Button
								variant="outline"
								onClick={handleCancel}
								className="w-full"
							>
								Cancel
							</Button>
							<Button onClick={handleStart} className="w-full">
								Start
							</Button>
						</div>
					)}
					{files.length === 0 && (
						<div className="flex h-10 justify-evenly items-center w-full opacity-70 mt-4">
							<div className="text-center">
								<div className="text-xl font-bold ">End-to-End</div>
								<div className="text-xs text-muted-foreground uppercase tracking-wider">
									Encryption
								</div>
							</div>
							<Separator orientation="vertical" />
							<div className="text-center">
								<div className="text-xl font-bold">P2P</div>
								<div className="text-xs text-muted-foreground uppercase tracking-wider">
									WebRTC
								</div>
							</div>

							<Separator orientation="vertical" />
							<div className="text-center">
								<div className="flex items-center justify-center ">
									<Infinity />
								</div>
								<div className="text-xs text-muted-foreground uppercase tracking-wider">
									File Size
								</div>
							</div>
						</div>
					)}
				</>
			)}

			{/* Show file table, QR code, and connection info after starting */}
			{hasStarted && (
				<>
					<SenderFileTable />

					<div className="flex gap-3 items-center w-full">
						<QRCode
							roomId={roomId || "example-room-id"}
							width={180}
							height={180}
						/>
						<div className="flex justify-between items-center flex-col w-full h-[140px]">
							<CopyLink
								label="Copy URL"
								value={createSharableLink(roomId || "example-room-id")}
							/>
							<CopyLink
								label="Copy RoomID"
								value={roomId || "example-room-id"}
							/>
						</div>
					</div>

					<Separator />

					<div className="w-full space-y-6">
						<div className="flex justify-between w-full">
							<h2 className="text-lg font-semibold text-primary mb-2">
								Connected Devices:
							</h2>
							<Button variant="destructive" onClick={handleStopUpload}>
								<CircleStop />
								Stop Upload
							</Button>
						</div>
						<div>
							{connectedDevices.map((device, index) => (
								<div
									key={`connected-device-${index}`}
									className="w-full space-y-2"
								>
									<div className="flex justify-between text-sm">
										<span className="space-x-2">
											<span className="font-medium ">
												{device.browserName}{" "}
												<span className="text-muted-foreground">
													v{device.browserVersion}
												</span>
											</span>
											{renderStatusBadge(status)}
										</span>
										<span className="font-medium truncate ml-2 max-w-64">
											{currentFileName}
										</span>
									</div>
									<Progress
										value={currentFileProgress * 100}
										className="h-2 [&>div]:rounded-r-full"
									/>
									<div className="flex justify-between text-xs text-muted-foreground">
										<span>{Math.round(currentFileProgress * 100)}%</span>
										<span>
											{completedFileCount} / {files.length} files
										</span>
									</div>
								</div>
							))}
						</div>
					</div>
				</>
			)}
		</main>
	);
}

function renderStatusBadge(status: SenderStatus) {
	switch (status) {
		case SenderStatus.READY:
			return <Badge>READY</Badge>;
		case SenderStatus.SENDING:
			return <Badge variant="sending">SENDING</Badge>;
		case SenderStatus.COMPLETED:
			return <Badge variant="completed">COMPLETED</Badge>;
		case SenderStatus.ERROR:
			return <Badge variant="destructive">ERROR</Badge>;
		default:
			return null;
	}
}
