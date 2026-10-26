import { ToastProgrammatic as Toast } from 'buefy'
import shared from '../mixins/shared'

let refreshPromise = null
let isRedirecting = false

function showPleaseLogInToast() {
  try {
    Toast.open({
      message: shared.methods.lang('Please log in'),
      type: 'is-danger',
      duration: 3000,
    })
  } catch (e) {
    // fallback if buefy toast cannot be invoked
  }
}

function handleSessionExpired(store) {
  const guestRedirection = store && store.state && store.state.config && store.state.config.guest_redirection
  if (guestRedirection) {
    if (!isRedirecting && typeof window !== 'undefined' && window.location) {
      isRedirecting = true
      showPleaseLogInToast()
      setTimeout(() => {
        window.location.href = guestRedirection
      }, 1000)
    }
  } else {
    showPleaseLogInToast()
    if (store && store.commit) {
      store.commit('destroyUser')
    }
  }
  const err = new Error('Session expired')
  err.sessionExpired = true
  return Promise.reject(err)
}

function getRefreshPromise(axiosInstance) {
  if (!refreshPromise) {
    refreshPromise = axiosInstance.get('getuser', { _isAuthRefresh: true })
      .then(res => {
        if (res && res.headers && res.headers['x-csrf-token']) {
          axiosInstance.defaults.headers.common['x-csrf-token'] = res.headers['x-csrf-token']
        }
        return res && res.data && res.data.data ? res.data.data : (res ? res.data : null)
      })
      .finally(() => {
        refreshPromise = null
      })
  }
  return refreshPromise
}

function isAuthError(error) {
  if (!error || !error.response) return false
  const status = error.response.status
  if (status === 401) return true
  if (status === 403) {
    const data = error.response.data
    if (data === 'Csrf token not valid' || (data && data.data === 'Csrf token not valid')) {
      return true
    }
  }
  return false
}

function isMalformedSuccess(response) {
  if (!response) return false
  const config = response.config || {}
  if (config.rawResponse || config._isAuthRefresh) return false
  const url = config.url || ''
  if (url.includes('getuser') || url.includes('getconfig')) return false

  const data = response.data
  if (typeof data !== 'object' || data === null || !('data' in data)) {
    return true
  }
  return false
}

function handleAuthFailure(axiosInstance, store, config) {
  if (!config || config._authRetried || config._isAuthRefresh) {
    return handleSessionExpired(store)
  }

  const url = config.url || ''
  if (url.includes('getuser') || url.includes('getconfig') || url.includes('login') || url.includes('logout')) {
    return handleSessionExpired(store)
  }

  return getRefreshPromise(axiosInstance)
    .then(user => {
      const currentUser = store && store.state && store.state.user
      const currentUsername = currentUser ? currentUser.username : ''

      if (user && user.role !== 'guest' && user.username && user.username === currentUsername) {
        config._authRetried = true
        config.headers = config.headers || {}
        if (axiosInstance.defaults.headers.common['x-csrf-token']) {
          config.headers['x-csrf-token'] = axiosInstance.defaults.headers.common['x-csrf-token']
        }
        return axiosInstance(config)
      } else {
        return handleSessionExpired(store)
      }
    })
    .catch(() => {
      return handleSessionExpired(store)
    })
}

export function installSessionInterceptor(axiosInstance, store) {
  axiosInstance.interceptors.response.use(
    response => {
      if (isMalformedSuccess(response)) {
        return handleAuthFailure(axiosInstance, store, response.config)
      }
      return response
    },
    error => {
      if (isAuthError(error)) {
        return handleAuthFailure(axiosInstance, store, error.config)
      }
      return Promise.reject(error)
    }
  )
}

export default installSessionInterceptor
