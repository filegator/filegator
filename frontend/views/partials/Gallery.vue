<template>
  <div>
    <div class="modal-card">
      <div class="modal-card-body preview">
        <strong>{{ currentItem.name }}</strong>
        <div class="columns is-mobile">
          <div class="column mainbox">
            <img :src="imageSrc(currentItem.path)" class="mainimg">
          </div>
          <div v-if="images.length > 1" class="column is-one-fifth sidebox">
            <ul>
              <li v-for="(image, index) in images" :key="index">
                <img v-lazy="imageSrc(image.path)" @click="currentItem = image">
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import _ from 'lodash'

export default {
  name: 'Gallery',
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
  },
  methods: {
    imageSrc(path) {
      return this.getDownloadLink(path)
    },
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
  cursor: pointer;
}

</style>
