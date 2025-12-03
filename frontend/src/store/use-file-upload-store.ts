import { create } from "zustand";
import createSelectors from "./selectors";

export type FileWithPreview = {
	file: File;
	id: string;
	preview?: string;
};

type State = {
	files: FileWithPreview[];
	isDragging: boolean;
	errors: string[];
};

type Actions = {
	actions: {
		setFiles: (files: FileWithPreview[]) => void;
		addFiles: (files: FileWithPreview[]) => void;
		removeFile: (id: string) => void;
		clearFiles: () => void;
		setIsDragging: (isDragging: boolean) => void;
		setErrors: (errors: string[]) => void;
		addError: (error: string) => void;
		clearErrors: () => void;
	};
};

const initialState: State = {
	files: [],
	isDragging: false,
	errors: [],
};

const useFileUploadStoreBase = create<State & Actions>()((set) => ({
	...initialState,
	actions: {
		setFiles: (files) => set({ files }),
		addFiles: (newFiles) =>
			set((state) => ({
				files: [...state.files, ...newFiles],
			})),
		removeFile: (id) =>
			set((state) => {
				const fileToRemove = state.files.find((file) => file.id === id);
				if (
					fileToRemove?.preview &&
					fileToRemove.file instanceof File &&
					fileToRemove.file.type.startsWith("image/")
				) {
					URL.revokeObjectURL(fileToRemove.preview);
				}
				return {
					files: state.files.filter((file) => file.id !== id),
					errors: [],
				};
			}),
		clearFiles: () =>
			set((state) => {
				// Clean up object URLs
				state.files.forEach((file) => {
					if (
						file.preview &&
						file.file instanceof File &&
						file.file.type.startsWith("image/")
					) {
						URL.revokeObjectURL(file.preview);
					}
				});
				return {
					files: [],
					errors: [],
				};
			}),
		setIsDragging: (isDragging) => set({ isDragging }),
		setErrors: (errors) => set({ errors }),
		addError: (error) =>
			set((state) => ({
				errors: [...state.errors, error],
			})),
		clearErrors: () => set({ errors: [] }),
	},
}));

export const useFileUploadActions = () =>
	useFileUploadStoreBase((state) => state.actions);

const useFileUploadStore = createSelectors(useFileUploadStoreBase);

export default useFileUploadStore;
