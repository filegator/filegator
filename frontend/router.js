import Vue from 'vue'
import Router from 'vue-router'
import Browser from './views/Browser.vue'
import Users from './views/Users.vue'
import Login from './views/Login.vue'
import Security from './views/Security.vue'
import ForgotPassword from './views/ForgotPassword.vue'
import ResetPassword from './views/ResetPassword.vue'
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
      path: '/forgot-password',
      name: 'forgot-password',
      component: ForgotPassword,
    },
    {
      path: '/reset-password',
      name: 'reset-password',
      component: ResetPassword,
    },
    {
      path: '/security',
      name: 'security',
      component: Security,
      beforeEnter: (to, from, next) => {
        if (store.state.user.role == 'user' || store.state.user.role == 'admin') {
          next()
        } else {
          next('/login')
        }
      },
    },
    {
      path: '/users',
      name: 'users',
      component: Users,
      beforeEnter: (to, from, next) => {
        if (store.state.user.role == 'admin') {
          next()
        } else {
          next('/')
        }
      },
    },
  ]
})
