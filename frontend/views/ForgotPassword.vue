<template>
  <div id="login" class="columns is-centered">
    <div class="column is-narrow">
      <form @submit.prevent="submit" v-if="!sent">
        <div class="box" style="max-width: 480px">
          <div class="has-text-centered">
            <img :src="$store.state.config.logo" class="logo">
          </div>
          <h3 class="is-size-5" style="margin: 1em 0">
            {{ lang('Reset your password') }}
          </h3>
          <p>{{ lang('Enter the email address associated with your account. If it matches an address on file, we will send you a password reset link shortly.') }}</p>
          <p style="margin-top: 0.75em">{{ lang('If you do not receive the email within a few minutes, check your spam folder or contact us for help.') }}</p>
          <br>
          <b-field :label="lang('Email')">
            <b-input v-model="email" type="email" required ref="email" />
          </b-field>
          <div class="login-actions">
            <a @click="$router.push('/login').catch(() => {})" class="login-link">{{ lang('Back to login') }}</a>
            <button class="button is-primary">
              {{ lang('Send reset link') }}
            </button>
          </div>
        </div>
      </form>

      <div class="box" v-else style="max-width: 480px">
        <h3 class="is-size-5">
          {{ lang('Check your inbox') }}
        </h3>
        <p>{{ message }}</p>
        <br>
        <button class="button is-primary" @click="$router.push('/login').catch(() => {})">
          {{ lang('Back to login') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import api from '../api/api'

export default {
  name: 'ForgotPassword',
  data() {
    return {
      email: '',
      sent: false,
      message: '',
    }
  },
  mounted() {
    this.$refs.email && this.$refs.email.focus()
  },
  methods: {
    submit() {
      api.requestPasswordReset({ email: this.email })
        .then(data => {
          this.message = (data && data.message) || this.lang('If that email matches an account, a reset link has been sent.')
          this.sent = true
        })
        .catch(error => {
          if (error.response && error.response.status === 429) {
            this.$toast.open({ message: this.lang('Too many requests'), type: 'is-warning' })
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
#login {
  padding: 120px 20px;
}
.login-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem 1.5rem;
  margin-top: 0.75em;
}
.login-link {
  font-size: 0.9em;
  white-space: nowrap;
}
</style>
