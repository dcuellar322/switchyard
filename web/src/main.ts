import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { createApp } from 'vue'

import App from './app/App.vue'
import { bootstrapBrowserSession } from './domains/session/bootstrap'
import './styles/base.css'

async function start() {
  await bootstrapBrowserSession().catch(() => undefined)
  const app = createApp(App)
  app.use(VueQueryPlugin, {
    queryClient: new QueryClient({
      defaultOptions: {
        queries: {
          refetchOnWindowFocus: false,
          retry: 1,
          staleTime: 10_000,
        },
      },
    }),
  })
  app.mount('#app')
}

void start()
