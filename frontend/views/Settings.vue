<template>
  <div class="container">
    <Menu />

    <section class="actions is-flex is-justify-between">
      <div>
        <h1 class="title is-4">{{ lang('Settings') }}</h1>
      </div>
    </section>

    <div class="columns">
      <div class="column is-one-third">
        <b-field :label="lang('App Name')">
          <b-input v-model="form.fc.app_name" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Language')" :message="lang('Language used across UI')">
          <b-select v-model="form.fc.language">
            <option v-for="l in availableLanguages" :key="l" :value="l">{{ capitalize(l) }}</option>
          </b-select>
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Logo URL')" :message="lang('Public URL to logo image')">
          <b-input v-model="form.fc.logo" />
        </b-field>
      </div>
    </div>

    <div class="columns">
      <div class="column is-one-third">
        <b-field :label="lang('Upload Max Size (bytes)')" :message="lang('Maximum upload size in bytes') + ' — ' + toMB(form.fc.upload_max_size)">
          <b-input type="number" v-model.number="form.fc.upload_max_size" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Upload Chunk Size (bytes)')" :message="lang('Chunk size in bytes for resumable uploads') + ' — ' + toMB(form.fc.upload_chunk_size)">
          <b-input type="number" v-model.number="form.fc.upload_chunk_size" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Upload Simultaneous')" :message="lang('Number of parallel upload requests')">
          <b-input type="number" v-model.number="form.fc.upload_simultaneous" />
        </b-field>
      </div>
    </div>

    <div class="columns">
      <div class="column is-one-third">
        <b-field :label="lang('Overwrite on Upload')" :message="lang('Replace existing files when uploading')">
          <b-switch v-model="form.root.overwrite_on_upload" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Date Format')" :message="lang('Moment.js format, e.g. YY/MM/DD hh:mm:ss')">
          <b-input v-model="form.fc.date_format" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Guest Redirection')" :message="lang('URL to redirect guest users')">
          <b-input v-model="form.fc.guest_redirection" />
        </b-field>
      </div>
    </div>

    <div class="columns">
      <div class="column is-one-third">
        <b-field :label="lang('Search Simultaneous')" :message="lang('Concurrent search requests')">
          <b-input type="number" v-model.number="form.fc.search_simultaneous" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Direct Download in Search')" :message="lang('Enable direct file download from search results')">
          <b-switch v-model="form.fc.search_direct_download" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Timezone')" :message="lang('PHP timezone identifier')">
          <b-input v-model="form.root.timezone" />
        </b-field>
      </div>
    </div>

    <div class="columns">
      <!-- <div class="column is-one-third">
        <b-field :label="lang('Lockout Attempts')" :message="lang('Failed login attempts before IP lockout')">
          <b-input type="number" v-model.number="form.root.lockout_attempts" />
        </b-field>
      </div> -->
      <!-- <div class="column is-one-third">
        <b-field :label="lang('Lockout Timeout (s)')" :message="lang('Lockout duration in seconds')">
          <b-input type="number" v-model.number="form.root.lockout_timeout" />
        </b-field>
      </div> -->
      <div class="column is-one-third">
        <b-field :label="lang('Download Inline Extensions (comma)')" :message="lang('Extensions to open inline in browser')">
          <b-input v-model="download_inline_text" />
        </b-field>
      </div>
    </div>

    <div class="columns">
      <div class="column is-one-third">
        <b-field :label="lang('Editable Extensions (comma)')" :message="lang('Extensions editable in the web editor')">
          <b-input v-model="editable_text" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Filter Entries (comma)')" :message="lang('Names to exclude from listings')">
          <b-input v-model="filter_entries_text" />
        </b-field>
      </div>
      <div class="column is-one-third">
        <b-field :label="lang('Pagination (comma)')" :message="lang('Items per page options')">
          <b-input v-model="pagination_text" />
        </b-field>
      </div>
    </div>

    <div class="columns">
      <div class="column">
        <b-field :label="lang('Custom CSS')" :message="lang('Injected last to override existing styles')">
          <b-input type="textarea" v-model="form.fc.custom_css" rows="6" />
        </b-field>
      </div>
    </div>
    <div class="columns">
      <div class="column">
        <b-field :label="lang('Custom JS')" :message="lang('Runs after app scripts load')">
          <b-input type="textarea" v-model="form.fc.custom_js" rows="6" />
        </b-field>
      </div>
    </div>

    <div class="buttons">
      <b-button type="is-primary" @click="save">{{ lang('Save') }}</b-button>
      <b-button @click="reset">{{ lang('Reset') }}</b-button>
    </div>
  </div>
