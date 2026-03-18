<script setup lang="ts">
import type { DropdownMenuItemProps } from "reka-ui"
import { DropdownMenuItem, useForwardProps } from "reka-ui"
import { computed } from "vue"
import { cn } from "@/lib/utils"

type Props = DropdownMenuItemProps & { class?: string; inset?: boolean }
const props = defineProps<Props>()

const forwarded = useForwardProps(props)

const itemClass = computed(() =>
  cn(
    "relative flex cursor-default select-none items-center rounded-md px-2.5 py-1.5 text-sm outline-none transition-colors",
    "data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
    props.inset ? "pl-8" : "",
    props.class,
  ),
)
</script>

<template>
  <DropdownMenuItem v-bind="forwarded" :class="itemClass">
    <slot />
  </DropdownMenuItem>
</template>
