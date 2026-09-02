// Vite 配置：移动端构建目标；生产构建产物由 server 通过 embed 内嵌
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  base: './',
})
