import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from './stores/auth'

const appRouteLoaders = {
  dashboard: () => import('./views/DashboardView.vue'),
  nodes: () => import('./views/NodesView.vue'),
  templates: () => import('./views/TemplatesView.vue'),
  ruleSets: () => import('./views/RuleSetsView.vue'),
  profiles: () => import('./views/ProfilesView.vue'),
  settings: () => import('./views/SettingsView.vue'),
  users: () => import('./views/UsersView.vue'),
}

type AppRouteName = keyof typeof appRouteLoaders

export async function preloadAppRouteViews(includeAdmin = false): Promise<void> {
  const names: AppRouteName[] = ['dashboard', 'nodes', 'templates', 'ruleSets', 'profiles', 'settings']
  if (includeAdmin) names.push('users')
  for (const name of names) await appRouteLoaders[name]()
}

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
        { path: '', name: 'dashboard', component: appRouteLoaders.dashboard },
        { path: 'nodes', name: 'nodes', component: appRouteLoaders.nodes },
        { path: 'templates', name: 'templates', component: appRouteLoaders.templates },
        { path: 'rules', name: 'ruleSets', component: appRouteLoaders.ruleSets },
        { path: 'subscriptions', name: 'profiles', component: appRouteLoaders.profiles },
        { path: 'profiles', redirect: { name: 'profiles' } },
        { path: 'preview', redirect: { name: 'profiles' } },
        { path: 'users', name: 'users', component: appRouteLoaders.users, meta: { admin: true } },
        { path: 'settings', name: 'settings', component: appRouteLoaders.settings },
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
