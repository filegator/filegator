<template>
  <div class="container" style="padding: 2em 1em; max-width: 720px">
    <h1 class="title is-4">
      {{ lang('Security') }}
    </h1>

    <!-- Email -->
    <section class="box">
      <h2 class="subtitle is-5">
        {{ lang('Email address') }}
      </h2>
      <p>{{ lang('Used to recover your password if you forget it.') }}</p>
      <br>
      <b-field>
        <b-input v-model="email" type="email" :placeholder="lang('you@example.com')" />
        <p class="control">
          <button class="button is-primary" @click="saveEmail" :disabled="saving">
            {{ lang('Save') }}
          </button>
        </p>
      </b-field>
    </section>

    <!-- Change password -->
    <section class="box">
      <h2 class="subtitle is-5">
        {{ lang('Change password') }}
      </h2>
      <b-field :label="lang('Current password')" :type="cpErrors.oldpassword ? 'is-danger' : ''" :message="cpErrors.oldpassword">
        <b-input v-model="oldPw" type="password" password-reveal />
      </b-field>
      <b-field :label="lang('New password')" :type="cpErrors.newpassword ? 'is-danger' : ''" :message="cpErrors.newpassword">
        <b-input v-model="newPw" type="password" password-reveal />
      </b-field>
      <div class="is-flex is-justify-content-end">
        <button class="button is-primary" @click="changePassword">
          {{ lang('Update password') }}
        </button>
      </div>
    </section>

    <!-- MFA -->
    <section class="box">
      <h2 class="subtitle is-5">
        {{ lang('Multi-factor authentication') }}
      </h2>

      <div v-if="state === null">
        {{ lang('Loading…') }}
      </div>

      <div v-else-if="state.enabled">
        <p>{{ lang('MFA is enabled on your account.') }} <strong>{{ state.backup_codes_remaining }}</strong> {{ lang('backup code(s) remaining.') }}</p>
        <br>
        <div class="buttons">
          <button class="button" @click="openManage('regenerate')">
            {{ lang('Regenerate backup codes') }}
          </button>
          <button class="button is-danger is-light" v-if="!state.required_by_role" @click="openManage('disable')">
            {{ lang('Disable MFA') }}
          </button>
          <span v-else class="tag is-info is-light" style="align-self: center; margin-left: .5em">
            {{ lang('Required by your role') }}
          </span>
        </div>
      </div>

      <div v-else-if="enrollment">
        <p>{{ lang('Scan this QR code with an authenticator app, then enter the 6-digit code.') }}</p>
        <div class="has-text-centered" style="margin: 1em 0">
          <canvas ref="qrCanvas" />
        </div>
        <p style="font-family: monospace; word-break: break-all; font-size: 0.9em">
          {{ lang('Manual key') }}: {{ enrollment.secret }}
        </p>
        <br>
        <b-field :label="lang('6-digit code')">
          <b-input v-model="enrollCode" placeholder="123456" />
        </b-field>
        <div class="is-flex is-justify-content-end">
          <button class="button" @click="cancelEnroll">
            {{ lang('Cancel') }}
          </button>
          <button class="button is-primary" @click="confirmEnroll">
            {{ lang('Verify') }}
          </button>
        </div>

        <div v-if="backupCodes" class="notification is-warning" style="margin-top: 1em">
          <p><strong>{{ lang('Save these backup codes') }}</strong></p>
          <p>{{ lang('Each can be used once if you lose access to your authenticator. They will not be shown again.') }}</p>
          <ul style="font-family: monospace; margin-top: 0.5em">
            <li v-for="c in backupCodes" :key="c">
              {{ c }}
            </li>
          </ul>
        </div>
      </div>

      <div v-else>
        <p>{{ lang('Add a second factor with a TOTP authenticator app.') }}</p>
        <br>
        <button class="button is-primary" @click="beginEnroll">
          {{ lang('Enable MFA') }}
        </button>
      </div>
    </section>

    <!-- Re-auth modal for disable / regenerate -->
    <b-modal :active.sync="manageOpen" has-modal-card>
      <div class="modal-card">
        <header class="modal-card-head">
          <p class="modal-card-title">
            {{ manageMode === 'disable' ? lang('Disable MFA') : lang('Regenerate backup codes') }}
          </p>
        </header>
        <section class="modal-card-body">
          <b-field :label="lang('Current password')">
            <b-input v-model="reauthPassword" type="password" password-reveal />
          </b-field>
          <b-field :label="useBackupForManage ? lang('Backup code') : lang('6-digit code')">
            <b-input v-model="reauthCode" :placeholder="useBackupForManage ? 'XXXXX-XXXXX' : '123456'" />
          </b-field>
          <a @click="useBackupForManage = !useBackupForManage">
            {{ useBackupForManage ? lang('Use authenticator code') : lang('Use a backup code') }}
          </a>
        </section>
        <footer class="modal-card-foot">
          <button class="button" @click="manageOpen = false">
            {{ lang('Cancel') }}
          </button>
          <button class="button is-primary" @click="performManage">
            {{ lang('Continue') }}
          </button>
        </footer>
      </div>
    </b-modal>
  </div>
