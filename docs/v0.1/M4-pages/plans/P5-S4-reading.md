# M4/P5/S4 阅读视图与代码高亮：实施计划

上级：[P5 文档](../05-P5-tree-reading.md) 3.8、3.9、3.12。

## 任务

1. `reading/enhancement.ts`：增强的类型、注册表、上下文；`ReadingView` 按注册顺序运行、换 HTML 之前按相反顺序清理、一个增强抛错不影响其余。
2. `reading/reading.css`：排版与渲染器标记的样式，高亮的明暗两套。
3. 代码高亮：加依赖 `highlight.js`；`highlight.worker.ts`（`lib/common`）；`highlight.ts`（找代码块、Worker 工厂、2 秒的总超时终止、单块 100 KB 的上限）；`highlight-markup.ts`（白名单与文字一致的核对）。
4. 构建：Vite 把 Worker 打成 `assets/` 下的同源文件；`tsconfig` 给 Worker 的文件 `WebWorker` 的 lib。

## 测试

- 管线：顺序、清理顺序、换 HTML 时先清理再重跑、抛错的增强被隔离。
- 高亮：同步的假 Worker 着色；不答复的假 Worker 到时终止、代码照常显示；白名单拒绝 `span` 之外的元素与别的 class；文字不一致不着色；没有代码块不建 Worker。
- 反向对照：超时不终止 Worker；白名单放过 `a`；清理不终止 Worker。

## 完成检查

`make check` 为绿；`make build` 之后在浏览器里打开带代码块的页面，着色、没有 CSP 违规（S5 的 PG5 页面版本固定下来）。
