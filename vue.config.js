module.exports = {
  indexPath: 'main.html',
  filenameHashing: false,
  css: {
	extract: true
  },
  configureWebpack: config => {
    config.entry = {
      app: [
        './frontend/main.js'
      ]
    }
  }
}
