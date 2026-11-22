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
        <button class="button" type="button" @click="close()">
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
      originalContent: '',
      currentItem: '',
      lineNumbers: true,
      confirming: false,
    }
  },
  computed: {
    isDirty() {
      return this.content !== this.originalContent
    },
  },
  mounted() {
    this.currentItem = this.item
    api.downloadItem({
      path: this.item.path,
    })
      .then((res) => {
        this.content = res
        this.originalContent = res
      })
      .catch(error => this.handleError(error))

    document.addEventListener('keyup', this.onKeyUp)
    this.backdrop = this.$parent.$el.querySelector('.modal-background')
    if (this.backdrop) {
      this.backdrop.addEventListener('click', this.close)
    }
  },
  beforeDestroy() {
    document.removeEventListener('keyup', this.onKeyUp)
    if (this.backdrop) {
      this.backdrop.removeEventListener('click', this.close)
    }
  },
  methods: {
    onKeyUp(event) {
      if (event.key === 'Escape' || event.keyCode === 27) {
        this.close()
      }
    },
    close() {
      if (!this.isDirty) {
        this.$parent.close()
        return
      }
      if (this.confirming) {
        return
      }
      this.confirming = true
      this.$dialog.confirm({
        message: this.lang('You have unsaved changes. Are you sure you want to close without saving?'),
        type: 'is-danger',
        cancelText: this.lang('Cancel'),
        confirmText: this.lang('Confirm'),
        onConfirm: () => {
          this.confirming = false
          this.$parent.close()
        },
        onCancel: () => {
          // defer so the escape keyup that closed the dialog does not reopen it
          setTimeout(() => { this.confirming = false }, 0)
        },
      })
    },
    saveFile() {
      api.saveContent({
        name: this.item.name,
        content: this.content,
      })
        .then((result) => {
          if (result.data !== 'Done') {
            this.handleError(this.lang('Unable to save file'))
            return
          }
          this.originalContent = this.content
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
