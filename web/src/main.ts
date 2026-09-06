import { createApp } from 'vue'
import { createPinia } from 'pinia'
import Vant from 'vant'
import App from './App.vue'
import router from './router'
import { setOnUnauthorized } from './lib/http'
import { useAuthStore } from './features/auth/store'

import 'vant/lib/index.css'
import './styles/theme.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(Vant)

setOnUnauthorized(() => {
  const auth = useAuthStore(pinia)
  auth.isLoggedIn = false
  router.push('/login')
})

app.mount('#app')
