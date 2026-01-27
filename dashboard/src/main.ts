import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './style.css'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)

// Initialize realtime store after Pinia is ready
import { useRealtimeStore } from './stores/realtime'
const realtimeStore = useRealtimeStore()
realtimeStore.init()

app.mount('#app')

