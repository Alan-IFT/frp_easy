// frpc 客户端配置 API
// 避免文件名冲突，故命名为 frpclient.ts
import apiClient from './client';
export async function apiGetClient(reveal = false) {
    const params = reveal ? { reveal: '1' } : {};
    const res = await apiClient.get('/api/v1/client', { params });
    return res.data;
}
export async function apiPutClient(config) {
    const res = await apiClient.put('/api/v1/client', config);
    return res.data;
}
