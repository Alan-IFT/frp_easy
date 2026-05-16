import apiClient from './client';
export async function apiGetMode() {
    const res = await apiClient.get('/api/v1/mode');
    return res.data;
}
export async function apiPutMode(state) {
    const res = await apiClient.put('/api/v1/mode', state);
    return res.data;
}
