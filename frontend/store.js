import Vue from 'vue'
import Vuex from 'vuex'
import shared from './mixins/shared'
import _ from 'lodash'

Vue.use(Vuex)

export default new Vuex.Store({
  state: {
    initialized: false,
    config: [],
    user: {
      role: 'guest',
      permissions: [],
      name: '',
      username: ''
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
      state.config = data
    },
    setUser(state, data) {
      state.user = data
    },
    destroyUser(state) {
      state.user = {
        role: 'guest',
        permissions: [],
        name: '',
        username: '',
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
