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
  // 会话失效：同时清掉 orgId，避免重新登录后被旧组织的引导标记判定影响
  auth.clearAuth()
  router.push('/login')
})

app.mount('#app')
