// https://docs.cypress.io/guides/guides/plugins-guide.html

// if you need a custom webpack configuration you can uncomment the following import
// and then use the `file:preprocessor` event
// as explained in the cypress docs
// https://docs.cypress.io/api/plugins/preprocessors-api.html#Examples

/* eslint-disable import/no-extraneous-dependencies, global-require, arrow-body-style */
// const webpack = require('@cypress/webpack-preprocessor')

module.exports = (on, config) => {
  // on('file:preprocessor', webpack({
  //  webpackOptions: require('@vue/cli-service/webpack.config'),
  //  watchOptions: {}
  // }))

  return Object.assign({}, config, {
    fixturesFolder: 'tests/frontend/e2e/fixtures',
    integrationFolder: 'tests/frontend/e2e/specs',
    screenshotsFolder: 'tests/frontend/e2e/screenshots',
    videosFolder: 'tests/frontend/e2e/videos',
    supportFile: 'tests/frontend/e2e/support/index.js'
  })
}
