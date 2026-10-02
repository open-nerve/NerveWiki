# M4/P3/S4 耗时：实施计划

上级：[P3 文档](../03-P3-markdown.md) 3.10。

## 任务

1. `markdowntest`：普通文档的生成器（固定种子：标题、段落里的强调与链接与代码段、列表、任务、表格、引用、围栏代码、脚注、少量行内 HTML，中英文混合）；病态输入的生成器（P3 文档第 1 节表格的每一类，另加 YAML、原始 HTML、地址、超长行、超大 frontmatter），按目标大小生成，带名字。
2. 相对耗时的测试（`markdown` 包）：每个病态输入 256 KB，三次里最快的 `Parse` + `Render` 不超过同样大小的普通文档的 K 倍；普通文档 1 MB 的渲染不超过 1 秒（粗的绝对下限，防数量级的退步）。竞态检测下跳过（`race_test.go`/`norace_test.go` 定义常量，同 bodyshape）；`Makefile` 的 `test-go` 在不带竞态检测的构建里另跑它，持续集成随之。
3. 基准：`BenchmarkParse`、`BenchmarkRender`（普通文档 100 KB、1 MB、5 MB），注释写手动运行的命令。
4. 模糊测试：`FuzzParse`、`FuzzRender`，种子为样例集与生成器的输出；性质：不 panic、`CheckHTML` 通过、frontmatter 的边界与规则 1 一致、`Parse` 的副本与原文等长。
5. 实测之后定预算与 K，写进 P3 文档第 7 节；超出的修到满足。

## 测试

- 上面的测试本身。反向对照：把 S1 的任一替换件换回原版（相对耗时的测试失败）；YAML 的上限放开（别名炸弹的输入超时失败）。
- 本地跑一次 `go test -fuzz=FuzzRender -fuzztime=10m`，结果写进第 7 节。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
