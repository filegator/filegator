<template>
  <li class="node-tree">
    <b-button :type="button_type" size="is-small" @click="toggleButton(node)">
      <span class="icon"><i :class="icon" /></span>
    </b-button>
    &nbsp;
    <!-- eslint-disable-next-line -->
    <a @click="$emit('selected', node)">{{ node.name }}</a>

    <ul v-if="node.children && node.children.length">
      <TreeNode v-for="(child, index) in node.children" :key="index" :node="child" @selected="$emit('selected', $event)" />
    </ul>
  </li>
</template>

<script>
import api from '../../api/api'
import _ from 'lodash'

export default {
  name: 'TreeNode',
  props: {
    node: {
      type: Object,
      required: true
    }
  },
  data() {
    return {
      active: false,
      button_type: 'is-primary'
    }
  },
  computed: {
    icon() {
      return {
        'fas': true,
        'mdi-24px': true,
        'fa-plus': ! this.active,
        'fa-minus': this.active,
      }
    },
  },
  mounted() {
    if (this.node.path == '/') {
      this.$store.commit('resetTree')
      this.toggleButton(this.node)
    }
  },
  methods: {
    toggleButton(node) {
      if (! this.active) {
        this.active = true
        this.button_type = 'is-primary is-loading'
        api.getDir({
          dir: node.path
        })
          .then(ret => {
            this.$store.commit('updateTreeNode', {
              children: _.filter(ret.files, ['type', 'dir']),
              path: node.path,
            })
            this.$forceUpdate()
            this.button_type = 'is-primary'
          })
          .catch(error => this.handleError(error))
      } else {
        this.active = false
        this.$store.commit('updateTreeNode', {
          children: [],
          path: node.path,
        })
      }
    }
  }
}
</script>

<style scoped>
a {
  color: #373737;
  font-weight: bold;
}
</style>
