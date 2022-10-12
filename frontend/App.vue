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

// Primary color
$primary: #34B891;
$primary-invert: findColorInvert($primary);

$colors: (
    "primary": ($primary, $primary-invert),
    "info": ($info, $info-invert),
    "success": ($success, $success-invert),
    "warning": ($warning, $warning-invert),
    "danger": ($danger, $danger-invert),
);

// Links
$link: $primary;
$link-invert: $primary-invert;
$link-focus-border: $primary;

// Disable the widescreen breakpoint
$widescreen-enabled: false;

// Disable the fullhd breakpoint
$fullhd-enabled: false;

@import "~bulma";
@import "~buefy/src/scss/buefy";

// Custom styles
html, body, #wrapper, #inner, .container {
  height: 100%;
}

.container {
  margin: 0 auto;
}

.is-justify-between {
  justify-content: space-between;
}

.is-justify-start {
  justify-content: flex-start;
}

.is-justify-end {
  justify-content: flex-end;
}

.upload-draggable {
  display: flex!important;
  flex-direction: column;
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
}

.upload input[type=file] {
  z-index: -10;
}

.modal-card-foot {
  justify-content: flex-end;
}

@media all and (max-width: 1088px) {
  .container {
    padding: 20px;
  }
}

</style>

