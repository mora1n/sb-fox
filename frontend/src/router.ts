import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from './stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('./views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: () => import('./components/AppShell.vue'),
      children: [
        { path: '', name: 'dashboard', component: () => import('./views/DashboardView.vue') },
        { path: 'nodes', name: 'nodes', component: () => import('./views/NodesView.vue') },
        { path: 'templates', name: 'templates', component: () => import('./views/TemplatesView.vue') },
        { path: 'profiles', name: 'profiles', component: () => import('./views/ProfilesView.vue') },
        { path: 'preview', name: 'preview', component: () => import('./views/ConfigPreviewView.vue') },
        { path: 'users', name: 'users', component: () => import('./views/UsersView.vue'), meta: { admin: true } },
        { path: 'settings', name: 'settings', component: () => import('./views/SettingsView.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

// Global guard: every non-public route requires an authenticated session.
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (to.meta.public) return true
  if (!auth.checked) {
    await auth.me()
  }
  if (!auth.isAuthenticated) {
    return { name: 'login' }
  }
  if (to.meta.admin && !auth.isAdmin) {
    return { name: 'dashboard' }
  }
  return true
})

export default router
