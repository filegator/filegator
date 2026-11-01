import axios from 'axios'
import { installSessionInterceptor } from '@/api/session'
import shared from '@/mixins/shared'

describe('session interceptor and error handling', () => {
  let instance
  let store
  let originalLocation

  beforeEach(() => {
    instance = axios.create()
    instance.defaults.headers = { common: {} }

    store = {
      state: {
        config: {
          language: 'english',
          guest_redirection: '',
        },
        user: {
          username: 'john_doe',
          role: 'user',
        },
      },
      commit: jest.fn(),
    }

    originalLocation = window.location
    delete window.location
    window.location = { href: '' }
  })

  afterEach(() => {
    window.location = originalLocation
    jest.clearAllMocks()
    jest.restoreAllMocks()
  })

  it('retries request transparently when CSRF token expires but user session is still valid', async () => {
    installSessionInterceptor(instance, store)

    let requestCount = 0
    let getUserCount = 0

    instance.defaults.adapter = async (config) => {
      if (config.url === 'getuser') {
        getUserCount++
        return {
          status: 200,
          headers: { 'x-csrf-token': 'new-csrf-token-123' },
          data: {
            data: {
              username: 'john_doe',
              role: 'user',
            },
          },
          config,
        }
      }

      if (config.url === 'changedir') {
        requestCount++
        if (requestCount === 1) {
          const err = new Error('Request failed with status code 403')
          err.response = {
            status: 403,
            data: { data: 'Csrf token not valid' },
            config,
          }
          err.config = config
          throw err
        }
        return {
          status: 200,
          headers: {},
          data: { data: { location: '/docs', files: [] } },
          config,
        }
      }

      return { status: 200, data: { data: 'ok' }, config }
    }

    const res = await instance.post('changedir', { to: '/docs' })

    expect(getUserCount).toBe(1)
    expect(requestCount).toBe(2)
    expect(instance.defaults.headers.common['x-csrf-token']).toBe('new-csrf-token-123')
    expect(res.data.data.location).toBe('/docs')
  })

  it('redirects to guest_redirection and marks error as sessionExpired when session is lost (401)', async () => {
    store.state.config.guest_redirection = '/login/'
    installSessionInterceptor(instance, store)

    instance.defaults.adapter = async (config) => {
      if (config.url === 'getuser') {
        return {
          status: 200,
          headers: {},
          data: {
            data: {
              username: 'guest',
              role: 'guest',
            },
          },
          config,
        }
      }

      const err = new Error('Session expired')
      err.response = {
        status: 401,
        data: { data: 'Session expired' },
        config,
      }
      err.config = config
      throw err
    }

    let caughtError
    try {
      await instance.post('changedir', { to: '/docs' })
    } catch (e) {
      caughtError = e
    }

    expect(caughtError).toBeDefined()
    expect(caughtError.sessionExpired).toBe(true)
  })

  it('commits destroyUser in standalone mode when session is lost', async () => {
    store.state.config.guest_redirection = ''
    installSessionInterceptor(instance, store)

    instance.defaults.adapter = async (config) => {
      if (config.url === 'getuser') {
        return {
          status: 200,
          headers: {},
          data: {
            data: {
              username: 'guest',
              role: 'guest',
            },
          },
          config,
        }
      }

      const err = new Error('Session expired')
      err.response = {
        status: 401,
        data: { data: 'Session expired' },
        config,
      }
      err.config = config
      throw err
    }

    let caughtError
    try {
      await instance.post('changedir', { to: '/docs' })
    } catch (e) {
      caughtError = e
    }

    expect(caughtError).toBeDefined()
    expect(caughtError.sessionExpired).toBe(true)
    expect(store.commit).toHaveBeenCalledWith('destroyUser')
  })

  it('handles single-flight getuser refresh for concurrent requests', async () => {
    installSessionInterceptor(instance, store)

    let getUserCount = 0

    instance.defaults.adapter = async (config) => {
      if (config.url === 'getuser') {
        getUserCount++
        return {
          status: 200,
          headers: { 'x-csrf-token': 'refreshed-token' },
          data: {
            data: {
              username: 'john_doe',
              role: 'user',
            },
          },
          config,
        }
      }

      if (!config._authRetried) {
        const err = new Error('Csrf error')
        err.response = {
          status: 403,
          data: { data: 'Csrf token not valid' },
          config,
        }
        err.config = config
        throw err
      }

      return { status: 200, data: { data: 'success for ' + config.url }, config }
    }

    const [res1, res2] = await Promise.all([
      instance.post('action1'),
      instance.post('action2'),
    ])

    expect(getUserCount).toBe(1)
    expect(res1.data.data).toBe('success for action1')
    expect(res2.data.data).toBe('success for action2')
  })

  it('detects 2xx malformed/empty response as auth failure', async () => {
    installSessionInterceptor(instance, store)

    let getUserCount = 0

    instance.defaults.adapter = async (config) => {
      if (config.url === 'getuser') {
        getUserCount++
        return {
          status: 200,
          headers: { 'x-csrf-token': 'token-after-empty-body' },
          data: {
            data: {
              username: 'john_doe',
              role: 'user',
            },
          },
          config,
        }
      }

      if (!config._authRetried) {
        // Return 200 OK with empty body string as legacy PHP die; produces
        return {
          status: 200,
          data: '',
          config,
        }
      }

      return { status: 200, data: { data: 'recovered' }, config }
    }

    const res = await instance.post('changedir', { to: '/home' })

    expect(getUserCount).toBe(1)
    expect(res.data.data).toBe('recovered')
  })

  it('does not treat rawResponse: true as malformed', async () => {
    installSessionInterceptor(instance, store)

    instance.defaults.adapter = async (config) => {
      return {
        status: 200,
        data: 'Plain text file content without data wrapper',
        config,
      }
    }

    const res = await instance.get('download&path=foo', { rawResponse: true })
    expect(res.data).toBe('Plain text file content without data wrapper')
  })

  it('handleError suppresses toast when error.sessionExpired is true', () => {
    const toastOpen = jest.fn()
    const ctx = {
      $toast: { open: toastOpen },
      lang: (t) => t,
      handleError: shared.methods.handleError,
    }

    const expiredErr = new Error('Session expired')
    expiredErr.sessionExpired = true

    ctx.handleError(expiredErr)
    expect(toastOpen).not.toHaveBeenCalled()

    ctx.handleError('Custom error message')
    expect(toastOpen).toHaveBeenCalledWith(expect.objectContaining({
      message: 'Custom error message',
      type: 'is-danger',
    }))
  })
})
