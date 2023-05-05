import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react-swc';

export default defineConfig({
  build: {
    sourcemap: true,
    outDir: 'build',
  },
  plugins: [react()],
});
