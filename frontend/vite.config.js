import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  build: {
    // Inline les images (logo) en base64 dans le bundle JS → pas de dépendance
    // au serveur d'assets de la webview (corrige le logo cassé).
    assetsInlineLimit: 200000,
  },
})
