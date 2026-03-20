<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button
        variant="outline"
        size="sm"
        class="h-9 w-full justify-between px-2"
        :disabled="disabled"
      >
        <span class="flex items-center gap-2">
          <span
            class="h-4 w-4 rounded-full border border-neutral-300 shadow-sm dark:border-neutral-700"
            :style="{ backgroundColor: modelValue }"
          />
          <span class="font-mono text-xs uppercase">{{ modelValue }}</span>
        </span>
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" class="w-56 space-y-3 p-3">
      <div class="grid grid-cols-5 gap-2">
        <button
          v-for="color in palette"
          :key="color"
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-full border border-neutral-200 transition-transform hover:scale-105 dark:border-neutral-800"
          :class="modelValue.toLowerCase() === color.toLowerCase() ? 'ring-2 ring-neutral-900 ring-offset-2 ring-offset-white dark:ring-neutral-100 dark:ring-offset-neutral-950' : ''"
          :style="{ backgroundColor: color }"
          @click="setColor(color)"
        >
          <Check
            v-if="modelValue.toLowerCase() === color.toLowerCase()"
            class="h-3.5 w-3.5"
            :class="isLightColor(color) ? 'text-neutral-900' : 'text-white'"
          />
        </button>
      </div>
      <div class="space-y-1.5">
        <div class="text-[11px] font-medium uppercase tracking-[0.16em] text-neutral-500 dark:text-neutral-400">
          Custom hex
        </div>
        <Input
          :model-value="draftValue"
          class="font-mono text-xs uppercase"
          placeholder="#111827"
          @update:model-value="updateDraft(String($event))"
        />
      </div>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { Check } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'

const props = withDefaults(defineProps<{
  modelValue: string
  disabled?: boolean
}>(), {
  disabled: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const palette = [
  '#111827',
  '#374151',
  '#6b7280',
  '#dc2626',
  '#ea580c',
  '#ca8a04',
  '#16a34a',
  '#0891b2',
  '#2563eb',
  '#7c3aed',
]

const draftValue = ref(props.modelValue)

watch(
  () => props.modelValue,
  (value) => {
    draftValue.value = value
  },
)

function setColor(value: string) {
  emit('update:modelValue', value)
}

function updateDraft(value: string) {
  draftValue.value = value
  if (/^#([0-9a-fA-F]{6})$/.test(value)) {
    emit('update:modelValue', value)
  }
}

function isLightColor(hex: string) {
  const normalized = hex.replace('#', '')
  const red = Number.parseInt(normalized.slice(0, 2), 16)
  const green = Number.parseInt(normalized.slice(2, 4), 16)
  const blue = Number.parseInt(normalized.slice(4, 6), 16)
  const luminance = (red * 299 + green * 587 + blue * 114) / 1000
  return luminance > 160
}
</script>
