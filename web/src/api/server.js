import apiClient from './client';
export async function apiGetServer(reveal = false) {
    const params = reveal ? { reveal: '1' } : {};
    const res = await apiClient.get('/api/v1/server', { params });
    return res.data;
}
export async function apiPutServer(config) {
    const res = await apiClient.put('/api/v1/server', config);
    return res.data;
}
