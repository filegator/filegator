import Vue from 'vue'
import moment from 'moment'
import store from '../store.js'
import api from '../api/api'
import { Base64 } from 'js-base64'
import _ from 'lodash'

import english from '../translations/english'
import spanish from '../translations/spanish'
import german from '../translations/german'
import indonesian from '../translations/indonesian'
import turkish from '../translations/turkish'
import lithuanian from '../translations/lithuanian'
import portuguese from '../translations/portuguese'
import dutch from '../translations/dutch'
import chinese from '../translations/chinese'
import bulgarian from '../translations/bulgarian'
import serbian from '../translations/serbian'
import french from '../translations/french'
import slovak from '../translations/slovak'
import polish from '../translations/polish'
import italian from '../translations/italian'
import korean from '../translations/korean'
import czech from '../translations/czech'
import galician from '../translations/galician'
import russian from '../translations/russian'
import hungarian from '../translations/hungarian'
import swedish from '../translations/swedish'
import japanese from '../translations/japanese'
import slovenian from '../translations/slovenian'
import hebrew from '../translations/hebrew'
import romanian from '../translations/romanian'
import arabic from '../translations/arabic'
import portuguese_br from '../translations/portuguese_br'
import persian from '../translations/persian'
import estonian from '../translations/estonian'
import ukrainian from '../translations/ukrainian'

const funcs = {
  methods: {

    /**
     * example:
     *      lang("{0} is dead, but {1} is alive! {0} {2}", "HTML", "HTML5")
     * output:
     *      HTML is dead, but HTML5 is alive! HTML {2}
     **/
    lang(term, ...rest) {

      let available_languages = {
        'english': english,
        'spanish': spanish,
        'german': german,
        'indonesian': indonesian,
        'turkish': turkish,
        'lithuanian': lithuanian,
        'portuguese': portuguese,
        'dutch': dutch,
        'chinese': chinese,
        'bulgarian': bulgarian,
        'serbian': serbian,
        'french': french,
        'slovak': slovak,
        'polish': polish,
        'italian': italian,
        'korean': korean,
        'czech': czech,
        'galician': galician,
        'russian': russian,
        'hungarian': hungarian,
        'swedish': swedish,
        'japanese': japanese,
        'slovenian': slovenian,
        'hebrew': hebrew,
        'romanian': romanian,
        'arabic': arabic,
        'portuguese_br': portuguese_br,
        'persian': persian,
        'estonian': estonian,
        'ukrainian': ukrainian,
      }

      let language = store.state.config.language

      let args = rest
      if(!available_languages[language] || available_languages[language][term] == undefined) {
        // translation required
        return term
      }
      return available_languages[language][term].replace(/{(\d+)}/g, function(match, number) {
        return typeof args[number] != 'undefined'
          ? args[number]
          : match
      })
    },
    is(role) {
      return this.$store.state.user.role == role
    },
    can(permissions) {
      return this.$store.getters.hasPermissions(permissions)
    },
    formatBytes(bytes, decimals = 2) {
      if (bytes === 0) return '0 Bytes'

      const k = 1024
      const dm = decimals < 0 ? 0 : decimals
      const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']

      const i = Math.floor(Math.log(bytes) / Math.log(k))

      return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
    },
    formatDate(timestamp) {
      return moment.unix(timestamp).format(store.state.config.date_format ? store.state.config.date_format : 'YY/MM/DD hh:mm:ss')
    },
    checkUser() {
      api.getUser()
        .then((user) => {
          if (user.username !== store.state.user.username) {
            this.$store.commit('destroyUser', user)
            this.$toast.open({
              message: this.lang('Please log in'),
              type: 'is-danger',
            })
          }
        })
        .catch(() => {
          this.$toast.open({
            message: this.lang('Please log in'),
            type: 'is-danger',
          })
        })
    },
    handleError(error) {
      this.checkUser()

      if (typeof error == 'string') {
        this.$toast.open({
          message: this.lang(error),
          type: 'is-danger',
          duration: 5000,
        })
        return
      } else if (error && error.response && error.response.data && error.response.data.data) {
        this.$toast.open({
          message: this.lang(error.response.data.data),
          type: 'is-danger',
          duration: 5000,
        })
        return
      }

      this.$toast.open({
        message: this.lang('Unknown error'),
        type: 'is-danger',
        duration: 5000,
      })
    },
    getDownloadLink(path) {
      return Vue.config.baseURL+'/download&path='+encodeURIComponent(Base64.encode(path))
    },
    hasPreview(name) {
      return this.isText(name) || this.isImage(name)
    },
    isText(name) {
      return this.hasExtension(name, store.state.config.editable)
    },
    isImage(name) {
      return this.hasExtension(name, ['.jpg', '.jpeg', '.gif', '.png', '.bmp', '.svg', '.tiff', '.tif'])
    },
    hasExtension(name, exts) {
      return !_.isEmpty(exts) && (new RegExp('(' + exts.join('|').replace(/\./g, '\\.') + ')$', 'i')).test(name)
    },
    capitalize(string) {
      return string.charAt(0).toUpperCase() + string.slice(1)
    },
  }
}

export default funcs
