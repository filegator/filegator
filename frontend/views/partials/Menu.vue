<template>
  <nav class="navbar" role="navigation" aria-label="main navigation">
    <div class="navbar-brand">
      <a class="navbar-item logo" @click="$router.push('/').catch(() => {})">
        <img :src="this.$store.state.config.logo">
      </a>

      <a :class="[navbarActive ? 'is-active' : '', 'navbar-burger burger']" role="button" aria-label="menu" aria-expanded="false" @click="navbarActive = !navbarActive">
        <span aria-hidden="true" />
        <span aria-hidden="true" />
        <span aria-hidden="true" />
      </a>
    </div>

    <div :class="[navbarActive ? 'is-active' : '', 'navbar-menu']">
      <div class="navbar-end">
        <a v-if="is('admin')" class="navbar-item files" @click="$router.push('/').catch(() => {})">
          {{ lang('Files') }}
        </a>
        <a v-if="is('admin')" class="navbar-item users" @click="$router.push('/users').catch(() => {})">
          {{ lang('Users') }}
        </a>
        <a v-if="is('guest')" class="navbar-item login" @click="login">
          {{ lang('Login') }}
        </a>
        <a v-if="!is('guest')" class="navbar-item profile" @click="profile">
          {{ this.$store.state.user.name }}
        </a>
        <a v-if="!is('guest')" class="navbar-item logout" @click="logout">
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
              this.$router.push('/').catch(() => {})
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
      this.$router.push('/login').catch(() => {})
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
