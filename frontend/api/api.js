import axios from 'axios'
import { Base64 } from 'js-base64'

const api = {
  getConfig() {
    return new Promise((resolve, reject) => {
      axios.get('getconfig')
        .then(res => resolve(res))
        .catch(error => reject(error))
    })
  },
  getUser() {
    return new Promise((resolve, reject) => {
      axios.get('getuser')
        .then(res => {
          // set/update csrf token
          axios.defaults.headers.common['x-csrf-token'] = res.headers['x-csrf-token']
          resolve(res.data.data)
        })
        .catch(error => reject(error))
    })
  },
  login(params) {
    return new Promise((resolve, reject) => {
      axios.post('login', {
        username: params.username,
        password: params.password,
      })
        .then(
          res => {
            resolve(res.data.data)
          },
          error => reject(error))
    })
  },
  logout() {
    return new Promise((resolve, reject) => {
      axios.post('logout')
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  changeDir(params) {
    return new Promise((resolve, reject) => {
      axios.post('changedir', {
        to: params.to,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  getDir(params) {
    return new Promise((resolve, reject) => {
      axios.post('getdir', {
        dir: params.dir,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  copyItems(params) {
    return new Promise((resolve, reject) => {
      axios.post('copyitems', {
        destination: params.destination,
        items: params.items,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  moveItems(params) {
    return new Promise((resolve, reject) => {
      axios.post('moveitems', {
        destination: params.destination,
        items: params.items,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  renameItem(params) {
    return new Promise((resolve, reject) => {
      axios.post('renameitem', {
        from: params.from,
        to: params.to,
        destination: params.destination,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  batchDownload (params) {
    return new Promise((resolve, reject) => {
      axios.post('batchdownload', {
        items: params.items,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  zipItems(params) {
    return new Promise((resolve, reject) => {
      axios.post('zipitems', {
        name: params.name,
        items: params.items,
        destination: params.destination,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  unzipItem(params) {
    return new Promise((resolve, reject) => {
      axios.post('unzipitem', {
        item: params.item,
        destination: params.destination,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  chmodItems(params) {
    return new Promise((resolve, reject) => {
      axios.post('chmoditems', {
        permissions: params.permissions,
        items: params.items,
        recursive: params.recursive,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  removeItems(params) {
    return new Promise((resolve, reject) => {
      axios.post('deleteitems', {
        items: params.items,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  createNew(params) {
    return new Promise((resolve, reject) => {
      axios.post('createnew', {
        type: params.type,
        name: params.name,
        destination: params.destination,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  listUsers() {
    return new Promise((resolve, reject) => {
      axios.get('listusers')
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  deleteUser(params) {
    return new Promise((resolve, reject) => {
      axios.post('deleteuser/'+params.username)
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  storeUser(params) {
    return new Promise((resolve, reject) => {
      axios.post('storeuser', {
        role: params.role,
        name: params.name,
        username: params.username,
        homedir: params.homedir,
        password: params.password,
        permissions: params.permissions,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  updateUser(params) {
    return new Promise((resolve, reject) => {
      axios.post('updateuser/'+params.key, {
        role: params.role,
        name: params.name,
        username: params.username,
        homedir: params.homedir,
        password: params.password,
        permissions: params.permissions,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  changePassword(params) {
    return new Promise((resolve, reject) => {
      axios.post('changepassword', {
        oldpassword: params.oldpassword,
        newpassword: params.newpassword,
      })
        .then(res => resolve(res.data.data))
        .catch(error => reject(error))
    })
  },
  downloadItem (params) {
    return new Promise((resolve, reject) => {
      axios.get('download&path='+encodeURIComponent(Base64.encode(params.path)),
        {
          transformResponse: [data => data],
        })
        .then(res => resolve(res.data))
        .catch(error => reject(error))
    })
  },
  saveContent (params) {
    return new Promise((resolve, reject) => {
      axios.post('savecontent', {
        name: params.name,
        content: params.content,
      })
        .then(res => resolve(res.data))
        .catch(error => reject(error))
    })
  },
}

export default api
