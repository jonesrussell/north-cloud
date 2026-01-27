import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { installVueQuery } from './plugins/query'
import './style.css'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)

// Install Vue Query for server state management
installVueQuery(app)

// Initialize realtime store after Pinia is ready
import { useRealtimeStore } from './stores/realtime'
const realtimeStore = useRealtimeStore()
realtimeStore.init()

app.mount('#app')
