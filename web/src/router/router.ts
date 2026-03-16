import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/Home.vue'
import Search from '@/views/Search.vue'
import Auth from '@/views/Auth.vue'
import D4S from '@/views/D4S.vue'
import AdminSettings from '@/views/AdminSettings.vue'
import AdminIntegrations from '@/views/AdminIntegrations.vue'
import AdminStatistics from '@/views/AdminStatistics.vue'
import AdminInvites from '@/views/AdminInvites.vue'
import { refreshAccessToken } from '@/auth/refresh'
import { accessToken, setAccessToken } from '@/auth/auth'
import { ensureCurrentUser } from '@/auth/ensure_user'
import { setCurrentUser } from '@/auth/user'
import { clearAdminCache } from '@/lib/admin'
import { checkIsAdmin } from '@/lib/admin'
import TaskView from '@/views/TaskView.vue'
import TasksList from '@/views/TasksList.vue'
import UserSettings from '@/views/UserSettings.vue'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'Home',
    component: HomeView,
    meta: { requiresAuth: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/search',
    name: 'Search',
    component: Search,
    meta: { requiresAuth: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: UserSettings,
    meta: { requiresAuth: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/pdf/:id',
    name: 'PDF',
    component: () => import('@/views/PDFReader.vue'),
    meta: { requiresAuth: true, hideSidebar: false, forceSidebarClosed: true },
  },
  {
    path: '/d4s',
    name: 'D4S',
    component: D4S,
    meta: { requiresAuth: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/admin',
    name: 'Admin',
    redirect: '/admin/settings',
    meta: { requiresAuth: true, requiresAdmin: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: AdminSettings,
    meta: { requiresAuth: true, requiresAdmin: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/admin/integrations',
    name: 'AdminIntegrations',
    component: AdminIntegrations,
    meta: { requiresAuth: true, requiresAdmin: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/admin/statistics',
    name: 'AdminStatistics',
    component: AdminStatistics,
    meta: { requiresAuth: true, requiresAdmin: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/admin/invites',
    name: 'AdminInvites',
    component: AdminInvites,
    meta: { requiresAuth: true, requiresAdmin: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/auth',
    name: 'Auth',
    component: Auth,
    meta: { requiresAuth: false, hideSidebar: true, forceSidebarClosed: true },
  },
  {
    path: '/admin/tasks',
    name: 'TaskList',
    component: TasksList,
    meta: { requiresAuth: true, requiresAdmin: true, hideSidebar: false, forceSidebarClosed: false },
  },
  {
    path: '/admin/task/:id',
    name: 'TaskView',
    component: TaskView,
    meta: { requiresAuth: true, requiresAdmin: true, hideSidebar: false, forceSidebarClosed: false },
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

let authCheckInflight: Promise<void> | null = null

async function ensureAuthenticatedRouteAccess() {
  if (authCheckInflight) return authCheckInflight

  authCheckInflight = (async () => {
    if (!accessToken.value) {
      await refreshAccessToken()
    }
    await ensureCurrentUser()
  })().finally(() => {
    authCheckInflight = null
  })

  return authCheckInflight
}

router.beforeEach(async (to) => {
  if (to.meta.requiresAuth === false) {
    return true
  }

  try {
    await ensureAuthenticatedRouteAccess()
    if (to.meta.requiresAdmin) {
      const isAdmin = await checkIsAdmin()
      if (!isAdmin) return { name: 'Home' }
    }
    return true
  } catch {
    setAccessToken(null)
    setCurrentUser(null)
    clearAdminCache()
    return { name: 'Auth' }
  }
})

export default router
