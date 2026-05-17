module.exports = {
  indexPath: 'main.html',
  filenameHashing: false,
  css: {
	extract: true
  },
  // Proxy API calls to the PHP backend so the SPA and the API share the same
  // origin during dev. Without this, axios POSTs from :8080 to :8081 are
  // cross-origin and Firefox refuses to send the session cookie on the second
  // request — which silently breaks the two-step MFA login flow (the
  // /login/mfa POST lands on a fresh session and reports "MFA challenge
  // expired or missing"). The PHP server routes everything through one front
  // controller, so a blanket proxy on the ?r= query is enough.
  devServer: {
    proxy: {
      '^/\\?r=': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
  configureWebpack: config => {
    config.entry = {
      app: [
        './frontend/main.js'
      ]
    }
  }
}
