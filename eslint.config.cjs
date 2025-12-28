const tsParser = require("@typescript-eslint/parser");
const tsPlugin = require("@typescript-eslint/eslint-plugin");
const security = require("eslint-plugin-security");

const sharedRules = {
    "no-var": "error",
    "prefer-const": "error",
    "no-prototype-builtins": "off",
    "no-restricted-syntax": [
        "error",
        {
            selector:
                "MemberExpression[object.name='String'][property.name='prototype']",
            message:
                "Modifying String.prototype is not allowed. Use utility functions instead."
        }
    ]
};

module.exports = [
    // Shared ignores
    {
        ignores: [
            "node_modules/**",
            "dist/**",
            "static/**/*.min.js",
            "static/**/vendor/**"
        ]
    },

    // JavaScript files
    {
        files: ["**/*.js"],

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
            security
        },

        rules: sharedRules
    },

    // TypeScript files
    {
        files: ["**/*.ts"],

        languageOptions: {
            parser: tsParser,
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
            security,
            "@typescript-eslint": tsPlugin
        },

        rules: {
            ...sharedRules,
            ...tsPlugin.configs.recommended.rules
        }
    }
];
