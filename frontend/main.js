import Vue from 'vue'
import App from './App.vue'
import router from './router'
import store from './store'
import Buefy from 'buefy'
import shared from './mixins/shared'
import axios from 'axios'
import api from './api/api'
import VueLazyload from 'vue-lazyload'
import '@fortawesome/fontawesome-free/css/all.css'
import '@fortawesome/fontawesome-free/css/fontawesome.css'

//TODO: import './registerServiceWorker'

Vue.config.productionTip = false

/* eslint-disable-next-line */
Vue.config.baseURL = process.env.VUE_APP_API_ENDPOINT ? process.env.VUE_APP_API_ENDPOINT : window.location.origin+window.location.pathname+'?r='

axios.defaults.withCredentials = true
axios.defaults.baseURL = Vue.config.baseURL

axios.defaults.headers['Content-Type'] = 'application/json'

Vue.use(Buefy, {
  defaultIconPack: 'fas',
})

Vue.use(VueLazyload, {
  preLoad: 1.3,
})


Vue.mixin(shared)

new Vue({
  router,
  store,
  created: function() {

    api.getConfig()
        .then(ret => {
          this.$store.commit('setConfig', ret.data.data)

          // Check for SID query parameter for auto-login
          const urlParams = new URLSearchParams(window.location.search)
          const sid = urlParams.get('sid')
          const doc = urlParams.get('doc')

          // Log doc parameter if present
          if (doc) {
            console.log('Doc parameter:', doc)
          }

          if (sid) {
            // First get CSRF token via getUser, then attempt SID login
            return api.getUser()
                .then(() => {
                  // CSRF token now set, attempt SID-based login
                  return api.login({
                    username: '__sid__',
                    password: sid
                  })
                })
                .then(() => {
                  // SID login successful, clean up URL
                  const url = new URL(window.location)
                  url.searchParams.delete('sid')
                  window.history.replaceState({}, document.title, url.pathname + url.search)

                  // Handle doc parameter if present
                  return this.handleDocParameter(doc)
                })
                .then(() => {
                  // Get updated user after potential directory change
                  return api.getUser()
                })
                .catch(() => {
                  // SID login failed or initial getUser failed, clean up URL and continue
                  const url = new URL(window.location)
                  url.searchParams.delete('sid')
                  window.history.replaceState({}, document.title, url.pathname + url.search)
                  return api.getUser()
                })
          } else {
            // No SID parameter, proceed with normal flow
            return api.getUser()
                .then((user) => {
                  // Handle doc parameter if present and user is logged in
                  if (user && user.role !== 'guest') {
                    return this.handleDocParameter(doc).then(() => user)
                  }
                  return user
                })
          }
        })
        .then((user) => {
          this.$store.commit('initialize')
          this.$store.commit('setUser', user)
          this.$router.push('/').catch(() => {})
        })
        .catch(() => {
          this.$notification.open({
            message: this.lang('Something went wrong'),
            type: 'is-danger',
            queue: false,
            indefinite: true,
          })
        })
  },
  methods: {
    handleDocParameter(doc) {
      return new Promise((resolve) => {
        if (!doc) {
          resolve()
          return
        }

        try {
          // URL decode the doc parameter
          const decodedPath = decodeURIComponent(doc)
          console.log('Navigating to decoded path:', decodedPath)

          // Call the changeDir API
          api.changeDir({ to: decodedPath })
            .then(() => {
              console.log('Successfully navigated to:', decodedPath)
              // Clean up URL parameter
              const url = new URL(window.location)
              url.searchParams.delete('doc')
              window.history.replaceState({}, document.title, url.pathname + url.search)
              resolve()
            })
            .catch((error) => {
              console.log('Failed to navigate to path, backend should handle fallback:', error)
              // Clean up URL parameter even on failure
              const url = new URL(window.location)
              url.searchParams.delete('doc')
              window.history.replaceState({}, document.title, url.pathname + url.search)
              resolve()
            })
        } catch (error) {
          console.log('Failed to decode doc parameter, ignoring:', error)
          // Clean up URL parameter on decode failure
          const url = new URL(window.location)
          url.searchParams.delete('doc')
          window.history.replaceState({}, document.title, url.pathname + url.search)
          resolve()
        }
      })
    }
  },
  render: h => h(App),
}).$mount('#app')
