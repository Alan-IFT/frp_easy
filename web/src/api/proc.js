import apiClient from './client';
export async function apiStartProc(kind) {
    const res = await apiClient.post(`/api/v1/proc/${kind}/start`);
    return res.data;
}
export async function apiStopProc(kind) {
    const res = await apiClient.post(`/api/v1/proc/${kind}/stop`);
    return res.data;
}
export async function apiRestartProc(kind) {
    const res = await apiClient.post(`/api/v1/proc/${kind}/restart`);
    return res.data;
}
export async function apiGetProcStatus() {
    const res = await apiClient.get('/api/v1/proc/status');
    return res.data;
}
