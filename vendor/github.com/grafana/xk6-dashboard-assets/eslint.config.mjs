import prettier from "eslint-plugin-prettier"
import globals from "globals"
import typescriptEslint from "@typescript-eslint/eslint-plugin"
import tsParser from "@typescript-eslint/parser"
import react from "eslint-plugin-react"
import reactRefresh from "eslint-plugin-react-refresh"
import js from "@eslint/js"
import { FlatCompat } from "@eslint/eslintrc"

const compat = new FlatCompat({
  recommendedConfig: js.configs.recommended
})

export default [
  {
    ignores: ["**/dist", "**/node_modules"]
  },
  {
    files: ["**/*.{js,ts,tsx,jsx}"],
    plugins: {
      prettier
    },
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node
      },
      ecmaVersion: "latest",
      sourceType: "module"
    },
    rules: {
      "prettier/prettier": "error"
    }
  },
  {
    files: ["packages/{model,view}/**/*.ts"],
    ...compat.extends("eslint:recommended", "plugin:@typescript-eslint/recommended", "prettier")[0],
    plugins: {
      "@typescript-eslint": typescriptEslint,
      prettier
    },
    languageOptions: {
      parser: tsParser
    },
    rules: {
      "prettier/prettier": "error",
      "@typescript-eslint/no-unused-vars": "error",
      "@typescript-eslint/consistent-type-definitions": ["error", "type"]
    }
  },
  {
    files: ["packages/{report,ui}/**/*.{ts,tsx,js,jsx}"],
    ...compat.extends(
      "eslint:recommended",
      "plugin:react/recommended",
      "plugin:@typescript-eslint/recommended",
      "prettier"
    )[0],
    plugins: {
      react,
      prettier
    },
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaFeatures: {
          jsx: true
        }
      }
    },
    settings: {
      react: {
        version: "detect"
      }
    },
    rules: {
      "prettier/prettier": "error",
      "react/prop-types": 0,
      "@typescript-eslint/no-empty-object-type": "off"
    }
  },
  {
    files: ["packages/ui/**/*.{ts,tsx,js,jsx}"],
    ...compat.extends("plugin:react-hooks/recommended")[0],
    plugins: {
      "react-refresh": reactRefresh
    },
    rules: {
      "react-refresh/only-export-components": ["warn", { allowConstantExport: true }]
    }
  }
]
