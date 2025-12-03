/** biome-ignore-all lint/suspicious/noArrayIndexKey: BS */

import { getFileIcon } from "@/components/file-table-helper";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { formatBytes } from "@/lib/utils";
import useFileUploadStore from "@/store/use-file-upload-store";

const SenderFileTable = () => {
	const files = useFileUploadStore.use.files();

	if (files.length === 0) return null;

	return (
		<div className="overflow-hidden rounded-md border bg-background w-full">
			<Table>
				<TableHeader className="text-xs">
					<TableRow className="bg-muted/50">
						<TableHead className="h-9 py-2">Name</TableHead>
						<TableHead className="h-9 py-2">Type</TableHead>
						<TableHead className="h-9 py-2 text-right">Size</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody className="text-[13px]">
					{files.map(({ file }, index) => (
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
							<TableCell className="py-2 text-muted-foreground whitespace-nowrap text-right">
								{formatBytes(file.size)}
							</TableCell>
						</TableRow>
					))}
				</TableBody>
			</Table>
		</div>
	);
};

export default SenderFileTable;
