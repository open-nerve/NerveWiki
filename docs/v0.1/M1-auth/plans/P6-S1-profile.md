# M1/P6/S1 设置的布局、资料与偏好：实施计划

上级：[P6 文档](../06-P6-settings.md) 3.2、3.3。

## 任务

1. `routes.tsx`：`/settings`（转到 `/settings/profile`）与三个子页，挂在 `Onboarded` 之下；子页按需加载。S1 只实现资料页，安全与令牌两页先放占位的标题（S2、S3 替换）。
2. `pages/settings/settings-layout.tsx`：导航（`NavLink`，`aria-current`）与 `Outlet`；宽屏左侧、窄屏上方。
3. `app/user-menu.tsx`：加"设置"（`DropdownMenuItem asChild` 包 `Link`）。
4. `pages/settings/profile-page.tsx`：显示名的表单（本地非空、422 在字段下方、已保存、没改不发请求）；邮箱只读与说明；偏好的两个单选组（`preferences.setTheme`、`setLocale`）。
5. 文案（`en`、`zh-CN`）。

## 测试

- 路由：未完成引导访问 `/settings/profile` 去 `/onboarding?next=…`；`/settings` 转到资料；导航的当前页。
- 用户菜单的"设置"到资料页。
- 资料：空名本地拦下；422 在字段下方并得到焦点；保存之后"已保存"与用户菜单的名字；没改不发请求。
- 偏好：选深色，`preferences.theme` 与顶栏菜单一致，写入存储；选简体中文，界面文字随之改变。

## 完成检查

`make check` 为绿。
