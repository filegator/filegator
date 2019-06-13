module.exports = {
  indexPath: 'main.html',
  filenameHashing: false,
  configureWebpack: config => {
    config.entry = {
      app: [
        './frontend/main.js'
      ]
    }
  }
}
