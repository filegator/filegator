<template>
  <div>
    <a v-if="can('read')" @click="$router.push('/')" id="back-arrow">
      <b-icon icon="times"></b-icon>
    </a>

    <div id="login" class="columns is-centered">
      <div class="column is-narrow">
        <form @submit.prevent="login">
          <div class="box">
            <div class="has-text-centered">
              <img class="logo" :src="$store.state.config.logo">
            </div>
            <br>
            <b-field :label="lang('Username')">
              <b-input name="username" v-model="username" @input="error = ''" required></b-input>
            </b-field>
            <b-field :label="lang('Password')">
              <b-input type="password" name="password" v-model="password"  @input="error = ''" required></b-input>
            </b-field>

            <div class="is-flex is-justify-end">
              <button class="button is-primary">
                {{ lang('Login') }}
              </button>
            </div>

            <div v-if="error">
              <code>{{ error }}</code>
            </div>

          </div>
        </form>
      </div>
    </div>

  </div>
</template>

<script>
import api from '../api/api'

export default {
  name: 'Login',
  data() {
    return {
      username: '',
      password: '',
      error: '',
    }
  },
  methods: {
    login() {
      api.login({
        username: this.username,
        password: this.password,
      })
        .then(user => {
          this.$store.commit('setUser', user)
          api.changeDir({
            to: '/'
          }).then(() => this.$router.push('/'))
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
</style>
