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
        api.getUser()
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
  render: h => h(App),
}).$mount('#app')
