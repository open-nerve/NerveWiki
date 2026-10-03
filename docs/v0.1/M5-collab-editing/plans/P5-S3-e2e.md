# M5/P5/S3 端到端：C5、C6 的闲置与受影响的故事：实施计划

上级：[P5 文档](../05-P5-autosave-idle.md) 3.9、3.10；[M5 总设计](../00-M5-design.md) 第 3 节（C5、C6）、第 9 节。

## 任务

1. `e2e/fixtures/wiki-pages.ts`：`holdContentWrites(page, pageId)`，扣住这一页正文的 `PUT` 直到放行（照 `holdStream`）。
2. `e2e/stories/collab/c5-autosave.spec.ts`：C5（页面）：停顿约 2 秒保存；再输入、再停顿仍是一个变更集；输入法组合中不发、确认之后发出确认的字；Ctrl+S 立即保存（页面的时钟停住时）。
3. `e2e/stories/collab/c6-expired.spec.ts`：C6（页面，闲置）：快进 30 分钟，阅读视图说明、焦点在 Edit、会话已删、B 随即能编辑。
4. 改写：PG7（"留下"之前扣住写；revision 只断言最后的）、PG9（等"已保存"与落库；输入法改为组合中不发、确认之后发）、PG10（PUT 的清单只断言其中的 409 与最后的 200）、C4（输入之前扣住写，离开之后放行，答 404 声明）。核对 C1–C3、C6、PG8、PG12 不受影响。
5. `docs/v0.1/M4-pages/manual/P6-ime-checklist.md`：M5 的部分（P5 文档 3.10）。

## 测试

全量 e2e；改过与新加的故事另以 `--repeat-each` 压测。反向对照（`e2emut.py`）：自动保存不注册（C5 失败）；组合中不等就保存（C5 的输入法失败）；闲置不离开（C6 失败）；闲置之后没有说明（C6 失败）；不扣住写时以重复跑核实 C4、PG7 的扣住是必要的。

## 完成检查

`make check`、`make e2e` 为绿；本地 9 个与 2 个 worker 各一次。
