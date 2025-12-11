import Link from "next/link";
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

function NotFound() {
	return (
		<Empty className="min-h-screen">
			<EmptyHeader>
				<EmptyMedia variant="icon">
					<MoodPuzzled />
				</EmptyMedia>
				<EmptyTitle>404 - Not Found</EmptyTitle>
				<EmptyDescription>
					The page you&apos;re looking for doesn&apos;t exist.
				</EmptyDescription>
			</EmptyHeader>
			<EmptyContent>
				<Button asChild size="sm" className="text-sm">
					<Link href="/" replace>
						Go to Home
					</Link>
				</Button>
			</EmptyContent>
		</Empty>
	);
}

export default NotFound;
