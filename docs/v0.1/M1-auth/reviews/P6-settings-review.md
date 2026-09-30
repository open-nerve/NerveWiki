# M1/P6 前端个人设置：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m1-p6-settings`（`main...e1aa1d9`，S1–S4），对照 [06-P6-settings.md](../06-P6-settings.md)、各 Step 计划与 [M1 总设计](../00-M1-design.md) |
| 审查方式 | 独立审查者在工作树上实测：<br>• 门禁：`make check`（vitest 401 个）、`make e2e`（47 个）；A7、A8、A10、A11 的页面版本另跑 `--repeat-each 8`，48 次全部通过；<br>• 用 Chromium 与临时的探测故事（已删除）核对：令牌视图点外部不关闭；创建发出之后按 Esc 取消，迟到的答复在页面上找不到令牌、列表有它；连点两下"停用"只发一个 POST，另一个标签页随之到 `/sign-in?next=%2Fsettings%2Ftokens`；撤销之后的焦点；<br>• 反向对照：对话框关闭之后仍挂载，两个令牌测试失败；另有四处没被抓住（M3）；<br>• 在 `America/New_York`、`America/Los_Angeles`、`Pacific/Kiritimati` 下跑 vitest（I1） |
| 日期 | 2026-10-01 |
| 结论 | 修复后合并。令牌的秘密只在对话框内容的 state 里，关闭即卸载，单元测试与 A10（DOM、存储、地址、控制台、剪贴板读回）守着；停用之后 `endSession(loginId)`、不发 logout，多标签页与双击实测正确；`useForm` 收拢了四个表单，P5 的焦点与上方错误的规则不变；`ApiTokenStore` 的读写交错沿用 `AccountStore` 的办法；契约核对扩展到 5 个操作。<br>1 项 Important、3 项 Minor、10 项 Nit：全部在合并前处理；审查者列出的"偏好写入存储没有单元测试"不另加（理由见下） |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| I1 | Important | 单元测试依赖本机时区：`formatDate`、`formatDateTime` 按宿主时区输出，令牌列表的测试写死了 `Jan 1, 2099` 等日期。在美洲时区下 401 个里有 1 个失败（"Expires Dec 31, 2098"）；持续集成是 UTC、本机是 Asia/Shanghai，没有暴露。美洲的开发者本地跑 `make check`（每步的门禁）必然是红的 | vitest 的 `test.env` 固定 `TZ: "UTC"`（注释说明页面按浏览器时区写日期）。`format.test.ts` 的时间之前的空格用 `\s`，不同 ICU 版本的普通空格与窄不换行空格都匹配。反向对照：去掉 `TZ`，在 `TZ=America/Los_Angeles` 下令牌列表的日期测试失败；留着它，在同一时区下全部通过 |
| M1 | Minor | 撤销令牌之后焦点落到 `<body>`：行与它的对话框一起卸载，Radix 要还给的触发按钮已经不在。键盘与读屏用户下一次 Tab 从页首开始（WCAG 2.4.3） | 抽出 `ConfirmDialog`（见 N2），带可选的 `focusAfter`：确认成功、对话框关闭时由它决定焦点去处。令牌页的标题 `tabIndex=-1`，撤销之后聚焦它。测试断言撤销之后 `activeElement` 是"访问令牌"标题。反向对照：去掉 `focusAfter`，测试失败 |
| M2 | Minor | "已保存"、"密码已修改"、"已复制"的 `<output>`（`role=status`）带着文字一起插进 DOM：live region 要先在 DOM 里，内容变化才能可靠地读出；第二次复制的文字不变，也不会再读。这几处是成功时唯一的反馈 | 三处的 `<output>` 一直挂载，只切换文字；复制时先清空再写"已复制"。测试改为断言状态的文字（没有结果时为空） |
| M3 | Minor | 四处改动全部测试仍然通过：去掉令牌视图的 `onInteractOutside`；`"clipboard" in navigator` 改成 `true`；去掉确认框 `onOpenChange` 的 `if (!sending)`；一年改成 366 天。另外令牌列表的加载失败与"重试"、A8 注释所说的"与顶栏的菜单一致"都没有测试 | 加测试：令牌视图点外部不关闭；没有 `navigator.clipboard` 时没有"复制"，令牌字段得到焦点并全选；发送中按 Esc，确认框保持打开；30 天、90 天（默认）、一年各自的到期时间；列表加载失败显示错误、点"重试"之后显示列表；A8 刷新之后打开顶栏的主题菜单，"深色"选中。反向对照：四处改动、去掉"重试"的 `mutate`、顶栏菜单不跟随 store（端到端），对应的测试都失败 |
| N1 | Nit | 创建成功之后焦点在对话框容器上，没有"复制"按钮时（局域网 HTTP）用户还得先点令牌字段 | 令牌字段挂载时得到焦点，聚焦即全选，Ctrl+C 立即可用；测试断言焦点与选区 |
| N2 | Nit | `DeactivateDialog` 与 `RevokeTokenDialog` 是同一套"确认、发送中、失败"的逻辑，约 40 行重复；M3 里那个没测到的保护有两份 | `app/confirm-dialog.tsx`（用 `errorText`，所以在 `app/` 而不在 `components/`）：两处都用它，保护只剩一份 |
| N3 | Nit | 引导的"继续"按钮不再全宽（它进了 `DisplayNameForm` 的一行），登录、注册的按钮仍是全宽；设计没写这个变化 | `DisplayNameForm` 加 `wide`：引导一步的按钮全宽，设置页的在"已保存"旁边 |
| N4 | Nit | `DisplayNameForm` 的 `className` 没有人用 | 随 N3 换成 `wide` |
| N5 | Nit | `theme-menu.tsx` 导出主题的选项与 `isThemePreference` 给设置页用；它们是偏好的数据 | 移到 `stores/preferences.store.ts`（`themePreferences`、`ThemePreference`、`isThemePreference`），与 `i18n/locale.ts` 的 `locales`、`isLocale` 对应；顶栏菜单与设置页都从那里取，store 读存储时也用它 |
| N6 | Nit | 缺 `field.name.invalid_format`：令牌名称含控制字符时显示通用的"格式不对"，而显示名的是"不能有控制字符" | 加上，两种语言 |
| N7 | Nit | `user-menu.tsx` 的一段注释有一行 97 列 | 重新折行 |
| N8 | Nit | 端到端的 `answerTo`、`noteOf` 是通用的，登录与引导的页面夹具里有同样的 `waitForResponse` 判断；`holdAnswer` 的注释说"下一个"答复，实际扣住之后所有匹配的请求 | 两个函数移到 `fixtures/browser.ts`，登录与引导的夹具也用 `answerTo`；注释改为"扣住每一个匹配的答复，直到放行" |
| N9 | Nit | zh-CN 的措辞："现在复制它：它不会再显示"、"失去访问……这不能撤回"、"以你的账户行事" | 改为"请现在复制，关闭之后不会再显示。"（与设计一致；英文同步为 "once closed, it will not be shown again"）、"失去访问权限……此操作无法撤销"、"以你的身份调用接口" |
| N10 | Nit | 改密码的表单没有 `autocomplete="username"` 的字段，密码管理器不知道更新哪个条目 | 加一个隐藏的只读字段，值为邮箱 |

