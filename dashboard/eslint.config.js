import js from '@eslint/js';
import vue from 'eslint-plugin-vue';
import globals from 'globals';

export default [
  // Ignore patterns
  {
    ignores: [
      'node_modules/**',
      'dist/**',
      '*.log',
      '.vscode/**',
      '.idea/**',
      '*.swp',
      '*.swo',
      '.DS_Store',
      'Thumbs.db',
      '.env.local',
      '.env.*.local',
    ],
  },
  // Base JavaScript config
  js.configs.recommended,
  // Vue plugin config
  ...vue.configs['flat/recommended'],
  // Custom rules
  {
    files: ['**/*.{js,mjs,cjs,jsx,vue}'],
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
      },
      ecmaVersion: 2022,
      sourceType: 'module',
    },
    rules: {
      // Add custom rules here if needed
    },
  },
];

