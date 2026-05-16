import { describe, it, expect, vi, beforeEach } from 'vitest';
import { apiGetMode, apiPutMode } from '../mode';
// apiClient をモック
vi.mock('../client', () => ({
    default: {
        get: vi.fn(),
        put: vi.fn(),
    },
    setCsrfTokenGetter: vi.fn(),
    extractApiError: vi.fn(),
    extractErrorMessage: vi.fn(),
}));
import apiClient from '../client';
describe('mode API', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });
    describe('apiGetMode', () => {
        it('返回 GET /api/v1/mode 的结果', async () => {
            const mockState = { frpc: true, frps: false };
            vi.mocked(apiClient.get).mockResolvedValueOnce({ data: mockState });
            const result = await apiGetMode();
            expect(apiClient.get).toHaveBeenCalledWith('/api/v1/mode');
            expect(result).toEqual(mockState);
        });
        it('frpc=false frps=true 的响应也能正确返回', async () => {
            const mockState = { frpc: false, frps: true };
            vi.mocked(apiClient.get).mockResolvedValueOnce({ data: mockState });
            const result = await apiGetMode();
            expect(result.frpc).toBe(false);
            expect(result.frps).toBe(true);
        });
        it('API 出错时传播异常', async () => {
            vi.mocked(apiClient.get).mockRejectedValueOnce(new Error('Network Error'));
            await expect(apiGetMode()).rejects.toThrow('Network Error');
        });
    });
    describe('apiPutMode', () => {
        it('向 PUT /api/v1/mode 发送模式状态', async () => {
            const reqState = { frpc: true, frps: false };
            const respState = { frpc: true, frps: false };
            vi.mocked(apiClient.put).mockResolvedValueOnce({ data: respState });
            const result = await apiPutMode(reqState);
            expect(apiClient.put).toHaveBeenCalledWith('/api/v1/mode', reqState);
            expect(result).toEqual(respState);
        });
        it('正确发送 frpc=false frps=false 的停用请求', async () => {
            const reqState = { frpc: false, frps: false };
            vi.mocked(apiClient.put).mockResolvedValueOnce({ data: reqState });
            const result = await apiPutMode(reqState);
            expect(result.frpc).toBe(false);
            expect(result.frps).toBe(false);
        });
        it('API 出错时传播异常', async () => {
            const reqState = { frpc: true, frps: true };
            vi.mocked(apiClient.put).mockRejectedValueOnce(new Error('Unauthorized'));
            await expect(apiPutMode(reqState)).rejects.toThrow('Unauthorized');
        });
    });
});
