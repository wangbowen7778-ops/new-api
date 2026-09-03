import { render } from '@testing-library/react'
import { test } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { Drawing } = await import('../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const apiClient = api as unknown as {
  get: (url: string) => Promise<{ data: unknown }>
}

apiClient.get = async (url: string) => {
  switch (url) {
    case '/api/user/self':
      return {
        data: { success: true, data: { quota: 100000, group: 'default' } },
      }
    case '/api/user/self/groups':
      return {
        data: {
          success: true,
          data: { default: { desc: 'Default', ratio: 1 } },
        },
      }
    case '/api/pricing':
      return { data: { success: true, data: [], group_ratio: {} } }
    default:
      throw new Error(`Unexpected GET ${url}`)
  }
}

test('renders drawing page without crashing', () => {
  const { container } = render(
    <I18nextProvider i18n={i18n}>
      <Drawing />
    </I18nextProvider>,
  )
  const html = container.innerHTML
  if (!html.includes('data-slot')) {
    throw new Error('drawing page did not render controls')
  }
})
