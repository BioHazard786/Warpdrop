"use client";

import { useParams } from "next/navigation";
import Receiver from "@/components/receiver/receiver";
import { IsSenderContext } from "@/context/is-sender-context";

export default function RoomPage() {
	const { roomId } = useParams<{ roomId: string }>();
	return (
		<IsSenderContext value={false}>
			<Receiver roomId={roomId} />
		</IsSenderContext>
	);
}