</template>

<script>
import Menu from './partials/Menu'
import api from '../api/api'

export default {
  name: 'Settings',
  components: { Menu },
  data() {
    return {
      form: {
        fc: {
          app_name: '',
          language: 'english',
          logo: '',
          upload_max_size: 0,
          upload_chunk_size: 0,
          upload_simultaneous: 0,
          default_archive_name: 'archive.zip',
          editable: [],
          date_format: '',
          guest_redirection: '',
          search_simultaneous: 0,
          search_direct_download: false,
          filter_entries: [],
          pagination: [],
          custom_css: '',
          custom_js: '',
        },
        root: {
          overwrite_on_upload: false,
          timezone: 'UTC',
          download_inline: [],
          lockout_attempts: 5,
          lockout_timeout: 15,
        },
      },
      editable_text: '',
      filter_entries_text: '',
      pagination_text: '',
      download_inline_text: '',
      original: null,
      availableLanguages: ['english','spanish','german','indonesian','turkish','lithuanian','portuguese','dutch','chinese','bulgarian','serbian','french','slovak','polish','italian','korean','czech','galician','russian','hungarian','swedish','japanese','slovenian','hebrew','romanian','arabic','portuguese_br','persian','estonian','ukrainian'],
    }
  },
  mounted() {
    api.getAdminFrontendConfig()
      .then(cfg => {
        this.form.fc = Object.assign({}, cfg.frontend_config)
        this.form.root = Object.assign({}, cfg.root_config)

        this.editable_text = (this.form.fc.editable || []).join(', ')
        this.filter_entries_text = (this.form.fc.filter_entries || []).join(', ')
        this.pagination_text = (this.form.fc.pagination || []).join(', ')
        this.download_inline_text = (this.form.root.download_inline || []).join(', ')

        this.original = JSON.parse(JSON.stringify(this.form))
      })
      .catch(error => this.handleError(error))
  },
  methods: {
    toMB(bytes) {
      const n = Number(bytes) || 0
      return (n / 1048576).toFixed(2) + ' MB'
    },
    save() {
      const toArray = (text) => text
        .split(',')
        .map(v => v.trim())
        .filter(v => v.length > 0)

      this.form.fc.editable = toArray(this.editable_text)
      this.form.fc.filter_entries = toArray(this.filter_entries_text)
      this.form.fc.pagination = toArray(this.pagination_text).map(v => (isNaN(v) ? v : Number(v)))
      this.form.root.download_inline = toArray(this.download_inline_text)

      api.updateAdminFrontendConfig({
        frontend_config: this.form.fc,
        root_config: this.form.root,
      })
        .then(cfg => {
          // backend returns updated frontend_config; merge root changes in store too
          const newConfig = Object.assign({}, this.$store.state.config, cfg)
          this.$store.commit('setConfig', newConfig)

          const rtlLangs = ['arabic', 'hebrew', 'persian']
          document.documentElement.setAttribute('dir', rtlLangs.includes(this.form.fc.language) ? 'rtl' : 'ltr')

          this.$toast.open({
            message: this.lang('Saved'),
            type: 'is-success',
          })
        })
        .catch(error => this.handleError(error))
    },
    reset() {
      if (this.original) {
        this.form = JSON.parse(JSON.stringify(this.original))
        this.editable_text = (this.form.fc.editable || []).join(', ')
        this.filter_entries_text = (this.form.fc.filter_entries || []).join(', ')
        this.pagination_text = (this.form.fc.pagination || []).join(', ')
        this.download_inline_text = (this.form.root.download_inline || []).join(', ')
      }
    },
  }
}
</script>

<style scoped>
.actions {
  margin: 50px 0 30px 0;
}
.buttons {
  margin-top: 20px;
}
.card + .card {
  margin-top: 1rem;
}
</style>
