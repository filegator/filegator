import Vue from 'vue'
import Vuex from 'vuex'
import shared from './mixins/shared'
import _ from 'lodash'

Vue.use(Vuex)

export default new Vuex.Store({
  state: {
    initialized: false,
    config: {
      pagination: ['', 5, 10, 15],
      password_reset_enabled: false,
      mfa_required_for_admins: true,
    },
    user: {
      role: 'guest',
      permissions: [],
      name: '',
      username: '',
      // Multi-folder fields. `homedirs` is the canonical list of folders
      // the user can access; for single-folder users it's a 1-element
      // array. `active_homedir` is the folder they're currently in
      // (selected via the picker, or auto-set at login for single-folder
      // users). Both come from the GET /getuser response.
      homedirs: [],
      active_homedir: null,
    },
    cwd: {
      location: '/',
      content: [],
    },
    tree: {},
  },
  getters: {
    hasPermissions: (state) => (permissions) => {
      if (_.isArray(permissions)) {
        return _.intersection(state.user.permissions, permissions).length == permissions.length
      }
      return _.find(state.user.permissions, p => p == permissions) ? true : false
    }
  },
  mutations: {
    initialize(state) {
      state.initialized = true
      this.commit('resetCwd')
      this.commit('resetTree')
      this.commit('destroyUser')
    },
    resetCwd(state) {
      state.cwd = {
        location: '/',
        content: [],
      }
    },
    resetTree(state) {
      state.tree = {
        path: '/',
        name: shared.methods.lang('Home'),
        children: [],
      }
    },
    setConfig(state, data) {
      state.config = {...state.config, ...data}
    },
    setUser(state, data) {
      // Defensive normalisation: backend may send either the new
      // `homedirs` array, the legacy `homedir` scalar, or both (during
      // the transition through Phase 10). Always populate the array
      // form for downstream consumers.
      const normalised = { ...data }
      if (!Array.isArray(normalised.homedirs)) {
        normalised.homedirs = normalised.homedir ? [normalised.homedir] : []
      }
      if (typeof normalised.active_homedir === 'undefined') {
        normalised.active_homedir = null
      }
      state.user = normalised
    },
    setActiveHomedir(state, path) {
      state.user.active_homedir = path
    },
    destroyUser(state) {
      state.user = {
        role: 'guest',
        permissions: [],
        name: '',
        username: '',
        homedirs: [],
        active_homedir: null,
      }
    },
    setCwd(state, data) {

      state.cwd.location = data.location
      state.cwd.content = []

      _.forEach(_.sortBy(data.content, [function(o) { return _.toLower(o.type) }]), (o) => {
        state.cwd.content.push(o)
      })

    },
    updateTreeNode(state, data) {
      let traverse = function (object) {
        for (let property in object) {
          if (object.hasOwnProperty(property)) {
            if (property === 'path' && object[property] === data.path) {
              Object.assign(object, {
                path: data.path,
                children: data.children,
              })
              return
            }
            if (typeof object[property] === 'object') {
              traverse(object[property])
            }
          }
        }
      }
      traverse(state.tree)
    },
  },
  actions: {
  }
})
