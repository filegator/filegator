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

          <b-field :label="lang('Password')" :type="formErrors.password ? 'is-danger' : ''" :message="formErrors.password">
            <b-input v-model="formFields.password" :placeholder="action == 'edit' ? lang('Leave blank for no change') : ''" password-reveal @keydown.native="formErrors.password = ''" />
          </b-field>
        </div>

        <b-field :label="lang('Homedir')" :type="formErrors.homedir ? 'is-danger' : ''" :message="formErrors.homedir">
          <b-input v-model="formFields.homedir" @focus="selectDir" />
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
        homedir: this.user.homedir,
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
    selectDir() {
      this.formErrors.homedir = ''

      this.$modal.open({
        parent: this,
        hasModalCard: true,
        component: Tree,
        events: {
          selected: dir => {
            this.formFields.homedir = dir.path
          }
        },
      })
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
    save() {

      let method = this.action == 'add' ? api.storeUser : api.updateUser

      method({
        key: this.user.username,
        role: this.formFields.role,
        name: this.formFields.name,
        username: this.formFields.username,
        homedir: this.formFields.homedir,
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

