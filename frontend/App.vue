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

/* Dark Theme */
@media (prefers-color-scheme: dark) {
  :root {
    /* Dark theme primary colors */
    --color-primary-a0: #34b891;
    --color-primary-a20: #6ec8a8;
    --color-primary-transparent: #34b89123;

    /* Dark theme surface colors */
    --color-surface-a0: #121212;
    --color-surface-a10: #282828;
    --color-surface-a20: #3f3f3f;
    --color-surface-a30: #575757;
    --color-surface-a50: #8b8b8b;

    --color-text: #fff;
  }

  html, body, .navbar {
    background-color: var(--color-surface-a0);
  }

  .navbar-item,
    .label,
    .table,
    .table thead th,
    .modal-card-title,
    .modal-card-body,
    .button:focus,
    .dropdown-item,
    .checkbox:hover {
    color: var(--color-text);
  }

  .box, .table, .modal-card-head, .modal-card-foot {
    background-color: var(--color-surface-a10);
  }

  .table.is-hoverable tbody tr:not(.is-selected):hover {
    background-color: var(--color-primary-transparent);;
  }

  .file-row a, .node-tree > a, strong {
    color: var(--color-text) !important;
  }

  a.navbar-item:hover {
    background-color: var(--color-primary-transparent);;
    border-radius: 5px;
  }

  .input {
    background-color: var(--color-surface-a20);
    color: var(--color-text);
    border-color: transparent;
  }

  .input:hover {
    border-color: var(--color-primary-a20);
  }

  .table td, .table th {
    border-bottom: 1px solid var(--color-surface-a0);
  }

  .is-current-sort {
    border-bottom: 1px solid var(--color-primary-a0) !important;
  }

  /* Modal */
  .modal-card-body {
    background-color: var(--color-surface-a20);
  }

  .modal-card-head {
    border-bottom: 1px solid var(--color-surface-a0);
  }

  .modal-card-foot {
    border-top: 1px solid var(--color-surface-a0);
  }

  .modal input {
    background-color: var(--color-surface-a30);
  }

  /* Button */
  .button {
    background-color: var(--color-primary-transparent);
    border-color: var(--color-primary-a0);
    color: var(--color-text)
  }

  .button:hover {
    background-color: var(--color-primary-a0);
    border-color: var(--color-primary-a0);
    color: var(--color-text)
  }

  .button:active {
    color: var(--color-primary-a0) !important;
  }

  /* Dropdown */
  .dropdown-content {
    background-color: var(--color-surface-a20);
  }

  a.dropdown-item:hover {
    background-color: var(--color-primary-transparent);
    color: var(--color-text);
  }

  a:hover {
    color: var(--color-primary-a20);
  }

  #bottom-info {
    color: var(--color-surface-a50);
  }

  .select select {
    background-color: var(--color-surface-a20);
    border-color: var(--color-surface-a20);
    color: var(--color-text);
  }

  .select select:hover, .select:not(.is-multiple):not(.is-loading):hover::after {
    border-color: var(--color-primary-a0);
  }
}
</style>

