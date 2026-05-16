<template>
  <div>
    <div id="login" class="columns is-centered">
      <div class="column is-narrow">
        <div class="box" style="max-width: 480px">
          <div class="has-text-centered">
            <img :src="$store.state.config.logo" class="logo">
          </div>

          <div v-if="state === 'validating'" class="has-text-centered" style="margin-top: 1em">
            <p>{{ lang('Validating link…') }}</p>
          </div>

          <div v-else-if="state === 'invalid'">
            <h3 class="is-size-5" style="margin: 1em 0">{{ lang('Link not valid') }}</h3>
            <p>{{ lang('This password reset link is invalid or has expired. Please request a new one.') }}</p>
            <br>
            <button class="button is-primary" @click="$router.push('/forgot-password').catch(() => {})">
              {{ lang('Request new link') }}
            </button>
          </div>

          <form v-else-if="state === 'form'" @submit.prevent="submit">
            <h3 class="is-size-5" style="margin: 1em 0">{{ lang('Choose a new password') }}</h3>
            <b-field :label="lang('New password')" :type="error ? 'is-danger' : ''" :message="error">
              <b-input v-model="newPassword" type="password" required password-reveal />
            </b-field>
            <b-field :label="lang('Confirm password')">
              <b-input v-model="confirm" type="password" required password-reveal />
            </b-field>
            <div class="is-flex is-justify-content-end">
              <button class="button is-primary">{{ lang('Update password') }}</button>
            </div>
          </form>

          <div v-else-if="state === 'done'">
            <h3 class="is-size-5" style="margin: 1em 0">{{ lang('Password updated') }}</h3>
            <p>{{ lang('You can now sign in with your new password.') }}</p>
            <br>
            <button class="button is-primary" @click="$router.push('/login').catch(() => {})">
              {{ lang('Go to login') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import api from '../api/api'

export default {
  name: 'ResetPassword',
  data() {
    return {
      state: 'validating',
      token: '',
      newPassword: '',
      confirm: '',
      error: '',
    }
  },
  mounted() {
    this.token = this.$route.query.token || ''
    if (!this.token) {
      this.state = 'invalid'
      return
    }
    api.validateResetToken(this.token)
      .then(data => {
        this.state = data && data.valid ? 'form' : 'invalid'
      })
      .catch(() => {
        this.state = 'invalid'
      })
  },
  methods: {
    submit() {
      this.error = ''
      if (this.newPassword.length < 8) {
        this.error = this.lang('Password must be at least 8 characters')
        return
      }
      if (this.newPassword !== this.confirm) {
        this.error = this.lang('Passwords do not match')
        return
      }
      api.confirmPasswordReset({ token: this.token, newPassword: this.newPassword })
        .then(() => { this.state = 'done' })
        .catch(error => {
          if (error.response && error.response.data && error.response.data.data) {
            const d = error.response.data.data
            this.error = this.lang(typeof d === 'string' ? d : (d.new_password || d.message || 'Could not reset password'))
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
  width: 260px;
  display: inline-block;
}
.box {
  padding: 30px;
}
#login {
  padding: 120px 20px;
}
</style>
