import { defineStore } from 'pinia';
import { apiGetProcStatus, apiStartProc, apiStopProc, apiRestartProc } from '../api/proc';
const defaultInfo = (kind) => ({
    kind,
    state: 'stopped',
    pid: 0,
    lastErr: '',
    changedAt: new Date().toISOString(),
});
export const useProcStore = defineStore('proc', {
    state: () => ({
        frpc: null,
        frps: null,
        pollingTimer: null,
    }),
    getters: {
        frpcInfo: (state) => state.frpc ?? defaultInfo('frpc'),
        frpsInfo: (state) => state.frps ?? defaultInfo('frps'),
    },
    actions: {
        async pollStatus() {
            try {
                const status = await apiGetProcStatus();
                this.frpc = status.frpc;
                this.frps = status.frps;
            }
            catch {
                // 忽略临时错误
            }
        },
        startPolling() {
            if (this.pollingTimer !== null)
                return;
            void this.pollStatus();
            this.pollingTimer = setInterval(() => {
                void this.pollStatus();
            }, 2000);
        },
        stopPolling() {
            if (this.pollingTimer !== null) {
                clearInterval(this.pollingTimer);
                this.pollingTimer = null;
            }
        },
        async startProc(kind) {
            const info = await apiStartProc(kind);
            if (kind === 'frpc')
                this.frpc = info;
            else if (kind === 'frps')
                this.frps = info;
            return info;
        },
        async stopProc(kind) {
            const info = await apiStopProc(kind);
            if (kind === 'frpc')
                this.frpc = info;
            else if (kind === 'frps')
                this.frps = info;
            return info;
        },
        async restartProc(kind) {
            const info = await apiRestartProc(kind);
            if (kind === 'frpc')
                this.frpc = info;
            else if (kind === 'frps')
                this.frps = info;
            return info;
        },
    },
});
