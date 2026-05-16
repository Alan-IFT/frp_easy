import apiClient from './client';
export async function apiListProxies() {
    const res = await apiClient.get('/api/v1/proxies');
    return res.data;
}
export async function apiCreateProxy(input) {
    const res = await apiClient.post('/api/v1/proxies', input);
    return res.data;
}
export async function apiUpdateProxy(id, input) {
    const res = await apiClient.put(`/api/v1/proxies/${id}`, input);
    return res.data;
}
export async function apiDeleteProxy(id) {
    await apiClient.delete(`/api/v1/proxies/${id}`);
}
