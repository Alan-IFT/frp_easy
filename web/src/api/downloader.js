import apiClient from './client';
export async function apiDownloadBin(kind) {
    await apiClient.post('/api/v1/system/download-bin', { kind });
}
export async function apiDownloadStatus(kind) {
    const res = await apiClient.get(`/api/v1/system/download-status/${kind}`);
    return res.data;
}
