import js from '@eslint/js';
import pluginVue from 'eslint-plugin-vue';
import vueTsEslintConfig from '@vue/eslint-config-typescript';
import prettierConfig from '@vue/eslint-config-prettier';

export default [
  {
    name: 'app/files-to-lint',
    files: ['**/*.{ts,mts,tsx,vue}'],
  },

  {
    name: 'app/files-to-ignore',
    ignores: [
      '**/dist/**',
      '**/dist-ssr/**',
      '**/coverage/**',
      '**/.quasar/**',
    ],
  },

  {
    rules: {
      languageOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',

        globals: {
          ...globals.browser,
          ...globals.node, // SSR, Electron, config files
          process: 'readonly', // process.env.*
          ga: 'readonly', // Google Analytics
          cordova: 'readonly',
          Capacitor: 'readonly',
          chrome: 'readonly', // BEX related
          browser: 'readonly', // BEX related
        },
      },

      // Custom rules can be added here
      'prefer-promise-reject-errors': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },

  js.configs.recommended,
  ...pluginVue.configs['flat/essential'],
  ...vueTsEslintConfig(),
  prettierConfig,
];
