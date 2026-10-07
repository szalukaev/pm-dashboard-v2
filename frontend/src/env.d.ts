/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare module 'textile-js' {
  const textile: (source: string, options?: { breaks?: boolean }) => string
  export default textile
}
