/** biome-ignore-all lint/suspicious/noExplicitAny: Cast to `any` to prevent mismatched React ref type errors across workspaces */
"use client";

import {
	type ChangeEvent,
	type DragEvent,
	type InputHTMLAttributes,
	useCallback,
	useRef,
} from "react";
import { formatBytes } from "@/lib/utils";
import useFileUploadStore, {
	type FileWithPreview,
	useFileUploadActions,
} from "@/store/use-file-upload-store";

export type FileUploadOptions = {
	maxFiles?: number; // Only used when multiple is true, defaults to Infinity
	maxSize?: number; // in bytes
	accept?: string;
	multiple?: boolean; // Defaults to false
	onFilesChange?: (files: FileWithPreview[]) => void; // Callback when files change
	onFilesAdded?: (addedFiles: FileWithPreview[]) => void; // Callback when new files are added
};

export type FileUploadState = {
	files: FileWithPreview[];
	isDragging: boolean;
	errors: string[];
};

export type FileUploadActions = {
	addFiles: (files: FileList | File[]) => void;
	removeFile: (id: string) => void;
	clearFiles: () => void;
	clearErrors: () => void;
	handleDragEnter: (e: DragEvent<HTMLElement>) => void;
	handleDragLeave: (e: DragEvent<HTMLElement>) => void;
	handleDragOver: (e: DragEvent<HTMLElement>) => void;
	handleDrop: (e: DragEvent<HTMLElement>) => void;
	handleFileChange: (e: ChangeEvent<HTMLInputElement>) => void;
	openFileDialog: () => void;
	getInputProps: (
		props?: InputHTMLAttributes<HTMLInputElement>,
	) => InputHTMLAttributes<HTMLInputElement> & {
		// Use `any` here to avoid cross-React ref type conflicts across packages
		ref: any;
	};
};

