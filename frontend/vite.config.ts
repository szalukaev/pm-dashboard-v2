import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { quasar, transformAssetUrls } from '@quasar/vite-plugin'

// Номер сборки: вшивается в код и кладётся в version.json. Открытое приложение
// сверяет их и предлагает обновиться, когда на сервере уже другая сборка
const BUILD_ID = String(Date.now())

export default defineConfig({
  plugins: [
    vue({ template: transformAssetUrls }),
    quasar(),
    {
      name: 'version-file',
      generateBundle() {
        this.emitFile({ type: 'asset', fileName: 'version.json', source: JSON.stringify({ build: BUILD_ID }) })
      }
    }
  ],
  define: {
    __BUILD_ID__: JSON.stringify(BUILD_ID)
  },
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true }
    }
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      output: {
        assetFileNames: `assets/[name]-${Date.now()}[extname]`
      }
    }
  }
})
