import { defineStore } from 'pinia';
import { apiListProxies, apiCreateProxy, apiUpdateProxy, apiDeleteProxy } from '../api/proxies';
export const useProxiesStore = defineStore('proxies', {
    state: () => ({
        proxies: [],
        loading: false,
    }),
    actions: {
        async fetchProxies() {
            this.loading = true;
            try {
                this.proxies = await apiListProxies();
            }
            finally {
                this.loading = false;
            }
        },
        async createProxy(input) {
            const proxy = await apiCreateProxy(input);
            this.proxies.push(proxy);
            return proxy;
        },
        async updateProxy(id, input) {
            const proxy = await apiUpdateProxy(id, input);
            const idx = this.proxies.findIndex((p) => p.id === id);
            if (idx >= 0)
                this.proxies[idx] = proxy;
            return proxy;
        },
        async deleteProxy(id) {
            await apiDeleteProxy(id);
            this.proxies = this.proxies.filter((p) => p.id !== id);
        },
    },
});
