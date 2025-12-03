import { create } from "zustand";
import createSelectors from "./selectors";

export type FileMetadata = {
	name: string;
	size: number;
	type: string;
};

type FileStreamsByName = Record<
	string,
	{
		stream: ReadableStream<Uint8Array>;
		enqueue: (chunk: Uint8Array) => void;
		close: () => void;
		isClosed: boolean;
	}
>;

export enum ReceiverStatus {
	IDLE = "idle",
	WS_CONNECTING = "ws_connecting",
	CONNECTING = "connecting",
	READY = "ready",
	RECEIVING_FILE = "receiving_file",
	COMPLETED = "completed",
	ERROR = "error",
}

type State = {
	filesMetadata: FileMetadata[];
	fileStreamsByName: FileStreamsByName;
	bytesDownloaded: number;
	currentFileIndex: number;
	fileProgressByName: Record<string, number>;
	roomId: string | null;
	status: ReceiverStatus;
	error: string | null;
};

type Actions = {
	actions: {
		setFilesMetadata: (filesMetadata: FileMetadata[]) => void;
		setFileStreamsByName: (fileStreamsByName: FileStreamsByName) => void;
		setBytesDownloaded: (bytesDownloaded: number) => void;
		setCurrentFileIndex: (currentFileIndex: number) => void;
		setFileProgress: (fileName: string, progress: number) => void;
		setRoomId: (roomId: string) => void;
		setStatus: (status: ReceiverStatus) => void;
		setError: (error: string | null) => void;
		clearError: () => void;
		reset: () => void;
	};
};

const initialState: State = {
	filesMetadata: [],
	fileStreamsByName: {},
	bytesDownloaded: 0,
	currentFileIndex: 0,
	fileProgressByName: {},
	roomId: null,
	status: ReceiverStatus.IDLE,
	error: null,
};

const useReceiverStoreBase = create<State & Actions>()((set) => ({
	...initialState,
	actions: {
		setFilesMetadata: (filesMetadata) =>
			set((state) => ({
				filesMetadata: [...state.filesMetadata, ...filesMetadata],
			})),
		setFileStreamsByName: (fileStreamsByName) => set({ fileStreamsByName }),
		setBytesDownloaded: (bytesDownloaded) =>
			set((state) => ({
				bytesDownloaded: state.bytesDownloaded + bytesDownloaded,
			})),
		setCurrentFileIndex: (currentFileIndex) => set({ currentFileIndex }),
		setFileProgress: (fileName, progress) =>
			set((state) => ({
				fileProgressByName: {
					...state.fileProgressByName,
					[fileName]: progress,
				},
			})),
		setRoomId: (roomId) => set({ roomId }),
		setStatus: (status) => set({ status }),
		setError: (error) =>
			set({
				error,
				status: error ? ReceiverStatus.ERROR : ReceiverStatus.IDLE,
			}),
		clearError: () => set({ error: null, status: ReceiverStatus.IDLE }),
		reset: () => set(initialState),
	},
}));

export const useReceiverActions = () =>
	useReceiverStoreBase((state) => state.actions);

const useReceiverStore = createSelectors(useReceiverStoreBase);

export default useReceiverStore;
