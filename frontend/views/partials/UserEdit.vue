<template>
  <div class="modal-card">
    <header class="modal-card-head">
      <p class="modal-card-title">
        {{ user.name }}
      </p>
    </header>
    <section class="modal-card-body">
      <form @submit.prevent="save">
        <div v-if="user.role == 'user' || user.role == 'admin'" class="field">
          <b-field :label="lang('Role')">
            <b-select v-model="formFields.role" :placeholder="lang('Role')" expanded required>
              <option key="user" value="user">
                {{ lang('User') }}
              </option>
              <option key="admin" value="admin">
                {{ lang('Admin') }}
              </option>
            </b-select>
          </b-field>

          <b-field :label="lang('Username')" :type="formErrors.username ? 'is-danger' : ''" :message="formErrors.username">
            <b-input v-model="formFields.username" @keydown.native="formErrors.username = ''" />
          </b-field>

          <b-field :label="lang('Name')" :type="formErrors.name ? 'is-danger' : ''" :message="formErrors.name">
            <b-input v-model="formFields.name" @keydown.native="formErrors.name = ''" />
          </b-field>

          <b-field :label="lang('Email')" :type="formErrors.email ? 'is-danger' : ''" :message="formErrors.email">
            <b-input v-model="formFields.email" type="email" @keydown.native="formErrors.email = ''" />
          </b-field>

          <b-field :label="lang('Password')" :type="formErrors.password ? 'is-danger' : ''" :message="formErrors.password">
            <b-input v-model="formFields.password" :placeholder="action == 'edit' ? lang('Leave blank for no change') : ''" password-reveal @keydown.native="formErrors.password = ''" />
          </b-field>

          <div v-if="action == 'edit' && user.mfa_enabled" class="field">
            <span class="tag is-info is-light">{{ lang('MFA enabled') }}</span>
            <button type="button" class="button is-small is-danger is-light" style="margin-left: 0.5em" @click="resetMfa">
              {{ lang('Reset MFA') }}
            </button>
          </div>
        </div>

        <b-field :label="folderFieldLabel" :type="formErrors.homedir ? 'is-danger' : ''" :message="formErrors.homedir">
          <div class="folders-list">
            <div
              v-for="(folder, idx) in formFields.homedirs"
              :key="idx"
              class="field has-addons folder-row"
            >
              <b-input
                v-model="formFields.homedirs[idx]"
                expanded
                @focus="() => selectDir(idx)"
                @input="formErrors.homedir = ''"
              />
              <p v-if="formFields.homedirs.length > 1" class="control">
                <button
                  type="button"
                  class="button is-danger"
                  :title="lang('Remove this folder')"
                  @click="removeFolder(idx)"
                >×</button>
              </p>
            </div>
            <button
              type="button"
              class="button is-small is-info is-light add-folder-btn"
              @click="addFolder"
            >
              + {{ lang('Add another folder') }}
            </button>
          </div>
        </b-field>

        <b-field :label="lang('Permissions')">
          <div class="block">
            <b-checkbox v-model="permissions.read">
              {{ lang('Read') }}
            </b-checkbox>
            <b-checkbox v-model="permissions.write">
              {{ lang('Write') }}
            </b-checkbox>
            <b-checkbox v-model="permissions.upload">
              {{ lang('Upload') }}
            </b-checkbox>
            <b-checkbox v-model="permissions.download">
              {{ lang('Download permission') }}
            </b-checkbox>
            <b-checkbox v-model="permissions.batchdownload">
              {{ lang('Batch Download') }}
            </b-checkbox>
            <b-checkbox v-model="permissions.zip">
              {{ lang('Zip') }}
            </b-checkbox>
            <b-checkbox v-model="permissions.chmod">
              {{ lang('Chmod') }}
            </b-checkbox>
          </div>
        </b-field>
      </form>
    </section>
    <footer class="modal-card-foot">
      <button class="button" type="button" @click="$parent.close()">
        {{ lang('Close') }}
      </button>
      <button class="button is-primary" type="button" @click="confirmSave">
        {{ lang('Save') }}
      </button>
    </footer>
  </div>
</template>

<script>
import Tree from './Tree'
import api from '../../api/api'
import _ from 'lodash'

