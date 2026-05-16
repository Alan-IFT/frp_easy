import { defineStore } from 'pinia';
import { apiDownloadBin, apiDownloadStatus } from '../api/downloader';
const idleState = () => ({ status: 'idle', progress: 0 });
export const useDownloaderStore = defineStore('downloader', {
    state: () => ({
        frpc: idleState(),
        frps: idleState(),
        _timers: {},
    }),
    getters: {
        isDownloading: (state) => (kind) => state[kind].status === 'downloading',
    },
    actions: {
        async downloadBin(kind) {
            try {
                await apiDownloadBin(kind);
            }
            catch {
                // 409 PROC_BUSY means already downloading — just start polling
            }
            this.startPolling(kind);
        },
        startPolling(kind) {
            // Stop any existing polling for this kind
            this.stopPolling(kind);
            const timer = setInterval(async () => {
                try {
                    const state = await apiDownloadStatus(kind);
                    this[kind] = state;
                    if (state.status !== 'downloading') {
                        this.stopPolling(kind);
                    }
                }
                catch {
                    // ignore transient poll errors
                }
            }, 1000);
            this._timers[kind] = timer;
        },
        stopPolling(kind) {
            if (this._timers[kind]) {
                clearInterval(this._timers[kind]);
                delete this._timers[kind];
            }
        },
        downloadState(kind) {
            return this[kind];
        },
    },
});
