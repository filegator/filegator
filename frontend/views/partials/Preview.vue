/* eslint-disable */
<template>
  <div>
    <div class="modal-card">
      <header class="modal-card-head">
        <p class="modal-card-title">
          {{ currentItem.name }}
        </p>
      </header>
      <section :class="[isImage(item.path) ? 'overflowyh' : '', 'modal-card-body preview']">
        <template v-if="isText(item.path)">
          <prism-editor v-model="content" language="md" :readonly="!can('write')" :line-numbers="lineNumbers" />
        </template>
        <div v-if="isImage(item.path)">
          <div class="columns is-mobile">
            <div class="column mainbox">
              <img :src="imageSrc(currentItem.path)" class="mainimg">
            </div>
            <div v-if="images.length > 1" class="column is-one-fifth sidebox">
              <ul>
                <li v-for="(image, index) in images" :key="index">
                  <img :src="imageSrc(image.path)" @click="currentItem = image">
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>
      <footer class="modal-card-foot">
        <button v-if="isText(item.path) && can('write')" class="button" type="button" @click="saveFile()">
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
import _ from 'lodash'

export default {
  name: 'Preview',
  components: { PrismEditor },
  props: [ 'item' ],
  data() {
    return {
      content: '',
      currentItem: '',
      lineNumbers: true,
    }
  },
  computed: {
    images() {
      return _.filter(this.$store.state.cwd.content, o => this.isImage(o.name))
    },
  },
  mounted() {
    this.currentItem = this.item
    if (this.isText(this.item.path)) {
      api.downloadItem({
        path: this.item.path,
      })
        .then((res) => {
          this.content = res
        })
        .catch(error => this.handleError(error))
    }
  },
  methods: {
    imageSrc(path) {
      return this.getDownloadLink(path)
    },
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

.mainbox {
  height: 70vh;
  display:flex;
  justify-content:center;
  align-items:center;
}

.mainimg {
  max-width:100%;
  max-height:100%;
}

.sidebox {
  overflow-y:auto;
  height: 70vh;
}

.sidebox {
  border-left: 1px solid #dbdbdb;
}

.sidebox img {
  padding: 5px 0 5px 0;
}

.preview {
  min-height: 450px;
}

.overflowyh {
  overflow-y: hidden;
}
</style>
