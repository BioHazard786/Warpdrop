import { create } from "zustand";
import createSelectors from "./selectors";

type State = {
	peerConnection: RTCPeerConnection | null;
	dataChannel: RTCDataChannel | null;
};

type Actions = {
	actions: {
		setPeerConnection: (pc: RTCPeerConnection | null) => void;
		setDataChannel: (dc: RTCDataChannel | null) => void;
		resetWebRTC: () => void;
	};
};

const initialState: State = {
	peerConnection: null,
	dataChannel: null,
};

const useRTCStoreBase = create<State & Actions>()((set) => ({
	...initialState,
	actions: {
		setPeerConnection: (pc: RTCPeerConnection | null) =>
			set({ peerConnection: pc }),
		setDataChannel: (dc: RTCDataChannel | null) => set({ dataChannel: dc }),
		resetWebRTC: () =>
			set((state) => {
				if (state.dataChannel) state.dataChannel.close();
				if (state.peerConnection) state.peerConnection.close();

				return { ...initialState };
			}),
	},
}));

export const useRTCActions = () => useRTCStoreBase((state) => state.actions);

const useRTCStore = createSelectors(useRTCStoreBase);

export default useRTCStore;
