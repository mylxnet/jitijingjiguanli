import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore, ORG_NAME_KEY } from '../features/auth/store'

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
      path: '/reset-password',
      name: 'ResetPassword',
      component: () => import('../features/auth/ResetPassword.vue'),
    },
    {
      path: '/contacts',
      component: () => import('../features/contacts/ContactsLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: '/contacts/overview' },
        { path: 'overview',   name: 'ContactsOverview', component: () => import('../features/contacts/pages/Overview.vue') },
        { path: 'parties',    name: 'ContactsParties',  component: () => import('../features/contacts/pages/PartiesList.vue') },
        { path: 'receivables/dividend', name: 'ContactsDividend', component: () => import('../features/contacts/pages/ReceivablesList.vue'), props: { kind: ['dividend','reinvest_dividend'], theme: 'blue',  pageTitle: '应收投资收益' } },
        { path: 'receivables/rent',     name: 'ContactsRent',     component: () => import('../features/contacts/pages/ReceivablesList.vue'), props: { kind: ['rent'],                      theme: 'orange', pageTitle: '应收土地流转费' } },
        { path: 'receivables/service',  name: 'ContactsService',  component: () => import('../features/contacts/pages/ReceivablesList.vue'), props: { kind: ['service'],                   theme: 'purple', pageTitle: '应收管理费' } },
        { path: 'contracts',   name: 'ContactsContracts',component: () => import('../features/contacts/pages/ContractsPage.vue') },
        { path: 'accrue',      name: 'ContactsAccrue',    component: () => import('../features/contacts/pages/AccrueWizard.vue') },
      ],
    },
    {
      path: '/investment',
      component: () => import('../features/investment/InvestmentLayout.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/flow',
      component: () => import('../features/flow/FlowLayout.vue'),
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
    {
      path: '/onboarding',
      name: 'Onboarding',
      component: () => import('../features/onboarding/OnboardingPage.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach(async (to, _from, next) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    await auth.checkLogin()
  }
  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next('/login')
    return
  }
  // 引导入口：已登录用户首次进入主流程页且未完成引导时，强制转到引导页
  const noOnboardPaths = ['/login', '/register', '/reset-password', '/onboarding']
  if (to.meta.requiresAuth && !noOnboardPaths.includes(to.path) && auth.orgId &&
      !localStorage.getItem(`jt_onboarding_done_${auth.orgId}`)) {
    next('/onboarding')
    return
  }
  next()
})

// 标签页标题统一为「{注册组织名}集体经济管理平台」；未登录时不带组织名前缀。
router.afterEach(() => {
  const auth = useAuthStore()
  const orgName = auth.isLoggedIn ? (localStorage.getItem(ORG_NAME_KEY) || '') : ''
  document.title = orgName ? `${orgName}集体经济管理平台` : '集体经济管理平台'
})

export default router