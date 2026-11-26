import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 5173,
    strictPort: true,
    // The design tool writes atomic temp files into frontend/design and locks
    // them; watching that tree crashes the dev server with EBUSY on Windows.
    watch: null
  },
  preview: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': 'http://localhost:8080'
    }
  },
  build: {
    // The chart engine is one lazy vendor chunk (≈300 kB gzipped) that only a page
    // with a chart fetches. The limit is set above it on purpose, so this warning
    // keeps firing for application code, which is what we can still act on.
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        // The two big libraries get their own chunks so they cache separately
        // and are only fetched by the pages that draw a chart or read a PDF.
        manualChunks(id: string) {
          if (!id.includes('node_modules')) return;
          if (id.includes('apexcharts')) return 'charts';
          if (id.includes('pdfjs-dist')) return 'pdf';
          return 'vendor';
        }
      }
    }
  }
});
