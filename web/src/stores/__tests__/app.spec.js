import { describe, it, expect, vi, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useAppStore } from '../app';
// system API をモック
vi.mock('../../api/system', () => ({
    apiGetReady: vi.fn(),
}));
import * as systemApi from '../../api/system';
describe('useAppStore', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.clearAllMocks();
    });
    it('初始状态：未初始化、无二进制缺失', () => {
        const store = useAppStore();
        expect(store.initialized).toBe(false);
        expect(store.binMissing).toEqual([]);
        expect(store.version).toBe('');
        expect(store.ready).toBe(false);
    });
    it('fetchReady 成功后 initialized、binMissing、version 更新', async () => {
        vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
            initialized: true,
            binMissing: [],
            version: '1.2.3',
        });
        const store = useAppStore();
        await store.fetchReady();
        expect(store.initialized).toBe(true);
        expect(store.binMissing).toEqual([]);
        expect(store.version).toBe('1.2.3');
        expect(store.ready).toBe(true);
    });
    it('fetchReady 更新二进制缺失列表', async () => {
        vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
            initialized: true,
            binMissing: ['frpc', 'frps'],
            version: '1.0.0',
        });
        const store = useAppStore();
        await store.fetchReady();
        expect(store.binMissing).toEqual(['frpc', 'frps']);
    });
    it('fetchReady 出错时 ready 保持 false', async () => {
        vi.mocked(systemApi.apiGetReady).mockRejectedValueOnce(new Error('Network Error'));
        const store = useAppStore();
        await store.fetchReady();
        expect(store.ready).toBe(false);
        expect(store.initialized).toBe(false);
    });
    describe('frpcMissing getter', () => {
        it('binMissing 包含 frpc 时为 true', async () => {
            vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
                initialized: true,
                binMissing: ['frpc'],
                version: '1.0.0',
            });
            const store = useAppStore();
            await store.fetchReady();
            expect(store.frpcMissing).toBe(true);
        });
        it('binMissing 不包含 frpc 时为 false', async () => {
            vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
                initialized: true,
                binMissing: ['frps'],
                version: '1.0.0',
            });
            const store = useAppStore();
            await store.fetchReady();
            expect(store.frpcMissing).toBe(false);
        });
        it('frpsMissing：binMissing 包含 frps 时为 true', async () => {
            vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
                initialized: true,
                binMissing: ['frps'],
                version: '1.0.0',
            });
            const store = useAppStore();
            await store.fetchReady();
            expect(store.frpsMissing).toBe(true);
        });
    });
});