</template>

<script>
import api from '../api/api'
import QRCode from 'qrcode'

export default {
  name: 'Security',
  data() {
    return {
      state: null,
      email: '',
      saving: false,
      enrollment: null,
      enrollCode: '',
      backupCodes: null,
      oldPw: '',
      newPw: '',
      cpErrors: {},
      manageOpen: false,
      manageMode: 'disable',
      reauthPassword: '',
      reauthCode: '',
      useBackupForManage: false,
    }
  },
  mounted() {
    this.refresh()
  },
  methods: {
    refresh() {
      api.mfaState().then(s => {
        this.state = s
        this.email = s.email || ''
      }).catch(e => this.handleError(e))
    },
    saveEmail() {
      this.saving = true
      api.updateMyEmail({ email: this.email })
        .then(() => {
          this.$toast.open({ message: this.lang('Saved'), type: 'is-success' })
        })
        .catch(e => {
          if (e.response && e.response.data && e.response.data.data && e.response.data.data.email) {
            this.$toast.open({ message: this.lang(e.response.data.data.email), type: 'is-danger' })
          } else {
            this.handleError(e)
          }
        })
        .finally(() => { this.saving = false })
    },
    changePassword() {
      this.cpErrors = {}
      api.changePassword({ oldpassword: this.oldPw, newpassword: this.newPw })
        .then(() => {
          this.oldPw = ''
          this.newPw = ''
          this.$toast.open({ message: this.lang('Password updated'), type: 'is-success' })
        })
        .catch(errors => {
          if (errors.response && errors.response.data) {
            const d = errors.response.data.data
            if (typeof d === 'object') this.cpErrors = d
            else this.handleError(errors)
          }
        })
    },
    beginEnroll() {
      api.mfaBeginEnroll().then(data => {
        this.enrollment = data
        this.backupCodes = null
        this.enrollCode = ''
        this.$nextTick(() => this.drawQr())
      }).catch(e => this.handleError(e))
    },
    drawQr() {
      if (!this.$refs.qrCanvas || !this.enrollment) return
      QRCode.toCanvas(this.$refs.qrCanvas, this.enrollment.otpauth_uri, { width: 220 }, () => {})
    },
    confirmEnroll() {
      api.mfaConfirmEnroll({ code: this.enrollCode })
        .then(res => {
          this.backupCodes = res.backup_codes
          this.refresh()
        })
        .catch(() => {
          this.$toast.open({ message: this.lang('Invalid code'), type: 'is-danger' })
        })
    },
    cancelEnroll() {
      this.enrollment = null
      this.backupCodes = null
    },
    openManage(mode) {
      this.manageMode = mode
      this.manageOpen = true
      this.reauthPassword = ''
      this.reauthCode = ''
      this.useBackupForManage = false
    },
    performManage() {
      const args = {
        password: this.reauthPassword,
        code: this.reauthCode,
        useBackup: this.useBackupForManage,
      }
      const call = this.manageMode === 'disable'
        ? api.mfaDisable(args)
        : api.mfaRegenerateBackupCodes(args)
      call.then(res => {
        this.manageOpen = false
        if (this.manageMode === 'regenerate') {
          this.backupCodes = res.backup_codes
        } else {
          this.$toast.open({ message: this.lang('MFA disabled'), type: 'is-success' })
        }
        this.refresh()
      }).catch(() => {
        this.$toast.open({ message: this.lang('Verification failed'), type: 'is-danger' })
      })
    },
  },
}
</script>
