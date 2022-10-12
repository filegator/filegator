<template>
  <div class="container">
    <Menu />

    <section class="actions is-flex is-justify-between">
      <div>
        <a @click="addUser">
          <b-icon icon="plus" size="is-small" /> {{ lang('New') }}
        </a>
      </div>
      <div>
        <Pagination :perpage="perPage" @selected="perPage = $event" />
      </div>
    </section>

    <b-table
      :data="users"
      :default-sort="defaultSort"
      :paginated="perPage > 0"
      :per-page="perPage"
      :current-page.sync="currentPage"
      :hoverable="true"
      :loading="isLoading"
    >
      <template slot-scope="props">
        <b-table-column :label="lang('Name')" field="name" sortable>
          <a @click="editUser(props.row)">
            {{ props.row.name }}
          </a>
        </b-table-column>

        <b-table-column :label="lang('Username')" field="username" sortable>
          <a @click="editUser(props.row)">
            {{ props.row.username }}
          </a>
        </b-table-column>

        <b-table-column :label="lang('Permissions')" field="role">
          {{ permissions(props.row.permissions) }}
        </b-table-column>

        <b-table-column :label="lang('Role')" field="role" sortable>
          {{ lang(capitalize(props.row.role)) }}
        </b-table-column>

        <b-table-column>
          <a v-if="props.row.role != 'guest'" @click="remove(props.row)">
            <b-icon icon="trash-alt" size="is-small" />
          </a>
        </b-table-column>
      </template>
    </b-table>
  </div>
</template>

<script>
import UserEdit from './partials/UserEdit'
import Menu from './partials/Menu'
import Pagination from './partials/Pagination'
import api from '../api/api'
import _ from 'lodash'

export default {
  name: 'Users',
  components: { Menu, Pagination },
  data() {
    return {
      perPage: '',
      currentPage: 1,
      isLoading: false,
      defaultSort: ['name', 'desc'],
      users: [],
    }
  },
  mounted() {
    api.listUsers()
      .then(ret => {
        this.users = ret
      })
      .catch(error => this.handleError(error))
  },
  methods: {
    remove(user) {
      this.$dialog.confirm({
        message: this.lang('Are you sure you want to do this?'),
        type: 'is-danger',
        cancelText: this.lang('Cancel'),
        confirmText: this.lang('Confirm'),
        onConfirm: () => {
          api.deleteUser({
            username: user.username
          })
            .then(() => {
              this.users = _.reject(this.users, u => u.username == user.username)
              this.$toast.open({
                message: this.lang('Deleted'),
                type: 'is-success',
              })
            })
            .catch(error => this.handleError(error))
          this.checked = []
        }
      })
    },
    permissions(array) {
      return _.join(array, ', ')
    },
    addUser() {
      this.$modal.open({
        parent: this,
        props: { user: { role: 'user'}, action: 'add' },
        hasModalCard: true,
        component: UserEdit,
        events: {
          updated: ret => {
            this.users.push(ret)
          }
        },
      })
    },
    editUser(user) {
      if (! user.username) {
        this.handleError('Missing username')
        return
      }
      this.$modal.open({
        parent: this,
        props: { user: user, action: 'edit' },
        hasModalCard: true,
        component: UserEdit,
        events: {
          updated: ret => {
            this.users.splice(_.findIndex(this.users, {username: ret.username}), 1, ret)
          }
        },
      })
    },
  }
}
</script>

<style scoped>
.actions {
  margin: 50px 0 30px 0;
}
</style>
