import apiClient from './client';
export async function apiGetReady() {
    const res = await apiClient.get('/api/v1/system/ready');
    return res.data;
}
export async function apiGetPublicIP() {
    const res = await apiClient.get('/api/v1/system/public-ip');
    return res.data;
}
