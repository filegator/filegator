<template>
  <div v-if="$store.state.initialized" id="wrapper">
    <Login v-if="is('guest') && ! can('write') && ! can('read') && ! can('upload')" />
    <div v-else id="inner">
      <router-view />
    </div>
  </div>
</template>

<script>
import Login from './views/Login'

export default {
  name: 'App',
  components: { Login }
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