export const useFileUpload = (
	options: FileUploadOptions = {},
): FileUploadActions => {
	const {
		maxFiles = Infinity,
		maxSize = Infinity,
		accept = "*",
		multiple = false,
		onFilesChange,
		onFilesAdded,
	} = options;

	const files = useFileUploadStore.use.files();
	const {
		setFiles,
		removeFile: removeFileFromStore,
		clearFiles: clearFilesInStore,
		setIsDragging,
		setErrors,
		clearErrors: clearErrorsInStore,
	} = useFileUploadActions();

	const inputRef = useRef<HTMLInputElement>(null);

	const validateFile = useCallback(
		(file: File): string | null => {
			if (file.size > maxSize) {
				return `File "${file.name}" exceeds the maximum size of ${formatBytes(maxSize)}.`;
			}

			if (accept !== "*") {
				const acceptedTypes = accept.split(",").map((type) => type.trim());
				const fileType = file.type || "";
				const fileExtension = `.${file.name.split(".").pop()}`;

				const isAccepted = acceptedTypes.some((type) => {
					if (type.startsWith(".")) {
						return fileExtension.toLowerCase() === type.toLowerCase();
					}
					if (type.endsWith("/*")) {
						const baseType = type.split("/")[0];
						return fileType.startsWith(`${baseType}/`);
					}
					return fileType === type;
				});

				if (!isAccepted) {
					return `File "${file.name}" is not an accepted file type.`;
				}
			}

			return null;
		},
		[accept, maxSize],
	);

	const createPreview = useCallback((file: File): string | undefined => {
		return URL.createObjectURL(file);
	}, []);

	const generateUniqueId = useCallback((file: File): string => {
		return `${file.name}-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
	}, []);

	const clearFiles = useCallback(() => {
		clearFilesInStore();

		if (inputRef.current) {
			inputRef.current.value = "";
		}

		onFilesChange?.([]);
	}, [clearFilesInStore, onFilesChange]);

	const addFiles = useCallback(
		(newFiles: FileList | File[]) => {
			if (!newFiles || newFiles.length === 0) return;

			const newFilesArray = Array.from(newFiles);
			const newErrors: string[] = [];

			// Clear existing errors when new files are uploaded
			clearErrorsInStore();

			// In single file mode, clear existing files first
			if (!multiple) {
				clearFiles();
			}

			// Check if adding these files would exceed maxFiles (only in multiple mode)
			if (
				multiple &&
				maxFiles !== Infinity &&
				files.length + newFilesArray.length > maxFiles
			) {
				newErrors.push(`You can only upload a maximum of ${maxFiles} files.`);
				setErrors(newErrors);
				return;
			}

			const validFiles: FileWithPreview[] = [];

			newFilesArray.forEach((file) => {
				// Only check for duplicates if multiple files are allowed
				if (multiple) {
					const isDuplicate = files.some(
						(existingFile) =>
							existingFile.file.name === file.name &&
							existingFile.file.size === file.size,
					);

					// Skip duplicate files silently
					if (isDuplicate) {
						return;
					}
				}

				// Check file size
				if (file.size > maxSize) {
					newErrors.push(
						multiple
							? `Some files exceed the maximum size of ${formatBytes(maxSize)}.`
							: `File exceeds the maximum size of ${formatBytes(maxSize)}.`,
					);
					return;
				}

				const error = validateFile(file);
				if (error) {
					newErrors.push(error);
				} else {
					validFiles.push({
						file,
						id: generateUniqueId(file),
						preview: createPreview(file),
					});
				}
			});

			// Only update state if we have valid files to add
			if (validFiles.length > 0) {
				// Call the onFilesAdded callback with the newly added valid files
				onFilesAdded?.(validFiles);

				const updatedFiles = !multiple ? validFiles : [...files, ...validFiles];
				setFiles(updatedFiles);
				if (newErrors.length > 0) {
					setErrors(newErrors);
				}
				onFilesChange?.(updatedFiles);
			} else if (newErrors.length > 0) {
				setErrors(newErrors);
			}

			// Reset input value after handling files
			if (inputRef.current) {
				inputRef.current.value = "";
			}
		},
		[
			files,
			maxFiles,
			multiple,
			maxSize,
			validateFile,
			createPreview,
			generateUniqueId,
			clearFiles,
			clearErrorsInStore,
			setFiles,
			setErrors,
			onFilesChange,
			onFilesAdded,
		],
	);

	const removeFile = useCallback(
		(id: string) => {
			removeFileFromStore(id);
			const newFiles = files.filter((file) => file.id !== id);
			onFilesChange?.(newFiles);
		},
		[removeFileFromStore, files, onFilesChange],
	);

	const clearErrors = useCallback(() => {
		clearErrorsInStore();
	}, [clearErrorsInStore]);

	const handleDragEnter = useCallback(
		(e: DragEvent<HTMLElement>) => {
			e.preventDefault();
			e.stopPropagation();
			setIsDragging(true);
		},
		[setIsDragging],
	);

	const handleDragLeave = useCallback(
		(e: DragEvent<HTMLElement>) => {
			e.preventDefault();
			e.stopPropagation();

			if (e.currentTarget.contains(e.relatedTarget as Node)) {
				return;
			}

			setIsDragging(false);
		},
		[setIsDragging],
	);

	const handleDragOver = useCallback((e: DragEvent<HTMLElement>) => {
		e.preventDefault();
		e.stopPropagation();
	}, []);

	const handleDrop = useCallback(
		(e: DragEvent<HTMLElement>) => {
			e.preventDefault();
			e.stopPropagation();
			setIsDragging(false);

			// Don't process files if the input is disabled
			if (inputRef.current?.disabled) {
				return;
			}

			if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
				// In single file mode, only use the first file
				if (!multiple) {
					const file = e.dataTransfer.files[0];
					addFiles([file]);
				} else {
					addFiles(e.dataTransfer.files);
				}
			}
		},
		[addFiles, multiple, setIsDragging],
	);

	const handleFileChange = useCallback(
		(e: ChangeEvent<HTMLInputElement>) => {
			if (e.target.files && e.target.files.length > 0) {
				addFiles(e.target.files);
			}
		},
		[addFiles],
	);

	const openFileDialog = useCallback(() => {
		if (inputRef.current) {
			inputRef.current.click();
		}
	}, []);

	const getInputProps = useCallback(
		(props: InputHTMLAttributes<HTMLInputElement> = {}) => {
			return {
				...props,
				type: "file" as const,
				onChange: handleFileChange,
				accept: props.accept || accept,
				multiple: props.multiple !== undefined ? props.multiple : multiple,
				// Cast to `any` to prevent mismatched React ref type errors across workspaces
				ref: inputRef as any,
			};
		},
		[accept, multiple, handleFileChange],
	);

	return {
		addFiles,
		removeFile,
		clearFiles,
		clearErrors,
		handleDragEnter,
		handleDragLeave,
		handleDragOver,
		handleDrop,
		handleFileChange,
		openFileDialog,
		getInputProps,
	};
};

// Helper function to format bytes to human-readable format
