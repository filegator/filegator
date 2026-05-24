<template>
  <div id="select-folder" class="columns is-centered">
    <div class="column is-narrow">
      <div class="box" style="max-width: 480px">
        <div class="has-text-centered">
          <img :src="$store.state.config.logo" class="logo">
        </div>
        <h3 class="is-size-5" style="margin: 1em 0">
          {{ lang('Select a folder') }}
        </h3>
        <p>{{ lang('Your account has access to multiple folders. Choose the one you want to open.') }}</p>
        <br>

        <div v-if="folders.length === 0">
          <p class="has-text-danger">
            {{ lang('No folders available') }}
          </p>
        </div>

        <div v-else>
          <button
            v-for="path in folders"
            :key="path"
            type="button"
            class="button is-fullwidth is-primary is-light folder-button"
            :disabled="busy"
            @click="pick(path)"
          >
            <span class="folder-path">{{ path }}</span>
          </button>
        </div>

        <div v-if="error" class="login-error">
          <code>{{ error }}</code>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import api from '../api/api'

export default {
  name: 'SelectFolder',
  data() {
    return {
      busy: false,
      error: '',
    }
  },
  computed: {
    folders() {
      const user = this.$store.state.user
      return (user && Array.isArray(user.homedirs)) ? user.homedirs : []
    },
  },
  mounted() {
    // Defensive guards: if the user landed here when they shouldn't have
    // (single-folder, or already has an active), bounce them to the
    // browser view.
    if (this.folders.length <= 1) {
      this.$router.push('/').catch(() => {})
      return
    }
    if (this.$store.state.user.active_homedir) {
      this.$router.push('/').catch(() => {})
    }
  },
  methods: {
    pick(path) {
      if (this.busy) return
      this.busy = true
      this.error = ''

      api.selectFolder({ homedir: path })
        .then(() => {
          this.$store.commit('setActiveHomedir', path)
          // Reset cwd before navigating into the browser so a stale
          // path from a previous session doesn't pre-fill.
          this.$store.commit('resetCwd')
          this.$router.push('/').catch(() => {})
        })
        .catch(error => {
          this.busy = false
          if (error && error.response && error.response.status === 422) {
            this.error = this.lang('Folder no longer available; please contact us.')
            // Refresh the user record — admin may have removed this folder.
            api.getUser().then(user => this.$store.commit('setUser', user)).catch(() => {})
          } else {
            this.handleError(error)
          }
        })
    },
  },
}
</script>

<style scoped>
.logo {
  width: 300px;
  display: inline-block;
}
.box {
  padding: 30px;
}
#select-folder {
  padding: 120px 20px;
}
.folder-button {
  margin-top: 0.5em;
  justify-content: flex-start;
  text-align: left;
}
.folder-path {
  font-family: monospace;
}
.login-error {
  margin-top: 0.75em;
  text-align: center;
}
</style>
