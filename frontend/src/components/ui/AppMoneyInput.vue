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

// Поле для денежной суммы. Введённое показывается группами по три цифры —
// 1 234 567,89, — а значение остаётся числом.
const props = withDefaults(defineProps<{
  modelValue: number
  placeholder?: string
}>(), {
  placeholder: '0',
})

const emit = defineEmits<{ (e: 'update:modelValue', value: number): void }>()

const el = ref<HTMLInputElement | null>(null)

// groups разбивает целую часть числа на группы по три цифры
function groups(whole: string): string {
  return whole.replace(/\B(?=(\d{3})+(?!\d))/g, ' ')
}

// display записывает число так, как его показывает поле; ноль — пустое поле
function display(value: number): string {
  if (!value) return ''
  const [whole, fraction] = (Math.round(value * 100) / 100).toString().split('.')
  return groups(whole) + (fraction ? ',' + fraction : '')
}

const text = ref(display(props.modelValue))

// Значение могут задать извне (копируемый счёт); пока пользователь печатает,
// поле остаётся таким, как он его набрал
watch(() => props.modelValue, (value) => {
  if (parse(text.value) !== value) text.value = display(value)
})

function parse(shown: string): number {
  return Number(shown.replace(/\s/g, '').replace(',', '.')) || 0
}

// clean оставляет от набранного текста то, из чего может состоять сумма:
// цифры и один десятичный разделитель с не более чем двумя цифрами после него
function clean(raw: string): string {
  const [head, ...tail] = raw.replace(/[^\d.,]/g, '').split(/[.,]/)
  const whole = head.replace(/^0+(?=\d)/, '')
  if (tail.length === 0) return groups(whole)
  return groups(whole || '0') + ',' + tail.join('').slice(0, 2)
}

// Символы, которые остаются после форматирования: по их числу курсор
// возвращается туда, где пользователь печатал
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

// При уходе из поля набранное приводится в порядок: "12," становится "12"
function onBlur() {
  text.value = display(parse(text.value))
}
</script>
