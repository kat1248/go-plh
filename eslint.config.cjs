module.exports = {
  ignores: ["node_modules/**", "dist/**", "static/**/*.min.js", "static/**/vendor/**"],
  languageOptions: {
    ecmaVersion: 2021,
    sourceType: "module",
    globals: {
      window: "readonly",
      document: "readonly",
      jQuery: "readonly",
      $: "readonly"
    }
  },
  plugins: {
    security: require("eslint-plugin-security")
  },
  rules: {
    "no-var": "error",
    "prefer-const": "error",
    "no-prototype-builtins": "off",
    "no-restricted-syntax": [
      "error",
      {
        selector: "MemberExpression[object.name='String'][property.name='prototype']",
        message: "Modifying String.prototype is not allowed. Use utility functions instead."
      }
    ]
  }
};
