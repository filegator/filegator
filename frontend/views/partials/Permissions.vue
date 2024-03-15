<template>
  <div class="modal-card">
    <header class="modal-card-head">
      <p class="modal-card-title">
        {{ lang('Change permissions for') }} {{ name }}
      </p>
    </header>
    <section class="modal-card-body">
      <div class="columns permission-item" v-for="(typePermissions, type) in table" :key="type">
        <div class="column permission-type is-3">
          {{ type }}
        </div>
        <div class="column permission-name is-3" v-for="(value, permission) in typePermissions" :key="permission">
          <b-checkbox :value="value" @input="changePermission(type, permission, !value)">
            {{ permission }}
          </b-checkbox>
        </div>
      </div>
      <div class="columns permission-item">
        <div class="column permission-type is-3">
          {{ lang('Permissions') }}
        </div>
        <div class="column permission-type is-3 manual-permission-cell">
          <b-field>
            <b-input
              :value="String(newPermissions).padStart(3, '0')"
              @input="v => newPermissions = parseInt(v.slice(-3))"
              maxlength="4"
              required
            />
          </b-field>
        </div>
      </div>
      <div class="columns permission-item" v-if="isDir">
        <div class="column permission-type is-3">
          {{ lang('Recursive') }}
        </div>
        <div class="column permission-type is-9">
          <b-field>
            <b-select v-model="recursive" expanded>
              <option :value="null">
                {{ lang('No') }}
              </option>
              <option value="all">
                {{ lang('Both folders and files') }}
              </option>
              <option value="folders">
                {{ lang('Apply only for folders') }}
              </option>
              <option value="files">
                {{ lang('Apply only for files') }}
              </option>
            </b-select>
          </b-field>
        </div>
      </div>
    </section>
    <footer class="modal-card-foot">
      <button class="button" type="button" @click="$parent.close()">
        {{ lang('Cancel') }}
      </button>
      <button class="button is-primary" type="button" @click="$emit('saved', newPermissions, recursive) && $parent.close()">
        {{ lang('Save') }}
      </button>
    </footer>
  </div>
</template>

<script>
export default {
  name: 'Permissions',
  props: ['name', 'permissions', 'isDir'],
  data() {
    return {
      newPermissions: 700,
      /** @type {null | 'all' | 'folders' | 'files'} */
      recursive: null,
    }
  },
  computed: {
    table() {
      // credits to ChatGPT for this function
      const binary = parseInt(this.newPermissions, 8).toString(2).padStart(9, '0') // Convert octal to binary and pad to 9 digits
      return {
        owner: {
          read: binary[0] === '1',
          write: binary[1] === '1',
          execute: binary[2] === '1'
        },
        group: {
          read: binary[3] === '1',
          write: binary[4] === '1',
          execute: binary[5] === '1'
        },
        other: {
          read: binary[6] === '1',
          write: binary[7] === '1',
          execute: binary[8] === '1'
        }
      }
    }
  },
  mounted() {
    if (this.permissions && this.permissions !== -1) {
      this.newPermissions = this.permissions
    }
  },
  methods: {
    changePermission(type, permission, on) {
      // credits to ChatGPT for this function
      let permissionsObject = this.table
      permissionsObject[type][permission] = on
      let permissions = 0
      // Calculate owner permissions
      if (permissionsObject.owner.read) permissions += 400
      if (permissionsObject.owner.write) permissions += 200
      if (permissionsObject.owner.execute) permissions += 100
      // Calculate group permissions
      if (permissionsObject.group.read) permissions += 40
      if (permissionsObject.group.write) permissions += 20
      if (permissionsObject.group.execute) permissions += 10
      // Calculate other permissions
      if (permissionsObject.other.read) permissions += 4
      if (permissionsObject.other.write) permissions += 2
      if (permissionsObject.other.execute) permissions += 1
      this.newPermissions = permissions
      return permissions
    },
  }
}
</script>

<style>
.permission-type {
  text-transform: capitalize;
}
.permission-name {
  text-transform: capitalize;
}
.manual-permission-cell .help.counter {
  display: none;
}
</style>
