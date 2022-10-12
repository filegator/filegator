<template>
  <div>
    <div v-if="visible && dropZone == false" class="progress-box">
      <div class="box">
        <div>
          <div class="is-flex is-justify-between">
            <div class="is-flex">
              <a @click="toggleWindow">
                <b-icon :icon="progressVisible ? 'angle-down' : 'angle-up'" />
              </a>
              <span v-if="activeUploads">
                {{ lang('Uploading files', resumable.getSize() > 0 ? Math.round(resumable.progress()*100) : 100, formatBytes(resumable.getSize())) }}
              </span>
              <span v-if="activeUploads && paused">
                ({{ lang('Paused') }})
              </span>
              <span v-if="! activeUploads">
                {{ lang('Done') }}
              </span>
            </div>
            <div class="is-flex">
              <a v-if="activeUploads" @click="togglePause()">
                <b-icon :icon="paused ? 'play-circle' : 'pause-circle'" />
              </a>
              <a class="progress-icon" @click="closeWindow()">
                <b-icon icon="times" />
              </a>
            </div>
          </div>
          <hr>
        </div>
        <div v-if="progressVisible" class="progress-items">
          <div v-for="(file, index) in resumable.files.slice().reverse()" :key="index">
            <div>
              <div>{{ file.relativePath != '/' ? file.relativePath : '' }}/{{ file.fileName }}</div>
              <div class="is-flex is-justify-between">
                <progress :class="[file.file.uploadingError ? 'is-danger' : 'is-primary', 'progress is-large']" :value="file.size > 0 ? file.progress()*100 : 100" max="100" />
                <a v-if="! file.isUploading() && file.file.uploadingError" class="progress-icon" @click="file.retry()">
                  <b-icon icon="redo" type="is-danger" />
                </a>
                <a v-else class="progress-icon" @click="file.cancel()">
                  <b-icon :icon="file.isComplete() ? 'check' : 'times'" />
                </a>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Resumable from 'resumablejs'
import Vue from 'vue'
import api from '../../api/api'
import axios from 'axios'
import _ from 'lodash'

export default {
  name: 'Upload',
  props: [ 'files', 'dropZone' ],
  data() {
    return {
      resumable: {},
      visible: false,
      paused: false,
      progressVisible: false,
      progress: 0,
    }
  },
  computed: {
    activeUploads() {
      return this.resumable.files.length > 0 && this.resumable.progress() < 1
    },
  },
  watch: {
    'files' (files) {
      _.forEach(files, file => {
        this.resumable.addFile(file)
      })
    },
  },
  mounted() {
    this.resumable = new Resumable({
      target: Vue.config.baseURL+'/upload',
      headers: {
        'x-csrf-token': axios.defaults.headers.common['x-csrf-token']
      },
      withCredentials: true,
      simultaneousUploads: this.$store.state.config.upload_simultaneous,
      minFileSize: 0,
      chunkSize: this.$store.state.config.upload_chunk_size,
      maxFileSize: this.$store.state.config.upload_max_size,
      maxFileSizeErrorCallback: (file) => {
        this.$notification.open({
          message: this.lang('File size error', file.name, this.formatBytes(this.$store.state.config.upload_max_size)),
          type: 'is-danger',
          queue: false,
          indefinite: true,
        })
      }
    })

    if (!this.resumable.support) {
      this.$dialog.alert({
        type: 'is-danger',
        message: this.lang('Browser not supported.'),
      })
      return
    }

    this.resumable.assignDrop(document.getElementById('dropzone'))

    this.resumable.on('fileAdded', (file) => {
      this.visible = true
      this.progressVisible = true

      if(file.relativePath === undefined || file.relativePath === null || file.relativePath == file.fileName) file.relativePath = this.$store.state.cwd.location
      else file.relativePath = [this.$store.state.cwd.location, file.relativePath].join('/').replace('//', '/').replace(file.fileName, '').replace(/\/$/, '')

      if (!this.paused) {
        this.resumable.upload()
      }

    })

    this.resumable.on('fileSuccess', (file) => {
      file.file.uploadingError = false
      this.$forceUpdate()
      if (this.can('read')) {
        api.getDir({
          to: '',
        })
          .then(ret => {
            this.$store.commit('setCwd', {
              content: ret.files,
              location: ret.location,
            })
          })
          .catch(error => this.handleError(error))
      }
    })
    this.resumable.on('fileError', (file) => {
      file.file.uploadingError = true
    })
  },
  methods: {
    closeWindow() {
      if (this.activeUploads) {
        this.$dialog.confirm({
          message: this.lang('Are you sure you want to stop all uploads?'),
          type: 'is-danger',
          cancelText: this.lang('Cancel'),
          confirmText: this.lang('Confirm'),
          onConfirm: () => {
            this.resumable.cancel()
            this.visible = false
          }
        })
      } else {
        this.visible = false
        this.resumable.cancel()
      }
    },
    toggleWindow() {
      this.progressVisible = ! this.progressVisible
    },
    togglePause() {
      if (this.paused) {
        this.resumable.upload()
        this.paused = false
      } else {
        this.resumable.pause()
        this.paused = true
      }
    },
  },
}
</script>

<style scoped>
.progress-icon {
  margin-left: 15px;
}
.progress-box {
  position: fixed;
  width: 100%;
  bottom: -30px;
  left: 0;
  padding: 25px;
  max-height: 50%;
  z-index: 1;
}
.progress-items {
  overflow-y: scroll;
  margin-right: -100px;
  padding-right: 100px;
  max-height: 300px; /* fix this */
}
</style>
