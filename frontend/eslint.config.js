import { defineConfigWithVueTs, vueTsConfigs } from "@vue/eslint-config-typescript";
import pluginVue from "eslint-plugin-vue";
import vueI18n from "@intlify/eslint-plugin-vue-i18n";
import prettier from "eslint-config-prettier";
import globals from "globals";

export default defineConfigWithVueTs(
  {
    languageOptions: {
      globals: globals.node,
      ecmaVersion: "latest",
      sourceType: "module",
    },
  },
  pluginVue.configs["flat/essential"],
  vueTsConfigs.recommended,
  ...vueI18n.configs["flat/recommended"],
  {
    settings: {
      "vue-i18n": {
        localeDir: "./src/i18n/en.json",
        messageSyntaxVersion: "^9.0.0",
      },
    },
    rules: {
      // --- preserved from original .eslintrc.json ---
      "no-unreachable": "off",
      "@intlify/vue-i18n/no-missing-keys": "error",
      "@intlify/vue-i18n/no-unused-keys": [
        "error",
        {
          src: "./src",
          extensions: [".js", ".vue", ".ts"],
        },
      ],
      "@intlify/vue-i18n/no-raw-text": [
        "error",
        {
          ignoreNodes: ["i", "v-icon"],
        },
      ],
      "@intlify/vue-i18n/no-missing-keys-in-other-locales": "off",
      "vue/multi-word-component-names": "off",
      "vue/no-mutating-props": [
        "error",
        {
          shallowOnly: true,
        },
      ],

      // --- rules new in upgraded packages not present in original config ---
      // Disable to avoid breaking existing codebase (can be re-enabled in follow-up cleanup)
      "vue/block-lang": "off",
      "vue/no-reserved-component-names": "off",
      "prefer-const": "off",
      "no-var": "off",
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/ban-ts-comment": "off",
      "@typescript-eslint/no-unused-vars": "off",
      "@typescript-eslint/no-unused-expressions": "off",
      "@typescript-eslint/no-empty-object-type": "off",
      "@typescript-eslint/no-this-alias": "off",
    },
  },
  prettier,
);
