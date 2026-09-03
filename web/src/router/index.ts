import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../features/auth/store'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('../features/auth/Login.vue'),
    },
    {
      path: '/register',
      name: 'Register',
      component: () => import('../features/auth/Register.vue'),
    },
    {
      path: '/contacts',
      name: 'Contacts',
      component: () => import('../features/contacts/ContactsPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/',
      name: 'Home',
      component: () => import('../features/transaction/Home.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/transactions',
      name: 'Transactions',
      component: () => import('../features/transaction/List.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/summary',
      name: 'Summary',
      component: () => import('../features/summary/SummaryPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/categories',
      name: 'Categories',
      component: () => import('../features/category/CategoryPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/settings',
      name: 'Settings',
      component: () => import('../features/settings/SettingsPage.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next('/login')
  } else {
    next()
  }
})

export default router