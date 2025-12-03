import {
	FileArchiveIcon,
	FileIcon,
	FileSpreadsheetIcon,
	FileTextIcon,
	HeadphonesIcon,
	ImageIcon,
	VideoIcon,
} from "lucide-react";
import type { FileMetadata } from "@/store/use-receiver-store";

export const getFileIcon = (file: FileMetadata | File) => {
	const fileType = file.type;
	const fileName = file.name;

	if (
		fileType.includes("pdf") ||
		fileName.endsWith(".pdf") ||
		fileType.includes("word") ||
		fileName.endsWith(".doc") ||
		fileName.endsWith(".docx")
	) {
		return <FileTextIcon className="size-4 opacity-60" />;
	} else if (
		fileType.includes("zip") ||
		fileType.includes("archive") ||
		fileName.endsWith(".zip") ||
		fileName.endsWith(".rar")
	) {
		return <FileArchiveIcon className="size-4 opacity-60" />;
	} else if (
		fileType.includes("excel") ||
		fileName.endsWith(".xls") ||
		fileName.endsWith(".xlsx")
	) {
		return <FileSpreadsheetIcon className="size-4 opacity-60" />;
	} else if (fileType.includes("video/")) {
		return <VideoIcon className="size-4 opacity-60" />;
	} else if (fileType.includes("audio/")) {
		return <HeadphonesIcon className="size-4 opacity-60" />;
	} else if (fileType.startsWith("image/")) {
		return <ImageIcon className="size-4 opacity-60" />;
	}
	return <FileIcon className="size-4 opacity-60" />;
};
