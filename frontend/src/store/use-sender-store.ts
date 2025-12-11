import { create } from "zustand";
import createSelectors from "./selectors";

type ConnectedDevice = {
	deviceName: string;
	deviceVersion: string;
};
export enum SenderStatus {
	IDLE = "idle",
	CONNECTING = "connecting",
	WS_CONNECTING = "ws_connecting",
	READY = "ready",
	SENDING = "sending",
	COMPLETED = "completed",
	ERROR = "error",
}

type State = {
	connectedDevices: ConnectedDevice[];
	currentFileName: string;
	currentFileOffset: number;
	currentFileProgress: number;
	completedFileCount: number;
	roomId: string | null;
	status: SenderStatus;
	error: string | null;
	transferSpeed: string;
	estimatedTimeRemaining: string;
};

type Actions = {
	actions: {
		setConnectedDevice: (connectedDevice: ConnectedDevice) => void;
		removeConnectedDevice: () => void;
		setCurrentFileName: (name: string) => void;
		setCurrentFileOffset: (offset: number) => void;
		setCurrentFileProgress: (progress: number) => void;
		setCompletedFileCount: (count: number) => void;
		setRoomId: (roomId: string) => void;
		setStatus: (status: SenderStatus) => void;
		setError: (error: string | null) => void;
		setTransferSpeed: (speed: string) => void;
		setEstimatedTimeRemaining: (eta: string) => void;
		clearError: () => void;
		reset: () => void;
	};
};

const initialState: State = {
	connectedDevices: [],
	currentFileName: "",
	currentFileOffset: 0,
	currentFileProgress: 0,
	completedFileCount: 0,
	roomId: null,
	status: SenderStatus.IDLE,
	error: null,
	transferSpeed: "0 B/s",
	estimatedTimeRemaining: "--",
};

const useSenderStoreBase = create<State & Actions>()((set) => ({
	...initialState,
	actions: {
		setConnectedDevice: (connectedDevice) =>
			set((state) => ({
				connectedDevices: [...state.connectedDevices, connectedDevice],
			})),
		removeConnectedDevice: () => set({ connectedDevices: [] }),
		setCurrentFileName: (name) => set({ currentFileName: name }),
		setCurrentFileOffset: (offset) => set({ currentFileOffset: offset }),
		setCurrentFileProgress: (progress) =>
			set({ currentFileProgress: progress }),
		setCompletedFileCount: (count) => set({ completedFileCount: count }),
		setRoomId: (roomId) => set({ roomId }),
		setStatus: (status) => set({ status }),
		setError: (error) =>
			set({ error, status: error ? SenderStatus.ERROR : SenderStatus.IDLE }),
		setTransferSpeed: (speed) => set({ transferSpeed: speed }),
		setEstimatedTimeRemaining: (eta) => set({ estimatedTimeRemaining: eta }),
		clearError: () => set({ error: null, status: SenderStatus.IDLE }),
		reset: () => set(initialState),
	},
}));

export const useSenderActions = () =>
	useSenderStoreBase((state) => state.actions);

const useSenderStore = createSelectors(useSenderStoreBase);

export default useSenderStore;
