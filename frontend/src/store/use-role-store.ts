import { create } from "zustand";
import createSelectors from "./selectors";

type State = {
	isSender: boolean;
};

type Actions = {
	actions: {
		setIsSender: (isSender: boolean) => void;
	};
};

const initialState: State = {
	isSender: false,
};

const useRoleStoreBase = create<State & Actions>()((set) => ({
	...initialState,
	actions: {
		setIsSender: (isSender: boolean) => set({ isSender }),
	},
}));

export const useRoleActions = () => useRoleStoreBase((state) => state.actions);

const useRoleStore = createSelectors(useRoleStoreBase);

export default useRoleStore;
