import useReceiverStore from "@/store/use-receiver-store";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { formatBytes } from "@/lib/utils";
import { formatETA, formatSpeed } from "@/lib/transfer-stats-utils";

const StatsTable = () => {
	const downloadStarted = useReceiverStore.getState().downloadStarted;
    const files = useReceiverStore.getState().filesMetadata;
    const totalTimeTaken = (Date.now() - downloadStarted) / 1000;
    const totalSize = files.reduce((acc, file) => acc + file.size, 0);

	return (
		<div className="overflow-hidden rounded-md border bg-background w-full">
			<Table>
				<TableHeader className="text-xs">
					<TableRow className="bg-muted/50">
						<TableHead className="h-9 py-2">Total Size</TableHead>
						<TableHead className="h-9 py-2 text-center">Duration</TableHead>
						<TableHead className="h-9 py-2 text-right">AVG Speed</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody className="text-[13px]">
					<TableRow>
                        <TableCell className="py-2 text-muted-foreground">
                            {formatBytes(totalSize)}
                        </TableCell>
                        <TableCell className="py-2 text-muted-foreground text-center">
                            {formatETA(totalTimeTaken)}
                        </TableCell>
                        <TableCell className="py-2 text-right flex justify-end whitespace-nowrap text-muted-foreground">
                            {formatSpeed(totalSize / totalTimeTaken)}
                        </TableCell>
                    </TableRow>
				</TableBody>
			</Table>
		</div>
	);
};

export default StatsTable;