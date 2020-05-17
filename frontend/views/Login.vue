<template>
  <div v-if="!$store.state.config.guest_redirection">
    <a v-if="can('read')" id="back-arrow" @click="$router.push('/').catch(() => {})">
      <b-icon icon="times" />
    </a>

    <div id="login" class="columns is-centered">
      <div class="column is-narrow">
        <form @submit.prevent="login">
          <div class="box">
            <div class="has-text-centered">
              <img :src="$store.state.config.logo" class="logo">
            </div>
            <br>
            <b-field :label="lang('Username')">
              <b-input v-model="username" name="username" required @input="error = ''" ref="username" />
            </b-field>
            <b-field :label="lang('Password')">
              <b-input v-model="password" type="password" name="password" required @input="error = ''" password-reveal />
            </b-field>

            <div class="is-flex is-justify-end">
              <button class="button is-primary">
                {{ lang('Login') }}
              </button>
            </div>

            <div v-if="error">
              <code>{{ error }}</code>
            </div>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script>
import api from '../api/api'

export default {
  name: 'Login',
  data() {
    return {
      username: '',
      password: '',
      error: '',
    }
  },
  mounted() {
    if (this.$store.state.config.guest_redirection) {
      window.location.href = this.$store.state.config.guest_redirection
      return
    }
    this.$refs.username.focus()
  },
  methods: {
    login() {
      api.login({
        username: this.username,
        password: this.password,
      })
        .then(user => {
          this.$store.commit('setUser', user)
          api.changeDir({
            to: '/'
          }).then(() => this.$router.push('/').catch(() => {}))
        })
        .catch(error => {
          if (error.response && error.response.data) {
            this.error = this.lang(error.response.data.data)
          } else {
            this.handleError(error)
          }
          this.password = ''
        })
    },
  }
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

#login {
  padding: 120px 20px;
}
#back-arrow {
  position: fixed;
  top: 0;
  right: 0;
  margin: 20px;
}
</style>
