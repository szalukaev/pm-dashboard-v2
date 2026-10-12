<template>
  <input
    ref="el"
    type="text"
    inputmode="decimal"
    autocomplete="off"
    :value="text"
    :placeholder="placeholder"
    @input="onInput"
    @blur="onBlur"
  />
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

// A field for an amount of money. What is typed is shown in groups of three
// digits — 1 234 567,89 — while the value stays a number.
const props = withDefaults(defineProps<{
  modelValue: number
  placeholder?: string
}>(), {
  placeholder: '0',
})

const emit = defineEmits<{ (e: 'update:modelValue', value: number): void }>()

const el = ref<HTMLInputElement | null>(null)

// groups splits the whole part of a number into groups of three digits
function groups(whole: string): string {
  return whole.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')
}

// display writes a number the way the field shows it; zero is an empty field
function display(value: number): string {
  if (!value) return ''
  const [whole, fraction] = (Math.round(value * 100) / 100).toString().split('.')
  return groups(whole) + (fraction ? ',' + fraction : '')
}

const text = ref(display(props.modelValue))

// The value may be set from outside (a copied invoice); while the user
// types, the field is left as they typed it
watch(() => props.modelValue, (value) => {
  if (parse(text.value) !== value) text.value = display(value)
})

function parse(shown: string): number {
  return Number(shown.replace(/\s/g, '').replace(',', '.')) || 0
}

// clean keeps of the typed text what an amount may consist of: digits and
// one decimal separator with up to two digits after it
function clean(raw: string): string {
  const [head, ...tail] = raw.replace(/[^\d.,]/g, '').split(/[.,]/)
  const whole = head.replace(/^0+(?=\d)/, '')
  if (tail.length === 0) return groups(whole)
  return groups(whole || '0') + ',' + tail.join('').slice(0, 2)
}

// The characters that survive formatting: by counting them the caret is
// put back where the user was typing
function meaningful(s: string): number {
  return s.replace(/[^\d,.]/g, '').length
}

function onInput() {
  const input = el.value
  if (!input) return
  const before = meaningful(input.value.slice(0, input.selectionStart ?? input.value.length))
  const formatted = clean(input.value)
  text.value = formatted
  input.value = formatted
  emit('update:modelValue', parse(formatted))

  nextTick(() => {
    let position = 0
    for (let seen = 0; position < formatted.length && seen < before; position++) {
      if (/[\d,]/.test(formatted[position])) seen++
    }
    input.setSelectionRange(position, position)
  })
}

// Leaving the field tidies what was typed: "12," becomes "12"
function onBlur() {
  text.value = display(parse(text.value))
}
</script>
