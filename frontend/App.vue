<template>
  <div v-if="$store.state.initialized" id="wrapper">
    <!--
      Default behaviour for a guest with no read/write/upload permissions is
      to force-render the login form regardless of route. Public auth flows
      (password reset request + confirm) need to be reachable in exactly that
      state, so allow router-view to take over for those specific routes.
    -->
    <Login v-if="is('guest') && ! can('write') && ! can('read') && ! can('upload') && ! isPublicAuthRoute" />
    <div v-else id="inner">
      <router-view />
    </div>
  </div>
</template>

<script>
import Login from './views/Login'

export default {
  name: 'App',
  components: { Login },
  computed: {
    isPublicAuthRoute() {
      return ['forgot-password', 'reset-password'].includes(this.$route.name)
    },
  },
}
</script>

<style lang="scss">
@import "~bulma/sass/utilities/_all";

// Theme base variables
@import "assets/scss/theme/variables";

@import "~bulma";
@import "~buefy/src/scss/buefy";

// Custom styles
@import "assets/scss/custom";

/* Dark Theme */
@import "assets/scss/theme/dark";
</style>
