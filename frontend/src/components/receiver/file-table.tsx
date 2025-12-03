/** biome-ignore-all lint/suspicious/noArrayIndexKey: BS */

import { CircleCheckBig } from "lucide-react";
import { getFileIcon } from "@/components/file-table-helper";
import { CircularProgress } from "@/components/ui/circular-progress";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { formatBytes } from "@/lib/utils";
import useReceiverStore from "@/store/use-receiver-store";

const FileProgress = ({ fileName }: { fileName: string }) => {
	const fileProgressByName = useReceiverStore.use.fileProgressByName();
	const progress = fileProgressByName[fileName] || 0;

	if (progress >= 1) {
		return (
			<div className="p-2.5">
				<CircleCheckBig className="text-emerald-500 size-5" />
			</div>
		);
	}

	return (
		<CircularProgress
			value={progress * 100}
			size={40}
			strokeWidth={4}
			className="inline-flex"
		/>
	);
};

const FileTable = () => {
	const files = useReceiverStore.use.filesMetadata();

	if (files.length === 0) {
		return null;
	}

	return (
		<div className="overflow-hidden rounded-md border bg-background w-full">
			<Table>
				<TableHeader className="text-xs">
					<TableRow className="bg-muted/50">
						<TableHead className="h-9 py-2">Name</TableHead>
						<TableHead className="h-9 py-2">Type</TableHead>
						<TableHead className="h-9 py-2">Size</TableHead>
						<TableHead className="h-9 py-2 text-right">Progress</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody className="text-[13px]">
					{files.map((file, index) => (
						<TableRow key={`file-row-${index}`}>
							<TableCell className="max-w-48 py-2 font-medium">
								<span className="flex items-center gap-2">
									<span className="shrink-0">{getFileIcon(file)}</span>{" "}
									<span className="truncate">{file.name}</span>
								</span>
							</TableCell>
							<TableCell className="py-2 text-muted-foreground">
								{file.type.split("/")[1]?.toUpperCase() || "UNKNOWN"}
							</TableCell>
							<TableCell className="py-2 text-muted-foreground">
								{formatBytes(file.size)}
							</TableCell>
							<TableCell className="py-2 text-right flex justify-end whitespace-nowrap text-muted-foreground">
								<FileProgress fileName={file.name} />
							</TableCell>
						</TableRow>
					))}
				</TableBody>
			</Table>
		</div>
	);
};

export default FileTable;
