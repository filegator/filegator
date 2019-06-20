<template>
  <div class="modal-card">
    <header class="modal-card-head">
      <p class="modal-card-title">{{ $store.state.user.name }}</p>
    </header>
    <section class="modal-card-body">
      <form @submit="save">
        <b-field :label="lang('Old password')" :type="formErrors.oldpassword ? 'is-danger' : ''" :message="formErrors.oldpassword">
          <b-input v-model="oldpassword" @keydown.native="formErrors.oldpassword = ''" password-reveal required></b-input>
        </b-field>

        <b-field :label="lang('New password')" :type="formErrors.newpassword ? 'is-danger' : ''" :message="formErrors.newpassword">
          <b-input v-model="newpassword" @keydown.native="formErrors.newpassword = ''" password-reveal required></b-input>
        </b-field>
      </form>
    </section>
    <footer class="modal-card-foot">
      <button class="button" type="button" @click="$parent.close()">{{ lang('Close') }}</button>
      <button class="button is-primary" type="button" @click="save">{{ lang('Save') }}</button>
    </footer>
  </div>
</template>

<script>
import api from '../../api/api'

export default {
  name: 'Profile',
  data() {
    return {
      oldpassword: '',
      newpassword: '',
      formErrors: {},
    }
  },
  methods: {
    save() {
      api.changePassword({
        oldpassword: this.oldpassword,
        newpassword: this.newpassword,
      })
        .then(res => {
          this.$toast.open({
            message: this.lang('Updated'),
            type: 'is-success',
          })
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
