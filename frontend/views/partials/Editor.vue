<template>
  <div>
    <div class="modal-card">
      <header class="modal-card-head">
        <p class="modal-card-title">
          {{ currentItem.name }}
        </p>
      </header>
      <section class="modal-card-body preview">
        <template>
          <prism-editor v-model="content" language="md" :readonly="!can('write')" :line-numbers="lineNumbers" />
        </template>
      </section>
      <footer class="modal-card-foot">
        <button v-if="can('write')" class="button" type="button" @click="saveFile()">
          {{ lang('Save') }}
        </button>
        <button class="button" type="button" @click="$parent.close()">
          {{ lang('Close') }}
        </button>
      </footer>
    </div>
  </div>
</template>

<script>
import api from '../../api/api'
import 'prismjs'
import 'prismjs/themes/prism.css'
import 'vue-prism-editor/dist/VuePrismEditor.css'
import PrismEditor from 'vue-prism-editor'

export default {
  name: 'Editor',
  components: { PrismEditor },
  props: [ 'item' ],
  data() {
    return {
      content: '',
      currentItem: '',
      lineNumbers: true,
    }
  },
  mounted() {
    this.currentItem = this.item
    api.downloadItem({
      path: this.item.path,
    })
      .then((res) => {
        this.content = res
      })
      .catch(error => this.handleError(error))
  },
  methods: {
    saveFile() {
      api.saveContent({
        name: this.item.name,
        content: this.content,
      })
        .then(() => {
          this.$toast.open({
            message: this.lang('Updated'),
            type: 'is-success',
          })
          this.$parent.close()
        })
        .catch(error => this.handleError(error))
    }
  },
}
</script>

<style scoped>
@media (min-width: 1100px) {
  .modal-card {
    width: 100%;
    min-width: 640px;
  }
}

.preview {
  min-height: 450px;
}
</style>
