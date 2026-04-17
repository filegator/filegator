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
              <span v-if="activeUploads" class="summary-text">
                ({{ formatSpeed(totalUploadSpeedBytes) }})
              </span>
              <span v-if="activeUploads && paused">
                ({{ lang('Paused') }})
              </span>
              <span v-if="! activeUploads">
                {{ lang('Done') }}
              </span>
              <span v-if="totalUploadsCount" class="summary-text">
                ({{ successUploadsCount }} successful, {{ failedUploadsCount }} failed, {{ totalUploadsCount }} total)
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
          <div v-for="entry in sortedUploadEntries" :key="entry.id">
            <div>
              <div>{{ entry.relativePath != '/' ? entry.relativePath : '' }}/{{ entry.fileName }}</div>
              <div class="entry-meta">
                {{ formatBytes(entry.size) }}
                <span v-if="showEntrySpeed(entry)">
                  , {{ formatSpeed(entry.speedBytesPerSecond) }}
                </span>
              </div>
              <div class="is-flex is-justify-between">
                <progress :class="[entryProgressClass(entry), 'progress is-large']" :value="entryProgress(entry)" max="100" />
                <a v-if="showRetry(entry)" class="progress-icon" @click="retryEntry(entry)">
                  <b-icon icon="redo" type="is-danger" />
                </a>
                <a v-else-if="showCancel(entry)" class="progress-icon" @click="cancelEntry(entry)">
                  <b-icon icon="times" />
                </a>
                <a v-else-if="showRemove(entry)" class="progress-icon" @click="removeEntry(entry)">
                  <b-icon icon="times" type="is-danger" />
                </a>
                <span v-else class="progress-icon">
                  <b-icon :icon="entry.isError ? 'times' : 'check'" :type="entry.isError ? 'is-danger' : 'is-success'" />
                </span>
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
      uploadEntries: [],
      nextUploadEntryId: 1,
    }
  },
  computed: {
    activeUploads() {
      return _.some(this.uploadEntries, entry => entry.inProgress)
    },
    totalUploadsCount() {
      return this.uploadEntries.length
    },
    successUploadsCount() {
      return _.filter(this.uploadEntries, entry => entry.isComplete && !entry.isError).length
    },
    failedUploadsCount() {
      return _.filter(this.uploadEntries, entry => entry.isError).length
    },
    totalUploadSpeedBytes() {
      return _.sumBy(this.uploadEntries, entry => entry.inProgress ? entry.speedBytesPerSecond : 0)
    },
    sortedUploadEntries() {
      return _.orderBy(this.uploadEntries, [
        entry => entry.isError ? 0 : 1,
        entry => entry.createdAt,
      ], ['asc', 'desc'])
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
        this.visible = true
        this.progressVisible = true
        this.createUploadEntry({
          fileName: file.name,
          relativePath: this.$store.state.cwd.location,
          size: file.size,
          retryable: false,
          resumableFile: null,
          inProgress: false,
          isComplete: true,
          isError: true,
        })
        this.$forceUpdate()
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

      const entry = this.createUploadEntry({
        fileName: file.fileName,
        relativePath: file.relativePath,
        size: file.size,
        retryable: true,
        resumableFile: file,
        inProgress: true,
        isComplete: false,
        isError: false,
      })
      this.resetEntrySpeed(entry)
      file.file.uploadEntryId = entry.id

      if (!this.paused) {
        this.resumable.upload()
      }
    })

    this.resumable.on('fileSuccess', (file) => {
      file.file.uploadingError = false
      this.updateEntry(file, {
        inProgress: false,
        isComplete: true,
        isError: false,
      })
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
      this.progressVisible = true
      this.updateEntry(file, {
        speedBytesPerSecond: 0,
        inProgress: false,
        isComplete: true,
        isError: true,
      })
      this.$forceUpdate()
    })
    this.resumable.on('fileProgress', (file) => {
      this.refreshEntrySpeed(this.findEntry(file))
      this.updateEntry(file, {
        inProgress: true,
      })
      this.$forceUpdate()
    })
  },
  methods: {
    createUploadEntry({fileName, relativePath, size, retryable, resumableFile, inProgress, isComplete, isError}) {
      const entry = {
        id: this.nextUploadEntryId++,
        fileName,
        relativePath,
        size,
        retryable,
        resumableFile,
        inProgress,
        isComplete,
        isError,
        speedBytesPerSecond: 0,
        lastUploadedBytes: 0,
        lastProgressAt: null,
        createdAt: Date.now() + this.nextUploadEntryId,
      }

      this.uploadEntries.push(entry)

      return entry
    },
    findEntry(file) {
      return _.find(this.uploadEntries, entry => entry.id == _.get(file, 'file.uploadEntryId'))
    },
    updateEntry(file, values) {
      const entry = this.findEntry(file)
      if (!entry) return

      Object.assign(entry, values)
    },
    entryProgress(entry) {
      if (entry.isComplete) return 100
      if (!entry.resumableFile || entry.size <= 0) return 100

      return entry.resumableFile.progress() * 100
    },
    currentUploadedBytes(entry) {
      if (!entry || !entry.resumableFile || entry.size <= 0) return 0

      return Math.min(entry.size, Math.max(0, entry.resumableFile.progress() * entry.size))
    },
    resetEntrySpeed(entry) {
      if (!entry) return

      const bytesUploaded = this.currentUploadedBytes(entry)

      entry.speedBytesPerSecond = 0
      entry.lastUploadedBytes = bytesUploaded
      entry.lastProgressAt = Date.now()
    },
    refreshEntrySpeed(entry) {
      if (!entry) return

      const now = Date.now()
      const bytesUploaded = this.currentUploadedBytes(entry)

      if (!entry.lastProgressAt) {
        this.resetEntrySpeed(entry)
        return
      }

      const elapsedMs = now - entry.lastProgressAt
      const deltaBytes = Math.max(0, bytesUploaded - entry.lastUploadedBytes)

      let speedBytesPerSecond = entry.speedBytesPerSecond
      if (elapsedMs > 0) {
        const currentSpeed = deltaBytes / (elapsedMs / 1000)
        speedBytesPerSecond = speedBytesPerSecond > 0
          ? speedBytesPerSecond * 0.7 + currentSpeed * 0.3
          : currentSpeed
      }

      entry.speedBytesPerSecond = speedBytesPerSecond
      entry.lastUploadedBytes = bytesUploaded
      entry.lastProgressAt = now
    },
    formatSpeed(bytesPerSecond) {
      return this.formatBytes(bytesPerSecond || 0) + '/s'
    },
    entryProgressClass(entry) {
      if (entry.isError) return 'is-danger'
      if (entry.isComplete) return 'is-success'
      return 'is-primary'
    },
    showRetry(entry) {
      return entry.retryable && entry.isError
    },
    showCancel(entry) {
      return entry.inProgress && entry.resumableFile
    },
    showEntrySpeed(entry) {
      return entry.inProgress && entry.resumableFile
    },
    showRemove(entry) {
      return entry.isError && !entry.retryable
    },
    retryEntry(entry) {
      if (!entry.resumableFile) return

      entry.isError = false
      entry.isComplete = false
      entry.inProgress = true
      entry.resumableFile.file.uploadingError = false
      this.resetEntrySpeed(entry)
      entry.resumableFile.retry()

      if (!this.paused) {
        this.resumable.upload()
      }

      this.$forceUpdate()
    },
    cancelEntry(entry) {
      if (!entry.resumableFile) return

      entry.resumableFile.cancel()
      this.uploadEntries = _.filter(this.uploadEntries, item => item.id != entry.id)
    },
    removeEntry(entry) {
      this.uploadEntries = _.filter(this.uploadEntries, item => item.id != entry.id)
    },
    closeWindow() {
      if (this.activeUploads) {
        this.$dialog.confirm({
          message: this.lang('Are you sure you want to stop all uploads?'),
          type: 'is-danger',
          cancelText: this.lang('Cancel'),
          confirmText: this.lang('Confirm'),
          onConfirm: () => {
            this.resumable.cancel()
            this.uploadEntries = []
            this.visible = false
          }
        })
      } else {
        this.visible = false
        this.resumable.cancel()
        this.uploadEntries = []
      }
    },
    toggleWindow() {
      this.progressVisible = ! this.progressVisible
    },
    togglePause() {
      if (this.paused) {
        _.forEach(this.uploadEntries, entry => {
          if (entry.inProgress) {
            this.resetEntrySpeed(entry)
          }
        })
        this.resumable.upload()
        this.paused = false
      } else {
        this.resumable.pause()
        _.forEach(this.uploadEntries, entry => {
          if (entry.inProgress) {
            this.resetEntrySpeed(entry)
          }
        })
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
.summary-text {
  margin-left: 6px;
}
.entry-meta {
  color: #6b7280;
  font-size: 0.9rem;
  margin-bottom: 6px;
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
