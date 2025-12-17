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
	downloadStarted: number;
	fileStreamsByName: FileStreamsByName;
	bytesDownloaded: number;
	currentFileIndex: number;
	fileProgressByName: Record<string, number>;
	roomId: string | null;
	status: ReceiverStatus;
	error: string | null;
	transferSpeed: string;
	estimatedTimeRemaining: string;
};

type Actions = {
	actions: {
		setFilesMetadata: (filesMetadata: FileMetadata[]) => void;
		setDownloadStarted: (downloadStarted: number) => void;
		setFileStreamsByName: (fileStreamsByName: FileStreamsByName) => void;
		setBytesDownloaded: (bytesDownloaded: number) => void;
		setCurrentFileIndex: (currentFileIndex: number) => void;
		setFileProgress: (fileName: string, progress: number) => void;
		setRoomId: (roomId: string) => void;
		setStatus: (status: ReceiverStatus) => void;
		setError: (error: string | null) => void;
		setTransferSpeed: (speed: string) => void;
		setEstimatedTimeRemaining: (eta: string) => void;
		clearError: () => void;
		reset: () => void;
	};
};

const initialState: State = {
	filesMetadata: [],
	downloadStarted: 0,
	fileStreamsByName: {},
	bytesDownloaded: 0,
	currentFileIndex: 0,
	fileProgressByName: {},
	roomId: null,
	status: ReceiverStatus.IDLE,
	error: null,
	transferSpeed: "0 B/s",
	estimatedTimeRemaining: "--",
};

const useReceiverStoreBase = create<State & Actions>()((set) => ({
	...initialState,
	actions: {
		setFilesMetadata: (filesMetadata) =>
			set((state) => ({
				filesMetadata: [...state.filesMetadata, ...filesMetadata],
			})),
		setDownloadStarted: (downloadStarted) => set({ downloadStarted }),
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
		setTransferSpeed: (speed) => set({ transferSpeed: speed }),
		setEstimatedTimeRemaining: (eta) => set({ estimatedTimeRemaining: eta }),
		clearError: () => set({ error: null, status: ReceiverStatus.IDLE }),
		reset: () => set(initialState),
	},
}));

export const useReceiverActions = () =>
	useReceiverStoreBase((state) => state.actions);

const useReceiverStore = createSelectors(useReceiverStoreBase);

export default useReceiverStore;
