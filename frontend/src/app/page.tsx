"use client";

import Sender from "@/components/sender/sender";
import { IsSenderContext } from "@/context/is-sender-context";

export default function Home() {
	return (
		<IsSenderContext value={true}>
			<Sender />
		</IsSenderContext>
	);
}
