"use client";

import { ArrowUpRightIcon, SearchIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef } from "react";
import MoodPuzzled from "@/components/mood-puzzled";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyContent,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupInput,
} from "@/components/ui/input-group";
import { Kbd } from "@/components/ui/kbd";

function NotFound() {
	const inputRef = useRef<HTMLInputElement | null>(null);
	const router = useRouter();

	return (
		<Empty className="min-h-screen">
			<EmptyHeader>
				<EmptyMedia variant="icon">
					<MoodPuzzled />
				</EmptyMedia>
				<EmptyTitle>404 - Not Found</EmptyTitle>
				<EmptyDescription>
					The room you&apos;re trying to join doesn&apos;t exist. Enter a valid
					room ID.
				</EmptyDescription>
			</EmptyHeader>
			<EmptyContent>
				<InputGroup className="sm:w-3/4">
					<InputGroupInput
						placeholder="Enter Room ID..."
						className="text-sm"
						ref={inputRef}
						onKeyDown={(e) => {
							if (e.key === "Enter" && inputRef.current) {
								const enteredRoomId = inputRef.current.value.trim();
								router.replace(`/r/${enteredRoomId}`);
							}
						}}
					/>
					<InputGroupAddon>
						<SearchIcon />
					</InputGroupAddon>
					<InputGroupAddon align="inline-end">
						<Kbd>/</Kbd>
					</InputGroupAddon>
				</InputGroup>
			</EmptyContent>
			<Button
				variant="link"
				asChild
				className="text-muted-foreground text-sm"
				size="sm"
			>
				<Link href="/" replace>
					Click here to send files <ArrowUpRightIcon />
				</Link>
			</Button>
		</Empty>
	);
}

export default NotFound;
