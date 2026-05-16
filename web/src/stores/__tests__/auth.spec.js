import { describe, it, expect, vi, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useAuthStore } from '../auth';
// API モック
vi.mock('../../api/auth', () => ({
    apiSetup: vi.fn(),
    apiLogin: vi.fn(),
    apiLogout: vi.fn(),
    apiGetMe: vi.fn(),
    apiGetCsrf: vi.fn(),
    apiChangePassword: vi.fn(),
}));
vi.mock('../../api/client', () => ({
    default: {},
    setCsrfTokenGetter: vi.fn(),
    extractApiError: vi.fn(),
    extractErrorMessage: vi.fn(),
}));
import * as authApi from '../../api/auth';
describe('useAuthStore', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.clearAllMocks();
    });
    it('初始状态：未登录', () => {
        const store = useAuthStore();
        expect(store.user).toBeNull();
        expect(store.csrfToken).toBe('');
    });
    it('login 成功后 user 和 csrfToken 被设置', async () => {
        vi.mocked(authApi.apiLogin).mockResolvedValueOnce({ ok: true });
        vi.mocked(authApi.apiGetCsrf).mockResolvedValueOnce({ csrfToken: 'test-csrf-token' });
        const store = useAuthStore();
        await store.login('admin', 'password123');
        expect(store.user).toBe('admin');
        expect(store.csrfToken).toBe('test-csrf-token');
    });
    it('login 失败时抛出错误', async () => {
        vi.mocked(authApi.apiLogin).mockRejectedValueOnce(new Error('401 Unauthorized'));
        const store = useAuthStore();
        await expect(store.login('admin', 'wrong')).rejects.toThrow();
        expect(store.user).toBeNull();
    });
    it('logout 后 user 和 csrfToken 被清空', async () => {
        vi.mocked(authApi.apiLogout).mockResolvedValueOnce(undefined);
        const store = useAuthStore();
        store.user = 'admin';
        store.csrfToken = 'some-token';
        await store.logout();
        expect(store.user).toBeNull();
        expect(store.csrfToken).toBe('');
    });
    it('checkMe 成功时返回 true 并设置用户', async () => {
        vi.mocked(authApi.apiGetMe).mockResolvedValueOnce({ username: 'admin' });
        vi.mocked(authApi.apiGetCsrf).mockResolvedValueOnce({ csrfToken: 'csrf-123' });
        const store = useAuthStore();
        const result = await store.checkMe();
        expect(result).toBe(true);
        expect(store.user).toBe('admin');
        expect(store.csrfToken).toBe('csrf-123');
    });
    it('checkMe 失败时返回 false', async () => {
        vi.mocked(authApi.apiGetMe).mockRejectedValueOnce(new Error('401'));
        const store = useAuthStore();
        const result = await store.checkMe();
        expect(result).toBe(false);
        expect(store.user).toBeNull();
    });
    it('fetchCsrf 成功时设置 csrfToken', async () => {
        vi.mocked(authApi.apiGetCsrf).mockResolvedValueOnce({ csrfToken: 'new-csrf' });
        const store = useAuthStore();
        await store.fetchCsrf();
        expect(store.csrfToken).toBe('new-csrf');
    });
});
