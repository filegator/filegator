<template>
  <nav class="navbar" role="navigation" aria-label="main navigation">
    <div class="navbar-brand">
      <a class="navbar-item logo" @click="$router.push('/')">
        <img :src="this.$store.state.config.logo">
      </a>

      <a @click="navbarActive = !navbarActive" role="button" :class="[navbarActive ? 'is-active' : '', 'navbar-burger burger']" aria-label="menu" aria-expanded="false">
        <span aria-hidden="true"></span>
        <span aria-hidden="true"></span>
        <span aria-hidden="true"></span>
      </a>
    </div>

    <div :class="[navbarActive ? 'is-active' : '', 'navbar-menu']">
      <div class="navbar-end">
        <a @click="$router.push('/')" v-if="is('admin')" class="navbar-item">
          {{ lang('Files') }}
        </a>
        <a @click="$router.push('/users')" v-if="is('admin')" class="navbar-item">
          {{ lang('Users') }}
        </a>
        <a @click="login" v-if="is('guest')" class="navbar-item">
          {{ lang('Login') }}
        </a>
        <a @click="profile" v-if="!is('guest')" class="navbar-item">
          {{ lang('Profile') }}
        </a>
        <a @click="logout" v-if="!is('guest')" class="navbar-item">
          {{ lang('Logout') }}
        </a>
      </div>
    </div>

  </nav>
</template>

<script>
import Profile from './Profile'
import api from '../../api/api'

export default {
  name: 'Menu',
  components: { Profile },
  data() {
    return {
      navbarActive: false,
    }
  },
  mounted() {
    if (this.$store.state.user.firstlogin) {
      this.profile()
    }
  },
  methods: {
    logout() {
      api.logout()
        .then(() => {
          this.$store.commit('initialize')
          api.getUser()
            .then(user => {
              this.$store.commit('setUser', user)
              this.$router.push('/login')
            })
            .catch(() => {
              this.$store.commit('initialize')
            })
        })
        .catch(error => {
          this.$store.commit('initialize')
          this.handleError(error)
        })
    },
    login() {
      this.$router.push('/login')
    },
    profile() {
      this.$modal.open({
        parent: this,
        hasModalCard: true,
        component: Profile,
      })
    },
  }
}
</script>

<style scoped>
.navbar {
  z-index: 10;
}
@media all and (max-width: 1088px) {
  .logo {
    padding: 0;
  }
  .logo img {
    max-height: 3rem;
  }
}
@media all and (min-width: 1088px) {
  .navbar {
    padding: 1rem 0;
  }
  .logo {
    padding: 0 0 0 12px;
  }
  .logo img {
    max-height: 2.5rem;
  }
}
</style>
