import js from '@eslint/js';
import vue from 'eslint-plugin-vue';
import globals from 'globals';
import tseslint from 'typescript-eslint';

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
  // TypeScript configs
  ...tseslint.configs.recommended,
  // Vue plugin config
  ...vue.configs['flat/recommended'],
  // Custom rules for JavaScript/TypeScript
  {
    files: ['**/*.{js,mjs,cjs,jsx,ts,tsx,vue}'],
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
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/no-explicit-any': 'warn',
    },
  },
];

