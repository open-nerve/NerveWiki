Nerve Wiki 镜像里的第三方许可证

/nervewiki 是一个程序：服务端的 Go 代码与 Go 本身（标准库与运行时）编译在里面，前端（@nervewiki/web）打包之后
也内嵌在里面。这个目录放它们的许可证文本，与 GitHub Release 的附件 nervewiki-<版本>-licenses.tar.gz 是同一份。

LICENSE                 Nerve Wiki 自己的许可证（AGPL-3.0-only）。
go/INDEX.tsv            链接进程序的每个 Go 模块与 Go 本身，每行：
                          <模块>@<版本> TAB <许可证文件，逗号分隔>
go/go@<Go 的版本>/       Go 本身的 LICENSE 与 PATENTS。
go/<模块>@<版本>/        这个模块根目录里的许可证文件，从 Go 的模块缓存原样复制。
npm/INDEX.tsv           前端（@nervewiki/web 与它用的工作区包）的每个生产依赖，加上代码随构建产物发出的开发依赖
                        （tailwindcss：它的 preflight 与默认主题编进了前端的 CSS），每行：
                          <包>@<版本> TAB <它的 package.json 写的许可证> TAB <许可证文件，逗号分隔；"-" 是包里没有许可证的文本>
npm/<包>@<版本>/         这个包根目录里的许可证文件，从装好的包原样复制。

许可证文件指名字是 LICENSE、LICENCE、UNLICENSE、COPYING、NOTICE、PATENTS 的文件（不分大小写，可带 .txt、.md、-MIT
之类的后缀；代码与数据文件不算，例如 license.go）。
构建镜像时由 tools/licenses/go.sh 与 tools/licenses/npm.mjs 收集（deploy/Dockerfile）。
源码：https://github.com/open-nerve/NerveWiki
