# 第三方组件与许可

- Go 依赖 `github.com/sashabaranov/go-openai` 的原始许可证保留在 [agent/vendor/github.com/sashabaranov/go-openai/LICENSE](agent/vendor/github.com/sashabaranov/go-openai/LICENSE)。版本以 `agent/go.mod` / `go.sum` 为准。
- Go 依赖 `gopkg.in/yaml.v3`（SKILL.md frontmatter 解析；纯 Go，无系统调用，随 Go 1.20 / windows-386 构建）的原始许可证与声明保留在 [agent/vendor/gopkg.in/yaml.v3/LICENSE](agent/vendor/gopkg.in/yaml.v3/LICENSE)、[agent/vendor/gopkg.in/yaml.v3/NOTICE](agent/vendor/gopkg.in/yaml.v3/NOTICE)。版本以 `agent/go.mod` / `go.sum` 为准。
- Vue 及 Vite 构建依赖的版本锁定在 `ui/package-lock.json`。Vue 随嵌入资源分发，其原始 [MIT许可证](docs/licenses/vue-LICENSE.txt) 已保留。Vite 为构建工具。
- Git、ripgrep、Sandboxie 与 Go/Node 工具链没有作为运行时二进制打入本源码仓库。制作分发包时，需随实际采用的组件保留其原始 LICENSE/NOTICE；不要沿用旧 RC 的二进制或哈希清单。

项目目前没有指定顶层开源 LICENSE。本次仓库整理未擅自选择 MIT、Apache 或其他许可证；第三方组件的许可不等于本项目整体许可。
