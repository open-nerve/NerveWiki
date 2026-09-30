# M0/P5/S3 UI 基座与主题：实施计划

上级：[P5 文档](../05-P5-web-shell.md) 3.5。

## 任务

1. Tailwind CSS 4（`@tailwindcss/vite`）；`src/styles.css`：Tailwind 入口、shadcn/ui 语义色的 CSS 变量（`:root` 浅色、`.dark` 深色）、系统字体栈。
2. `lib/cn.ts`；`components/ui/` 按 shadcn/ui 的写法加入用到的组件（`Button`、`DropdownMenu`）。
3. `PreferencesStore` 的主题部分：`system` / `light` / `dark`，存储与媒体查询从构造函数传入；解析后的主题。
4. `public/theme-init.js`：首次绘制前按同一个存储键设置 `dark` 类；`index.html` 在 `<head>` 中同步加载它。
5. 布局外壳 `app/layout.tsx`：顶栏（产品名、主题切换），`<Outlet>`；把解析后的主题写到 `<html>` 的 `observer` 组件。

## 测试

- `PreferencesStore`：默认跟随系统；保存与读取；系统主题变化时解析结果跟着变；存储不可用时退回默认。
- `theme-init.js` 与 store 用同一个存储键，解析规则一致（在 jsdom 中执行这个脚本）。
- 布局渲染出顶栏与子页面；切换主题后 `<html>` 的类随之改变。

## 完成检查

`make check` 为绿；`make build-web` 通过。
