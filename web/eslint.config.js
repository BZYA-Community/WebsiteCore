import pluginVue from 'eslint-plugin-vue';
// ESLint flat config — 强度等级: 中等
// Vue: strongly-recommended (base < essential < strongly-recommended < recommended 中取中档)
// TS:  recommended (非 strict)
import globals from 'globals';
import tseslint from 'typescript-eslint';
import vueParser from 'vue-eslint-parser';

export default tseslint.config(
  {
    ignores: [
      'dist/**',
      'node_modules/**',
      'src/components.d.ts', // unplugin-vue-components 生成
      '**/*.d.ts', // 类型声明文件不做风格约束
    ],
  },
  ...pluginVue.configs['flat/strongly-recommended'],
  ...tseslint.configs.recommended,
  {
    files: ['**/*.vue'],
    languageOptions: {
      // 顶层解析器必须是 vue-eslint-parser(tseslint 的 base 会把 .vue 也抢成 ts parser)
      parser: vueParser,
      parserOptions: {
        // <script lang="ts"> 走 TS parser
        parser: tseslint.parser,
      },
    },
  },
  {
    files: ['**/*.{js,ts,vue}'],
    languageOptions: {
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      // 中档定制: 仓库存量代码较多, 以下规则降级为 warn 不阻塞
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': [
        'warn',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      'vue/multi-word-component-names': 'off', // 页面多为单词命名(Messages/Home 等), 中档不强制
      'vue/no-v-html': 'warn',
      // 纯排版类规则交还给 biome(format 职责), 不在 ESLint 里重复报
      'vue/html-indent': 'off',
      'vue/max-attributes-per-line': 'off',
      'vue/singleline-html-element-content-newline': 'off',
      'vue/multiline-html-element-content-newline': 'off',
      'vue/html-closing-bracket-newline': 'off',
      'vue/html-closing-bracket-spacing': 'off',
      'vue/html-self-closing': 'off',
      'vue/first-attribute-linebreak': 'off',
      'vue/mustache-interpolation-spacing': 'off',
      'vue/no-multi-spaces': 'off',
      'vue/html-quotes': 'off',
    },
  },
);
