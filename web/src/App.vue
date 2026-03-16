<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppSidebar from '@/components/own/AppSidebar.vue'

const route = useRoute()

const hideSidebar = computed(
    () => route.meta.hideSidebar === true
)
</script>

<template>
  <div class="flex min-h-screen">
    <AppSidebar v-if="!hideSidebar">
      <main class="p-4 flex-1">
        <RouterView />
      </main>
    </AppSidebar>

    <main v-else class="flex-1">
      <RouterView />
    </main>
  </div>
</template>
