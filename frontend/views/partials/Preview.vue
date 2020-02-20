/* eslint-disable */
<template>
  <div class="modal-card">
    <header class="modal-card-head">
      <p class="modal-card-title">
        {{ item.name }}
      </p>
    </header>
    <section class="modal-card-body preview">
      <textarea v-if="isText(item.path)" v-model="content" class="textarea" name="content" rows="20" />
      <div v-if="isImage(item.path)" class="image">
        <img :src="content">
      </div>
    </section>
    <footer class="modal-card-foot">
      <button v-if="isText(item.path) && can(['write'])" class="button" type="button" @click="saveFile()">
        {{ lang('Save') }}
      </button>
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
    if (this.isText(this.item.path)) {
      api.downloadItem({
        path: this.item.path,
      })
        .then((res) => {
          this.content = res
        })
        .catch(error => this.handleError(error))
    } else if (this.isImage(this.item.path)) {
      this.content =this.getDownloadLink(this.item.path)
    }
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
.preview {
  min-height: 450px;
}
</style>
