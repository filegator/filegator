/* eslint-disable */
<template>
  <div class="modal-card">
    <header class="modal-card-head">
      <p class="modal-card-title">
        {{ item.name }}
      </p>
    </header>
    <section class="modal-card-body preview">
      <textarea v-if="isText()" v-model="content" class="textarea" name="content" rows="20" />
      <div v-if="isImage()" class="image">
        <img :src="content">
      </div>
    </section>
    <footer class="modal-card-foot">
      <button class="button" type="button" @click="$parent.close()">
        {{ lang('Close') }}
      </button>
    </footer>
  </div>
</template>

<script>
import api from '../../api/api'

export default {
  name: 'Preview',
  props: [ 'item' ],
  data() {
    return {
      content: '',
    }
  },
  mounted() {
    if (this.isText()) {
      api.downloadItem({
        path: this.item.path,
      })
        .then((res) => {
          this.content = res
        })
        .catch(error => this.handleError(error))
    } else if (this.isImage()) {
      this.content =this.getDownloadLink(this.item.path)
    }
  },
  methods: {
    isText() {
      return this.hasExtension(['.txt', '.html', '.css', '.js', '.ts', '.php'])
    },
    isImage() {
      return this.hasExtension(['.jpg', '.jpeg', '.gif', '.png'])
    },
    hasExtension(exts) {
      return (new RegExp('(' + exts.join('|').replace(/\./g, '\\.') + ')$', 'i')).test(this.item.path)
    },
  },
}
</script>

<style scoped>
.preview {
  min-height: 450px;
}
</style>
