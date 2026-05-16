import { defineStore } from 'pinia';
import { apiGetReady } from '../api/system';
export const useAppStore = defineStore('app', {
    state: () => ({
        initialized: false,
        binMissing: [],
        version: '',
        ready: false,
    }),
    getters: {
        frpcMissing: (state) => state.binMissing.includes('frpc'),
        frpsMissing: (state) => state.binMissing.includes('frps'),
    },
    actions: {
        async fetchReady() {
            try {
                const info = await apiGetReady();
                this.initialized = info.initialized;
                this.binMissing = info.binMissing ?? [];
                this.version = info.version ?? '';
                this.ready = true;
            }
            catch {
                this.ready = false;
            }
        },
    },
});
