import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useProcStore } from '../proc';
// API モック
vi.mock('../../api/proc', () => ({
    apiGetProcStatus: vi.fn(),
    apiStartProc: vi.fn(),
    apiStopProc: vi.fn(),
    apiRestartProc: vi.fn(),
}));
import * as procApi from '../../api/proc';
const mockStatusAll = {
    frpc: {
        kind: 'frpc',
        state: 'running',
        pid: 1234,
        lastErr: '',
        changedAt: '2026-05-16T00:00:00Z',
    },
    frps: {
        kind: 'frps',
        state: 'stopped',
        pid: 0,
        lastErr: '',
        changedAt: '2026-05-16T00:00:00Z',
    },
};
describe('useProcStore', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.clearAllMocks();
        vi.useFakeTimers();
    });
    afterEach(() => {
        vi.useRealTimers();
        const store = useProcStore();
        store.stopPolling();
    });
    it('初始状态为 null', () => {
        const store = useProcStore();
        expect(store.frpc).toBeNull();
        expect(store.frps).toBeNull();
    });
    it('frpcInfo 默认状态为 stopped', () => {
        const store = useProcStore();
        expect(store.frpcInfo.state).toBe('stopped');
        expect(store.frpcInfo.kind).toBe('frpc');
    });
    it('pollStatus 更新进程状态', async () => {
        vi.mocked(procApi.apiGetProcStatus).mockResolvedValueOnce(mockStatusAll);
        const store = useProcStore();
        await store.pollStatus();
        expect(store.frpc?.state).toBe('running');
        expect(store.frpc?.pid).toBe(1234);
        expect(store.frps?.state).toBe('stopped');
    });
    it('pollStatus 出错时状态不变', async () => {
        vi.mocked(procApi.apiGetProcStatus).mockRejectedValueOnce(new Error('network error'));
        const store = useProcStore();
        await store.pollStatus();
        expect(store.frpc).toBeNull();
        expect(store.frps).toBeNull();
    });
    it('startPolling 每 2 秒轮询', async () => {
        vi.mocked(procApi.apiGetProcStatus).mockResolvedValue(mockStatusAll);
        const store = useProcStore();
        store.startPolling();
        // 初回は即時呼ばれる
        await vi.runAllTicks();
        expect(vi.mocked(procApi.apiGetProcStatus)).toHaveBeenCalledTimes(1);
        // 2秒後に追加呼び出し
        vi.advanceTimersByTime(2000);
        await vi.runAllTicks();
        expect(vi.mocked(procApi.apiGetProcStatus)).toHaveBeenCalledTimes(2);
        store.stopPolling();
    });
    it('stopPolling 停止轮询', async () => {
        vi.mocked(procApi.apiGetProcStatus).mockResolvedValue(mockStatusAll);
        const store = useProcStore();
        store.startPolling();
        await vi.runAllTicks();
        store.stopPolling();
        vi.advanceTimersByTime(4000);
        await vi.runAllTicks();
        // 初回の 1 回のみ（stopPolling 後は呼ばれない）
        expect(vi.mocked(procApi.apiGetProcStatus)).toHaveBeenCalledTimes(1);
    });
    it('startProc 更新 frpc 状态', async () => {
        const runningInfo = { kind: 'frpc', state: 'running', pid: 999, changedAt: '2026-05-16T00:00:00Z' };
        vi.mocked(procApi.apiStartProc).mockResolvedValueOnce(runningInfo);
        const store = useProcStore();
        const info = await store.startProc('frpc');
        expect(info.state).toBe('running');
        expect(store.frpc?.pid).toBe(999);
    });
});
