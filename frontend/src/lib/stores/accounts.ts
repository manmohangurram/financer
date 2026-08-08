import { reactive } from 'vue';

const state = reactive<{ selectedAccountId: string | null }>({ selectedAccountId: null });

export function useAccountsStore() {
  function select(id: string | null) {
    state.selectedAccountId = id;
  }
  return { state, select };
}
