// Base ESLint flat config for JavaScript and TypeScript projects.
// Requires these dependencies:
//
//   npm install -D eslint @eslint/js typescript-eslint \
//     eslint-config-prettier eslint-plugin-prettier prettier

import eslint from '@eslint/js';
import tseslint from 'typescript-eslint';
import prettierConfig from 'eslint-config-prettier';
import pluginPrettier from 'eslint-plugin-prettier';

export default tseslint.config(
  // --- Ignores ---
  {
    ignores: ['**/node_modules/', '**/dist/', '**/build/'],
  },

  // --- Base: all JS/TS files ---
  eslint.configs.recommended,
  prettierConfig,
  {
    plugins: {
      prettier: pluginPrettier,
    },
    rules: {
      'prettier/prettier': 'error',
      'block-scoped-var': 'error',
      eqeqeq: 'error',
      'no-var': 'error',
      'prefer-const': 'error',
      'eol-last': 'error',
      'prefer-arrow-callback': 'error',
      'no-trailing-spaces': 'error',
      quotes: ['warn', 'single', {avoidEscape: true}],
      'no-restricted-properties': [
        'error',
        {object: 'describe', property: 'only'},
        {object: 'it', property: 'only'},
      ],
    },
  },

  // --- TypeScript files ---
  {
    files: ['**/*.ts', '**/*.tsx'],
    extends: [tseslint.configs.recommended],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        projectService: true,
      },
    },
    rules: {
      // Enforce T[] for simple types, Array<T> for complex types.
      '@typescript-eslint/array-type': ['error', {default: 'array-simple'}],
      // Warn on @ts-ignore / @ts-expect-error.
      '@typescript-eslint/ban-ts-comment': 'warn',
      // Catch unhandled promises.
      '@typescript-eslint/no-floating-promises': 'error',

      // --- Relaxed rules ---
      // Non-null assertions (x!) are sometimes necessary.
      '@typescript-eslint/no-non-null-assertion': 'off',
      // Hoisting is fine with function declarations.
      '@typescript-eslint/no-use-before-define': 'off',
      // Allow TODO/FIXME comments.
      '@typescript-eslint/no-warning-comments': 'off',
      // Empty functions are fine (e.g., no-op callbacks).
      '@typescript-eslint/no-empty-function': 'off',
      // Don't require explicit return types — inference is usually sufficient.
      '@typescript-eslint/explicit-function-return-type': 'off',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      // Allow empty object types for generics.
      '@typescript-eslint/no-empty-object-type': 'off',
    },
  },
);
