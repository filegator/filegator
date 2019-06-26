module.exports = {
  extends: [
    // add more generic rulesets here, such as:
    'eslint:recommended',
    'plugin:vue/recommended'
  ],
  rules: {
    // override/add rules settings here, such as:
    'no-unused-vars': 'error',
    'vue/require-prop-types': 0,
    'vue/max-attributes-per-line': 4
  }
}
