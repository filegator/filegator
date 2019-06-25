<template>
  <div id="dropzone" class="container"
                     @dragover="dropZone = can('upload') && ! isLoading ? true : false"
                     @dragleave="dropZone = false"
                     @drop="dropZone = false">

    <div id="loading" v-if="isLoading"></div>

    <Upload v-if="can('upload')" v-show="dropZone == false" :files="files" :dropZone="dropZone"></Upload>

    <b-upload v-if="dropZone && ! isLoading" @input="files = $event" multiple drag-drop>
      <b class="drop-info">{{ lang('Drop files to upload') }}</b>
    </b-upload>

    <div class="container" v-if="!dropZone">

      <Menu></Menu>

      <div id="browser">

        <div v-if="can('read')" class="is-flex is-justify-between">
          <div class="breadcrumb" aria-label="breadcrumbs">
            <ul>
              <li v-for="(item, index) in breadcrumbs" :key="index">
                <a @click="goTo(item.path)">{{ item.name }}</a>
              </li>
            </ul>
          </div>
          <div>
            <a class="is-paddingless" @click="selectDir">
              <b-icon icon="sitemap" class="is-marginless" size="is-small"></b-icon>
            </a>
          </div>
        </div>

        <section class="actions is-flex is-justify-between">
          <div>
            <b-field v-if="can('upload') && ! checked.length" class="file is-inline-block">
              <b-upload @input="files = $event" multiple native>
                <a v-if="! checked.length" class="is-inline-block">
                  <b-icon icon="upload" size="is-small"></b-icon> {{ lang('Add files') }}
                </a>
              </b-upload>
            </b-field>
            <a v-if="can(['read', 'write']) && ! checked.length" class="is-inline-block">
              <b-dropdown aria-role="list" :disabled="checked.length > 0">
                <span slot="trigger">
                  <b-icon icon="plus" size="is-small"></b-icon> {{ lang('New') }}
                </span>

                <b-dropdown-item @click="create('dir')" aria-role="listitem">
                  <b-icon icon="folder" size="is-small"></b-icon> {{ lang('Folder') }}
                </b-dropdown-item>
                <b-dropdown-item @click="create('file')" aria-role="listitem">
                  <b-icon icon="file" size="is-small"></b-icon> {{ lang('File') }}
                </b-dropdown-item>

              </b-dropdown>
            </a>
            <a v-if="can('batchdownload') && checked.length" @click="batchDownload" class="is-inline-block">
              <b-icon icon="download" size="is-small"></b-icon> {{ lang('Download') }}
            </a>
            <a v-if="can('write') && checked.length" @click="copy" class="is-inline-block">
              <b-icon icon="copy" size="is-small"></b-icon> {{ lang('Copy') }}
            </a>
            <a v-if="can('write') && checked.length" @click="move" class="is-inline-block">
              <b-icon icon="external-link-square-alt" size="is-small"></b-icon> {{ lang('Move') }}
            </a>
            <a v-if="can(['write', 'zip']) && checked.length" @click="zip" class="is-inline-block">
              <b-icon icon="file-archive" size="is-small"></b-icon> {{ lang('Zip') }}
            </a>
            <a v-if="can('write') && checked.length" @click="remove" class="is-inline-block">
              <b-icon icon="trash-alt" size="is-small"></b-icon> {{ lang('Delete') }}
            </a>
          </div>
          <div v-if="can('read')">
            <Pagination :perpage="perPage" @selected="perPage = $event"></Pagination>
          </div>
        </section>

        <b-table v-if="can('read')"
                 :data="content"
                 :default-sort="defaultSort"
                 :paginated="perPage > 0"
                 :per-page="perPage"
                 :current-page.sync="currentPage"
                 :hoverable="true"
                 :is-row-checkable="(row) => row.type != 'back'"
                 :row-class="(row) => 'file-row type-'+row.type"
                 :checked-rows.sync="checked"
                 :loading="isLoading"
                 checkable>
                 <template slot-scope="props">

                   <b-table-column field="data.name" :label="lang('Name')" :custom-sort="sortByName" sortable>
                     <a @click="itemClick(props.row)" class="is-block name">
                       {{ props.row.name }}
                     </a>
                   </b-table-column>

                   <b-table-column field="data.size" :label="lang('Size')" :custom-sort="sortBySize" sortable numeric width="150">
                     {{ props.row.type == 'back' || props.row.type == 'dir' ? lang('Folder') : formatBytes(props.row.size) }}
                   </b-table-column>

                   <b-table-column field="data.time" :label="lang('Time')" :custom-sort="sortByTime" sortable numeric width="200">
                     {{ props.row.time ? formatDate(props.row.time) : '' }}
                   </b-table-column>

                   <b-table-column class="action-padding" width="51">
                     <b-dropdown v-if="props.row.type != 'back'" aria-role="list" position="is-bottom-left" :disabled="checked.length > 0">
                       <button class="button is-small" slot="trigger">
                         <b-icon icon="ellipsis-h" size="is-small"></b-icon>
                       </button>

                       <b-dropdown-item v-if="props.row.type == 'file' && can('download')" @click="download(props.row)" aria-role="listitem">
                         <b-icon icon="download" size="is-small"></b-icon> {{ lang('Download') }}
                       </b-dropdown-item>
                       <b-dropdown-item v-if="can('write')" @click="copy($event, props.row)" aria-role="listitem">
                         <b-icon icon="copy" size="is-small"></b-icon> {{ lang('Copy') }}
                       </b-dropdown-item>
                       <b-dropdown-item v-if="can('write')" @click="move($event, props.row)" aria-role="listitem">
                         <b-icon icon="external-link-square-alt" size="is-small"></b-icon> {{ lang('Move') }}
                       </b-dropdown-item>
                       <b-dropdown-item v-if="can('write')" @click="rename($event, props.row)" aria-role="listitem">
                         <b-icon icon="file-signature" size="is-small"></b-icon> {{ lang('Rename') }}
                       </b-dropdown-item>
                       <b-dropdown-item v-if="can(['write', 'zip']) && isArchive(props.row)" @click="unzip($event, props.row)" aria-role="listitem">
                         <b-icon icon="file-archive" size="is-small"></b-icon> {{ lang('Unzip') }}
                       </b-dropdown-item>
                       <b-dropdown-item v-if="can(['write', 'zip']) && ! isArchive(props.row)" @click="zip($event, props.row)" aria-role="listitem">
                         <b-icon icon="file-archive" size="is-small"></b-icon> {{ lang('Zip') }}
                       </b-dropdown-item>
                       <b-dropdown-item v-if="can('write')" @click="remove($event, props.row)" aria-role="listitem">
                         <b-icon icon="trash-alt" size="is-small"></b-icon> {{ lang('Delete') }}
                       </b-dropdown-item>
                       <b-dropdown-item v-if="props.row.type == 'file' && can('download')" v-clipboard:copy="getDownloadLink(props.row)" aria-role="listitem">
                         <b-icon icon="clipboard" size="is-small"></b-icon> {{ lang('Copy link') }}
                       </b-dropdown-item>

                     </b-dropdown>
                   </b-table-column>

                 </template>

                 <template slot="bottom-left">
                   <span>{{ lang('Selected', checked.length, totalCount) }}</span>
                 </template>

        </b-table>

      </div>
    </div>
  </div>