export default {
  name: 'UserEdit',
  props: [ 'user', 'action' ],
  data() {
    return {
      formFields: {
        role: this.user.role,
        name: this.user.name,
        username: this.user.username,
        email: this.user.email || '',
        // Multi-folder: copy the array, falling back to wrapping a legacy
        // scalar homedir. Always ensure at least one (possibly empty) row
        // so the dialog renders the input even for new users.
        homedirs: (Array.isArray(this.user.homedirs) && this.user.homedirs.length)
          ? [...this.user.homedirs]
          : [this.user.homedir || ''],
        password: '',
      },
      formErrors: {},
      permissions: {
        read: _.find(this.user.permissions, p => p == 'read') ? true : false,
        write: _.find(this.user.permissions, p => p == 'write') ? true : false,
        upload: _.find(this.user.permissions, p => p == 'upload') ? true : false,
        download: _.find(this.user.permissions, p => p == 'download') ? true : false,
        batchdownload: _.find(this.user.permissions, p => p == 'batchdownload') ? true : false,
        zip: _.find(this.user.permissions, p => p == 'zip') ? true : false,
        chmod: _.find(this.user.permissions, p => p == 'chmod') ? true : false,
      }
    }
  },
  computed: {
    folderFieldLabel() {
      return this.formFields.homedirs.length > 1
        ? this.lang('Folders')
        : this.lang('Folder')
    },
  },
  watch: {
    'permissions.read' (val) {
      if (!val) {
        this.permissions.write = false
        this.permissions.batchdownload = false
        this.permissions.zip = false
        this.permissions.chmod = false
      }
    },
    'permissions.write' (val) {
      if (val) {
        this.permissions.read = true
      } else {
        this.permissions.zip = false
        this.permissions.chmod = false
      }
    },
    'permissions.download' (val) {
      if (!val) {
        this.permissions.batchdownload = false
      }
    },
    'permissions.batchdownload' (val) {
      if (val) {
        this.permissions.read = true
        this.permissions.download = true
      }
    },
    'permissions.zip' (val) {
      if (val) {
        this.permissions.read = true
        this.permissions.write = true
      }
    },
    'permissions.chmod' (val) {
      if (val) {
        this.permissions.read = true
        this.permissions.write = true
      }
    },
  },
  methods: {
    selectDir(idx) {
      this.formErrors.homedir = ''

      this.$modal.open({
        parent: this,
        hasModalCard: true,
        component: Tree,
        events: {
          selected: dir => {
            // $set is required for index assignment to stay reactive in
            // Vue 2. Plain `this.formFields.homedirs[idx] = path` would
            // mutate the array but skip change detection on that slot.
            this.$set(this.formFields.homedirs, idx, dir.path)
          }
        },
      })
    },
    addFolder() {
      this.formFields.homedirs.push('')
    },
    removeFolder(idx) {
      // Keep at least one row visible; the validator on the backend
      // will reject an empty homedirs array regardless.
      if (this.formFields.homedirs.length <= 1) return
      this.formFields.homedirs.splice(idx, 1)
    },
    getPermissionsArray() {
      return _.reduce(this.permissions, (result, value, key) => {
        if (value == true) {
          result.push(key)
        }
        return result
      }, [])
    },
    confirmSave() {

      if (this.formFields.role == 'guest' && this.getPermissionsArray().length) {
        this.$dialog.confirm({
          message: this.lang('Are you sure you want to allow access to everyone?'),
          type: 'is-danger',
          cancelText: this.lang('Cancel'),
          confirmText: this.lang('Confirm'),
          onConfirm: () => {
            this.save()
          }
        })
      } else {
        this.save()
      }
    },
    resetMfa() {
      this.$dialog.confirm({
        message: this.lang('Reset MFA for this user? They will need to re-enroll on next login.'),
        type: 'is-danger',
        cancelText: this.lang('Cancel'),
        confirmText: this.lang('Reset'),
        onConfirm: () => {
          api.adminResetMfa({ username: this.user.username })
            .then(() => {
              this.$toast.open({ message: this.lang('MFA reset'), type: 'is-success' })
              this.$emit('updated')
              this.$parent.close()
            })
            .catch(e => this.handleError(e))
        }
      })
    },
    save() {

      let method = this.action == 'add' ? api.storeUser : api.updateUser

      // Strip blank rows before submitting; the backend would do the
      // same but trimming here gives a tighter payload and lets
      // single-element submissions still set the legacy `homedir`
      // back-compat field correctly.
      const homedirs = this.formFields.homedirs
        .map(h => (typeof h === 'string') ? h.trim() : '')
        .filter(h => h !== '')

      method({
        key: this.user.username,
        role: this.formFields.role,
        name: this.formFields.name,
        username: this.formFields.username,
        email: this.formFields.email,
        // New canonical key — Phase 4 backend reads this first.
        homedirs: homedirs,
        // Back-compat scalar — read by pre-refactor backends; harmless
        // for the new backend (normaliseHomedirsInput prefers the array).
        // Phase 10 drops this.
        homedir: homedirs[0] || '',
        password: this.formFields.password,
        permissions: this.getPermissionsArray(),
      })
        .then(res => {
          this.$toast.open({
            message: this.lang('Updated'),
            type: 'is-success',
          })
          this.$emit('updated', res)
          this.$parent.close()
        })
        .catch(errors => {
          if (typeof errors.response.data.data != 'object') {
            this.handleError(errors)
          }
          _.forEach(errors.response.data, err => {
            _.forEach(err, (val, key) => {
              this.formErrors[key] = this.lang(val)
              this.$forceUpdate()
            })
          })
        })
    },
  },
}
</script>

<style scoped>
.folders-list {
  width: 100%;
}
.folder-row {
  margin-bottom: 0.4em;
}
.folder-row:last-of-type {
  margin-bottom: 0.5em;
}
.add-folder-btn {
  margin-top: 0.25em;
}
</style>
