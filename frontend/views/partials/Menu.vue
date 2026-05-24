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
        <!-- Folder switcher: visible only for multi-folder users. -->
        <b-dropdown
          v-if="hasMultipleFolders"
          aria-role="menu"
          class="navbar-item folder-switcher"
          :disabled="switching"
        >
          <a slot="trigger" role="button" class="folder-switcher-trigger">
            <b-icon icon="folder-open" size="is-small" />
            <span style="margin: 0 0.4em">
              {{ activeFolderLabel }}
            </span>
            <b-icon icon="caret-down" size="is-small" />
          </a>

          <b-dropdown-item custom>
            <small>{{ lang('Switch folder') }}</small>
          </b-dropdown-item>
          <b-dropdown-item
            v-for="path in $store.state.user.homedirs"
            :key="path"
            :class="{ 'is-active': path === $store.state.user.active_homedir }"
            @click="switchFolder(path)"
          >
            <code style="font-size: 0.9em">{{ path }}</code>
          </b-dropdown-item>
        </b-dropdown>
        <a v-if="is('guest')" class="navbar-item login" @click="login">
          {{ lang('Login') }}
        </a>
        <a v-if="!is('guest')" class="navbar-item profile" @click="$router.push('/security').catch(() => {})">
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
import api from '../../api/api'

export default {
  name: 'Menu',
  data() {
    return {
      navbarActive: false,
      switching: false,
    }
  },
  computed: {
    hasMultipleFolders() {
      const u = this.$store.state.user
      return u && Array.isArray(u.homedirs) && u.homedirs.length > 1
    },
    activeFolderLabel() {
      return this.$store.state.user.active_homedir || this.lang('Folder')
    },
  },
  methods: {
    switchFolder(path) {
      if (this.switching) return
      if (path === this.$store.state.user.active_homedir) return
      this.switching = true
      api.selectFolder({ homedir: path })
        .then(() => {
          this.$store.commit('setActiveHomedir', path)
          this.$store.commit('resetCwd')
          // Refresh the file view. If the user is already on '/', the
          // existing $route watcher in Browser.vue picks up the cd query
          // change; if not, we push to '/'.
          if (this.$route.path !== '/') {
            this.$router.push('/').catch(() => {})
          } else {
            // Force the browser to re-list at the new folder root.
            api.changeDir({ to: '/' })
              .then(content => this.$store.commit('setCwd', content))
              .catch(error => this.handleError(error))
          }
        })
        .catch(error => this.handleError(error))
        .finally(() => {
          this.switching = false
        })
    },
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
  }
}
</script>

<style scoped>
.navbar {
  z-index: 10;
}
.folder-switcher-trigger {
  display: inline-flex;
  align-items: center;
  cursor: pointer;
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
