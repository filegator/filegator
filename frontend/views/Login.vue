<template>
  <div v-if="!$store.state.config.guest_redirection">
    <a v-if="can('read')" id="back-arrow" @click="$router.push('/').catch(() => {})">
      <b-icon icon="times" />
    </a>

    <div id="login" class="columns is-centered">
      <div class="column is-narrow">
        <!-- Step 1: username + password -->
        <form v-if="step === 'password'" @submit.prevent="login">
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

            <div class="login-actions">
              <a v-if="$store.state.config.password_reset_enabled" @click="$router.push('/forgot-password').catch(() => {})" class="login-link">
                {{ lang('Forgot password?') }}
              </a>
              <span v-else />
              <button class="button is-primary">
                {{ lang('Login') }}
              </button>
            </div>

            <div v-if="error" class="login-error">
              <code>{{ error }}</code>
            </div>
          </div>
        </form>

        <!-- Step 2: verify TOTP -->
        <form v-else-if="step === 'mfa'" @submit.prevent="verifyMfa">
          <div class="box">
            <div class="has-text-centered">
              <img :src="$store.state.config.logo" class="logo">
            </div>
            <br>
            <p>{{ useBackup ? lang('Enter one of your backup codes') : lang('Enter the 6-digit code from your authenticator app') }}</p>
            <br>
            <b-field>
              <b-input
                v-model="mfaCode"
                type="text"
                :placeholder="useBackup ? 'XXXXX-XXXXX' : '123456'"
                :style="useBackup ? 'font-family: monospace; font-size: 1.1em; letter-spacing: 0.05em; text-transform: uppercase' : 'font-family: monospace; font-size: 1.2em; letter-spacing: 0.15em'"
                required
                autocomplete="one-time-code"
                @input="onMfaInput"
                ref="mfa"
                key="mfa-input"
              />
            </b-field>
            <div class="login-actions">
              <a @click="toggleBackup" class="login-link">
                {{ useBackup ? lang('Use authenticator code') : lang('Use a backup code') }}
              </a>
              <div class="buttons" style="margin-bottom: 0">
                <button class="button" type="button" @click="cancel">
                  {{ lang('Cancel') }}
                </button>
                <button class="button is-primary">
                  {{ lang('Verify') }}
                </button>
              </div>
            </div>
            <div v-if="error" class="login-error">
              <code>{{ error }}</code>
            </div>
          </div>
        </form>

        <!-- Step 3: forced MFA setup (admins) -->
        <form v-else-if="step === 'mfa_setup'" @submit.prevent="completeSetup">
          <div class="box" style="max-width: 480px">
            <div class="has-text-centered">
              <img :src="$store.state.config.logo" class="logo">
            </div>
            <h3 class="is-size-5" style="margin: 1em 0">
              {{ lang('MFA setup required') }}
            </h3>
            <p>{{ lang('Your administrator account requires multi-factor authentication. Scan the QR code with an authenticator app (Google Authenticator, Authy, 1Password), then enter the 6-digit code shown.') }}</p>
            <br>
            <div class="has-text-centered">
              <canvas ref="qrCanvas" />
            </div>
            <p style="font-family: monospace; word-break: break-all; font-size: 0.9em; margin-top: 0.5em">
              {{ lang('Manual key') }}: {{ enrollment.secret }}
            </p>
            <br>
            <b-field :label="lang('6-digit code')">
              <b-input
                v-model="mfaCode"
                type="text"
                placeholder="123456"
                style="font-family: monospace; font-size: 1.2em; letter-spacing: 0.15em"
                required
                autocomplete="one-time-code"
                @input="error = ''"
                key="mfa-setup-input"
              />
            </b-field>
            <div class="buttons is-right" style="margin-top: 1.25em; margin-bottom: 0">
              <button class="button" type="button" @click="cancel">
                {{ lang('Cancel') }}
              </button>
              <button class="button is-primary">
                {{ lang('Verify and continue') }}
              </button>
            </div>
            <div v-if="error" class="login-error">
              <code>{{ error }}</code>
            </div>

            <div v-if="setupBackupCodes" class="notification is-warning" style="margin-top: 1em">
              <p><strong>{{ lang('Save these backup codes') }}</strong></p>
              <p>{{ lang('Each can be used once if you lose access to your authenticator. They will not be shown again.') }}</p>
              <ul style="font-family: monospace; margin-top: 0.5em">
                <li v-for="c in setupBackupCodes" :key="c">
                  {{ c }}
                </li>
              </ul>
              <div class="buttons is-right" style="margin-top: 1em; margin-bottom: 0">
                <button class="button is-primary" type="button" @click="finishSetup">
                  {{ lang('Continue') }}
                </button>
              </div>
            </div>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script>
