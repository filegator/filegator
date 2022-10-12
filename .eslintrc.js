module.exports = {
  extends: [
    // add more generic rulesets here, such as:
    'eslint:recommended',
    'plugin:vue/recommended',
  ],
  rules: {
    // override/add rules settings here, such as:
    'no-console': 0,
    'no-unused-vars': 'error',
    'vue/require-prop-types': 0,
    'vue/max-attributes-per-line': 4,
    'vue/attributes-order': 0,
    'semi': [
      'error',
      'never'
    ],
    'linebreak-style': [
      'error',
      'unix'
    ],
    'quotes': [
      'error',
      'single'
    ],
    'no-trailing-spaces': 1,
  }
}
