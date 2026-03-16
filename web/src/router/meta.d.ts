import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    requiresAdmin?: boolean
    hideSidebar?: boolean
    forceSidebarClosed?: boolean
  }
}
