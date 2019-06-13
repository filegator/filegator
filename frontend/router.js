import Vue from 'vue'
import Router from 'vue-router'
import Browser from './views/Browser.vue'
import Users from './views/Users.vue'
import Login from './views/Login.vue'
import store from './store'

Vue.use(Router)

export default new Router({
  mode: 'hash',
  routes: [
    {
      path: '/',
      name: 'browser',
      component: Browser,
    },
    {
      path: '/login',
      name: 'login',
      component: Login,
    },
    {
      path: '/users',
      name: 'users',
      component: Users,
      beforeEnter: (to, from, next) => {
        if (store.state.user.role == 'admin') {
          next()
        }
      },
    },
  ]
})