</template>

<script>
import Vue from 'vue'
import Menu from './partials/Menu'
import Tree from './partials/Tree'
import Pagination from './partials/Pagination'
import Upload from './partials/Upload'
import api from '../api/api'
import VueClipboard from 'vue-clipboard2'

Vue.use(VueClipboard)

export default {
  name: 'Browser',
  components: { Menu, Pagination, Upload },
  data() {
    return {
      dropZone: false,
      perPage: "",
      currentPage: 1,
      checked: [],
      isLoading: false,
      defaultSort: ['data.name', 'desc'],
      files: [],
    }
  },
  mounted() {
    if (this.can('read')) {
      this.loadFiles()
    }
  },
  watch: {
    '$route' (to, from) {
      this.isLoading = true
      this.checked = []
      this.currentPage = 1
      api.changeDir({
        to: to.query.cd
      })
        .then(ret => {
          this.$store.commit('setCwd', {
            content: ret.files,
            location: ret.location,
          })
          this.isLoading = false
        })
        .catch(error => {
          this.isLoading = false
          this.handleError(error)
        })
    },
  },
  computed: {
    breadcrumbs() {
      let path = ''
      let breadcrumbs = [{name: this.lang('Home'), path: '/'}]

      _.forEach(_.split(this.$store.state.cwd.location, '/'), (dir) => {
        path += dir + '/'
        breadcrumbs.push({
          name: dir,
          path: path,
        })
      })

      return _.filter(breadcrumbs, o => o.name)
    },
    content() {
      return this.$store.state.cwd.content
    },
    totalCount() {
      return _.sumBy(this.$store.state.cwd.content, (o) => {
        return o.type == 'file' || o.type == 'dir'
      }) || 0
    },
  },
  methods: {
    loadFiles() {
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
    },
    goTo(path) {
      this.$router.push({ name: 'browser', query: { 'cd': path }})
    },
    getSelected() {
      return _.reduce(this.checked, function(result, value) {
        result.push(value)
        return result
      }, [])
    },
    itemClick(item) {
      if (item.type == 'dir' || item.type == 'back') {
        this.goTo(item.path)
      } else {
        this.download(item)
      }
    },
    selectDir() {
      this.$modal.open({
        parent: this,
        hasModalCard: true,
        component: Tree,
        events: {
          selected: dir => {
            this.goTo(dir.path)
          }
        },
      })
    },
    copy(event, item) {
      this.$modal.open({
        parent: this,
        hasModalCard: true,
        component: Tree,
        events: {
          selected: dir => {
            this.isLoading = true
            api.copyItems({
              destination: dir.path,
              items: item ? [item] : this.getSelected(),
            })
              .then(res => {
                this.isLoading = false
              })
              .catch(error => {
                this.isLoading = false
                this.handleError(error)
              })
            this.checked = []
          }
        },
      })
    },
    move(event, item) {
      this.$modal.open({
        parent: this,
        hasModalCard: true,
        component: Tree,
        events: {
          selected: dir => {
            this.isLoading = true
            api.moveItems({
              destination: dir.path,
              items: item ? [item] : this.getSelected(),
            })
              .then(res => {
                this.isLoading = false
              })
              .catch(error => {
                this.isLoading = false
                this.handleError(error)
              })
            this.checked = []
          }
        },
      })
    },
    batchDownload() {
      let items = this.getSelected()

      this.isLoading = true
      api.batchDownload({
        items: items,
      })
        .then(ret => {
          this.isLoading = false
          this.$dialog.alert({
            message: this.lang('Your file is ready'),
            confirmText: this.lang('Download'),
            onConfirm: () => {
              window.open(Vue.config.baseURL+'/batchdownload&uniqid='+ret.uniqid, '_blank')
            }
          })
        })
        .catch(error => {
          this.isLoading = false
          this.handleError(error)
        })
    },
    getDownloadLink(item) {
      return Vue.config.baseURL+'/download/'+btoa(item.path);
    },
    download(item) {
      window.open(this.getDownloadLink(item), '_blank')
    },
    search() {
      // TODO: create search logic
    },
    edit() {
      // TODO: create edit file logic
    },
    isArchive(item) {
      return item.type == 'file' && item.name.split('.').pop() == 'zip'
    },
    unzip(event, item) {
      this.$dialog.confirm({
        message: this.lang('Are you sure you want to do this?'),
        type: 'is-danger',
        cancelText: this.lang('Cancel'),
        confirmText: this.lang('Unzip'),
        onConfirm: () => {
          this.isLoading = true
          api.unzipItem({
            item: item.path,
            destination: this.$store.state.cwd.location,
          })
            .then(res => {
              this.isLoading = false
              this.loadFiles()
            })
            .catch(error => {
              this.isLoading = false
              this.handleError(error)
            })
          this.checked = []
        }
      })
    },
    zip(event, item) {
      this.$dialog.prompt({
        message: this.lang('Name'),
        cancelText: this.lang('Cancel'),
        confirmText: this.lang('Create'),
        inputAttrs: {
          value: this.$store.state.config.default_archive_name,
          placeholder: this.$store.state.config.default_archive_name,
          maxlength: 100,
          required: false,
        },
        onConfirm: (value) => {
          if (! value) {
            return;
          }
          this.isLoading = true
          api.zipItems({
            name: value,
            items: item ? [item] : this.getSelected(),
            destination: this.$store.state.cwd.location,
          })
            .then(ret => {
              this.isLoading = false
              this.loadFiles()
            })
            .catch(error => {
              this.isLoading = false
              this.handleError(error)
            })
          this.checked = []
        }
      })
    },
    rename(event, item) {
      this.$dialog.prompt({
        message: this.lang('New name'),
        cancelText: this.lang('Cancel'),
        confirmText: this.lang('Rename'),
        inputAttrs: {
          value: item ? item.name : this.getSelected()[0].name,
          maxlength: 100,
          required: false,
        },
        onConfirm: (value) => {
          this.isLoading = true
          api.renameItem({
            from: item.name,
            to: value,
            destination: this.$store.state.cwd.location,
          })
            .then(res => {
              this.isLoading = false
              this.loadFiles()
            })
            .catch(error => {
              this.isLoading = false
              this.handleError(error)
            })
          this.checked = []
        }
      })
    },
    create(type) {
      this.$dialog.prompt({
        cancelText: this.lang('Cancel'),
        confirmText: this.lang('Create'),
        inputAttrs: {
          placeholder: type == 'dir' ? 'MyFolder' : 'file.txt',
          maxlength: 100,
          required: false,
        },
        onConfirm: (value) => {
          this.isLoading = true
          api.createNew({
            type: type,
            name: value,
            destination: this.$store.state.cwd.location,
          })
          // TODO: cors is triggering this too early?
            .then(ret => {
              this.isLoading = false
              this.loadFiles()
            })
            .catch(error => {
              this.isLoading = false
              this.handleError(error)
            })
          this.checked = []
        }
      })
    },
    remove(event, item) {
      this.$dialog.confirm({
        message: this.lang('Are you sure you want to do this?'),
        type: 'is-danger',
        cancelText: this.lang('Cancel'),
        confirmText: this.lang('Delete'),
        onConfirm: () => {
          this.isLoading = true
          api.removeItems({
            items: item ? [item] : this.getSelected(),
          })
            .then(ret => {
              this.isLoading = false
              this.loadFiles()
            })
            .catch(error => {
              this.isLoading = false
              this.handleError(error)
            })
          this.checked = []
        }
      })
    },
    sortByName(a, b, isAsc) {
      return this.customSort(a, b, isAsc, 'name')
    },
    sortBySize(a, b, isAsc) {
      return this.customSort(a, b, isAsc, 'size')
    },
    sortByTime(a, b, isAsc) {
      return this.customSort(a, b, isAsc, 'time')
    },
    customSort(a, b, isAsc, param) {
      // TODO: firefox is broken
      if (b.type == 'back') return 1
      if (a.type == 'back') return -1
      if (b.type == 'dir' && a.type == 'dir') {
        return (a[param] < b[param]) || isAsc ? -1 : 1
      } else if (b.type == 'dir') {
        return 1
      } else if (a.type == 'dir') {
        return -1
      }
      if (_.isString(a[param])) {
        return (_.lowerCase(a[param]) < _.lowerCase(b[param])) || isAsc ? -1 : 1
      } else {
        return (a[param] < b[param]) || isAsc ? -1 : 1
      }
    },
  }
}
</script>

<style scoped>
#loading {
  width: 100%;
  height: 100%;
  position: fixed;
  z-index: 1000;
  top: 0;
  left: 0;
  user-drag: none; 
  user-select: none;
  -moz-user-select: none;
  -webkit-user-drag: none;
  -webkit-user-select: none;
  -ms-user-select: none;
}
#dropzone {
  padding: 0;
}
#browser {
  margin: 50px auto 100px auto;
}
.breadcrumb a {
  font-weight: bold;
}
.actions {
  min-height: 55px;
}
.actions a {
  margin: 0 15px 15px 0;
}
.file-row a {
  color: #373737;
}
.file-row.type-dir a.name {
  font-weight: bold
}
.action-padding {
  padding: 6px 12px;
}
.drop-info {
  margin: 20% auto;
}
</style>
