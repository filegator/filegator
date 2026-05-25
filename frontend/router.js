import Vue from 'vue'
import Router from 'vue-router'
import Browser from './views/Browser.vue'
import Users from './views/Users.vue'
import Login from './views/Login.vue'
import Security from './views/Security.vue'
import ForgotPassword from './views/ForgotPassword.vue'
import ResetPassword from './views/ResetPassword.vue'
import SelectFolder from './views/SelectFolder.vue'
import store from './store'
import { needsFolderPicker } from './mixins/postLogin'

Vue.use(Router)

const router = new Router({
  mode: 'hash',
  routes: [
    {
      path: '/',
      name: 'browser',
      component: Browser,
    },
    {
      path: '/select-folder',
      name: 'select-folder',
      component: SelectFolder,
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

// Multi-folder guard: a user with more than one folder and no active
// selection must pick before entering the browser view. Prevents the
// deep-link case where someone bookmarks `/` and lands there directly
// without going through routeAfterLogin's branch.
router.beforeEach((to, from, next) => {
  if (to.name !== 'browser') {
    next()
    return
  }
  const user = store.state.user
  if (!user || user.role === 'guest') {
    next()
    return
  }
  if (needsFolderPicker(user)) {
    next('/select-folder')
    return
  }
  next()
})

export default router
