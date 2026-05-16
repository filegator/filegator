<template>
  <div>
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
            <p>{{ lang('Enter your account email and we will send you a link to reset your password.') }}</p>
            <br>
            <b-field :label="lang('Email')">
              <b-input v-model="email" type="email" required ref="email" />
            </b-field>
            <div class="is-flex is-justify-content-space-between" style="align-items: center">
              <a @click="$router.push('/login').catch(() => {})">{{ lang('Back to login') }}</a>
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
</style>