import api from '../api/api'
import QRCode from 'qrcode'

export default {
  name: 'Login',
  data() {
    return {
      step: 'password',
      username: '',
      password: '',
      mfaCode: '',
      useBackup: false,
      enrollment: null,
      setupBackupCodes: null,
      pendingUser: null,
      error: '',
    }
  },
  mounted() {
    if (this.$store.state.config.guest_redirection) {
      window.location.href = this.$store.state.config.guest_redirection
      return
    }
    this.$refs.username && this.$refs.username.focus()
  },
  methods: {
    login() {
      api.login({
        username: this.username,
        password: this.password,
      })
        .then(data => {
          if (data && data.mfa_required) {
            this.step = 'mfa'
            this.$nextTick(() => this.$refs.mfa && this.$refs.mfa.focus())
            return
          }
          if (data && data.mfa_setup_required) {
            this.enrollment = data.enrollment
            this.step = 'mfa_setup'
            this.$nextTick(() => this.drawQr())
            return
          }
          this.$store.commit('setUser', data)
          api.changeDir({ to: '/' }).then(() => this.$router.push('/').catch(() => {}))
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
    verifyMfa() {
      api.loginMfa({ code: this.mfaCode, useBackup: this.useBackup })
        .then(user => {
          this.$store.commit('setUser', user)
          api.changeDir({ to: '/' }).then(() => this.$router.push('/').catch(() => {}))
        })
        .catch(() => {
          this.error = this.lang('Invalid code')
          this.mfaCode = ''
        })
    },
    onMfaInput() {
      this.error = ''
      // Backup codes are printed in uppercase with a hyphen, e.g. ABCDE-12345.
      // Auto-uppercase so users see the value as it was issued.
      if (this.useBackup) {
        this.mfaCode = (this.mfaCode || '').toUpperCase()
      }
    },
    completeSetup() {
      api.loginMfaSetup({ code: this.mfaCode })
        .then(res => {
          this.pendingUser = res.user
          this.setupBackupCodes = res.backup_codes
        })
        .catch(() => {
          this.error = this.lang('Invalid code')
          this.mfaCode = ''
        })
    },
    finishSetup() {
      this.$store.commit('setUser', this.pendingUser)
      api.changeDir({ to: '/' }).then(() => this.$router.push('/').catch(() => {}))
    },
    toggleBackup() {
      this.useBackup = !this.useBackup
      this.mfaCode = ''
    },
    cancel() {
      api.loginMfaCancel().catch(() => {})
      this.reset()
    },
    reset() {
      this.step = 'password'
      this.mfaCode = ''
      this.password = ''
      this.useBackup = false
      this.enrollment = null
      this.setupBackupCodes = null
      this.pendingUser = null
      this.error = ''
      this.$nextTick(() => this.$refs.username && this.$refs.username.focus())
    },
    drawQr() {
      if (!this.$refs.qrCanvas || !this.enrollment) return
      QRCode.toCanvas(this.$refs.qrCanvas, this.enrollment.otpauth_uri, { width: 220 }, () => {})
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

/* Row that holds an inline link on the left and the form's submit button(s)
   on the right. Wraps to two lines on narrow screens so the link never
   collides with the buttons. */
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

.login-error {
  margin-top: 0.75em;
  text-align: center;
}
</style>