## 对有意决定的判断

| 决定 | 判断 |
|---|---|
| 令牌的秘密只在对话框内容的 state 里，store 只存不含秘密的字段；对话框关闭即卸载 | 同意；迟到的答复写到已卸载的组件上，什么也不显示，不需要 Nerve 的代数计数与延时清理。反向对照按预期失败 |
| 停用之后 `endSession(loginId)`，不发 logout | 同意；在锁里执行，记录已经是别的会话的就跟随它；多标签页、双击只发一次实测通过 |
| `useForm` 收拢登录、注册、个人资料与设置的四个表单 | 同意；P5 的焦点、上方错误的规则、发送中禁用都不变 |
| `formErrors` 的 `onField` 把 problem 码放到字段下方 | 同意 |
| `ApiTokenStore` 的读写交错沿用 `AccountStore` 的计数；创建与撤销之间不排队 | 同意；一致而且简单 |
| 每代 `RootStore` 只创建一个客户端，两个 service 共用 | 同意 |
| 端到端在数据库层面断言；A10 查 DOM、存储、地址与控制台，用剪贴板读回核对 | 同意；重复运行稳定 |

## 文档与代码的不一致

审查者在 Step 已记录的偏差之外列出 7 处，全部处理（P6 文档第 7 节第 1–9 项，正文已同步）：

1. 布局不与第一个子页同一个 chunk，是自己的 lazy chunk，与子页并行加载；`password-length` 也单独成 chunk：3.2 已改。
2. 3.1 的文件表缺 `stores/context.tsx` 的 `useApiTokens`、`button.tsx` 的 `destructive`、主题的选项（随 N5 移到偏好的 store）、`profile-step.tsx` 改用 `DisplayNameForm`：3.1 已列。
3. 端到端的夹具多了 `answerTo`、`noteOf`（随 N8 在 `browser.ts`），`changePasswordWith` 返回状态码：3.8 已改。
4. A8 没有核对"与顶栏的菜单一致"：随 M3 补上。
5. S1 计划的"偏好写入设备"没有单元测试断言：不另加。设置页只调 `preferences.setTheme`、`setLocale`，页面的测试断言 store 的值与顶栏菜单；写入存储由 `preferences.store.test.ts`（M0/P5）断言，A8 刷新之后核对；反向对照"只改页面、不写设备"让 A8 失败（S4 已做）。
6. 令牌列表的加载失败与重试没有测试：随 M3 补上。
7. 只显示一次的警告与设计不同：随 N9 改为设计的"关闭之后不会再显示"。

## 没有失败的反向对照

- 去掉令牌视图的 `onInteractOutside`；`"clipboard" in navigator` 改成 `true`；去掉确认框的 `if (!sending)`；一年改成 366 天（M3，已补）。

## 没能验证的风险

- 审查者提出：`format.test.ts` 断言时间之前是普通空格，Node 24.15（ICU 78）输出 0x20，其他 ICU 版本可能是窄不换行空格。已改用 `\s`，两者都匹配；其余测试不断言带时间的文字。
