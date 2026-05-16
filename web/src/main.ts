import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from './stores/auth'
import { setCsrfTokenGetter } from './api/client'
import router from './router'
import App from './App.vue'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)

// Pinia 初期化後に CSRF トークンゲッターを登録
const authStore = useAuthStore()
setCsrfTokenGetter(() => authStore.csrfToken)

app.mount('#app')
