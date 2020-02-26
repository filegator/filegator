<template>
  <div class="modal-card">
    <header class="modal-card-head">
      <p class="modal-card-title">
        {{ lang('Search') }}
      </p>
    </header>
    <section class="modal-card-body">
      <b-input ref="input" v-model="term" @input="searchFiles" :placeholder="lang('Name')" />
      <br>
      <b-loading :is-full-page="false" :active.sync="searching" />
      <ul ref="results">
        <li v-for="(item, index) in results" :key="index">
          <a @click="select(item)">{{ item.file.path }}</a>
        </li>
      </ul>
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
import _ from 'lodash'

export default {
  name: 'Search',
  data() {
    return {
      active: false,
      searching: false,
      term: '',
      results: [],
    }
  },
  mounted() {
    this.active = true
    this.searching = false
    this.$refs.input.focus()
  },
  beforeDestroy() {
    this.active = false
    this.searching = false
  },
  methods: {
    select(item) {
      this.$emit('selected', item)
      this.$parent.close()
    },
    searchFiles: _.debounce(function(val) {
      this.results = []
      if (val.length > 0) {
        this.searching = true
        this.getDir('/')
      }
    }, 1000),
    getDir(path) {
      if (!this.active) return
      this.searching = true
      api.getDir({
        dir: path
      })
        .then(ret => {
          this.searching = false
          _.forEach(ret.files, item => {
            if (item.name.toLowerCase().indexOf(this.term.toLowerCase()) > -1) {
              this.results.push({
                file: item,
                dir: path,
              })
            }
          })
          _.forEach(_.filter(ret.files, ['type', 'dir']), subdir => {
            this.getDir(subdir.path)
          })
        })
        .catch(error => this.handleError(error))

    },
  },
}
</script>

<style>
</style>
