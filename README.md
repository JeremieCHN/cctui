# CC Switch TUI

`CC Switch TUI` 是一个终端界面工具，用来管理并切换 `Claude`、`Codex`、`Gemini`、`Opencode` 的多套供应商配置。

它适合以下场景：

- 在官方接口、代理接口、公司内网网关之间快速切换
- 为不同应用分别维护多套 `Base URL`、`API Key`、`Model`
- 用可视化 TUI 替代手动编辑多个配置文件

## 功能特性

- 支持 `Claude`、`Codex`、`Gemini`、`Opencode` 四类应用
- `Claude`、`Codex`、`Gemini` 为替换模式：同一时刻只有一个供应商生效
- `Opencode` 为增量模式 `(I)`：所有供应商共存于同一配置文件
- 使用 SQLite 保存供应商配置，数据默认位于 `~/.cc-switch/`
- 首次启动时，如果某个应用还没有保存的供应商，会尝试导入当前 live 配置
- 切换前会先读取当前 live 配置并回写数据库，尽量保留你在外部手动改过的内容
- 支持新增、编辑、删除、切换供应商
- `Claude` 支持分别配置主模型、`Fable`、`Haiku`、`Sonnet`、`Opus` 默认模型与子代理模型
- 模型字段可按 `Ctrl+L` 从供应商拉取模型列表并选择，不必手输
- `Codex` 额外支持配置 `Reasoning Effort`

## 管理的配置文件

程序会读取并写入这些 live 配置文件：

- `Claude`：`~/.claude/settings.json`
- `Claude` 兼容旧文件：`~/.claude/claude.json`
- `Codex`：`~/.codex/auth.json`
- `Codex`：`~/.codex/config.toml`
- `Gemini`：`~/.gemini/.env`
- `Gemini`：`~/.gemini/settings.json`
- `Opencode`：`~/.config/opencode/opencode.json`

程序自己的本地数据默认保存在：

- `~/.cc-switch/cc-switch.db`
- `~/.cc-switch/settings.json`

## 环境要求

- `Go 1.26+`
- 可以访问对应应用的本地配置目录
- 终端支持 TUI/Alt Screen

## 构建与运行

### 直接运行

```bash
go run .
```

### 构建二进制

```bash
go build -o cctui .
./cctui
```

## GitHub Actions 构建与发布

仓库内置了两个 workflow：

- `CI`（`.github/workflows/ci.yml`）：在 push 到 `main`、提交 PR 或手动触发时，执行 `gofmt` 检查、`go vet`、`go test` 与一次构建。
- `Release`（`.github/workflows/release.yml`）：在 Actions 页面手动触发，交叉编译 `linux`/`darwin` 的 `amd64`、`arm64` 以及 `windows/amd64` 二进制，生成 `sha256` 校验文件，并在固定的 `latest` tag 上创建或覆盖更新 GitHub Release。

### 发布一次

1. 打开仓库的 `Actions` 页面，选择 `Release` workflow。
2. 点击 `Run workflow`，无需填写任何内容。
3. 运行结束后，产物会出现在 `Releases` 页面，下载地址固定为 `https://github.com/<owner>/<repo>/releases/latest`。

每次触发都会把 `latest` tag 移动到当次构建的 commit，并用新产物覆盖旧产物，所以 Releases 页面始终只有这一个 Release，下载链接长期不变，适合小范围持续分发。

产物命名形如 `cctui-linux-amd64`、`cctui-darwin-arm64`、`cctui-windows-amd64.exe`，并各自附带 `.sha256` 文件。Release 说明中会记录本次构建的完整 commit。

### 说明

- 不需要配置任何 secret。
- 全程无需手动打 tag：workflow 会自动创建或移动固定的 `latest` tag。
- Release 需要仓库的 Workflow 权限允许写入（`Settings -> Actions -> General -> Workflow permissions` 选择 `Read and write permissions`，或依赖 job 内的 `permissions: contents: write`）。
- `latest` 是一个会被强制移动的 tag，本地 `git fetch --tags` 时可能需要加 `-f` 才能同步。

## 使用方式

启动后会看到按应用分组的供应商列表：

- `Enter`：替换模式下将当前选中的供应商设为正在使用；增量模式下进入编辑
- `a`：新增供应商
- `e`：编辑供应商
- `d`：删除供应商
- `↑/↓` 或 `j/k`：移动光标
- `1/2/3/4`：快速跳转到 `Claude` / `Codex` / `Gemini` / `Opencode`
- `g/G`：跳到顶部 / 底部
- `q`：退出

表单模式下：

- `Tab` / `Shift+Tab`：切换字段
- `Enter`：下一项，最后一项时保存
- `Ctrl+L`：聚焦在模型字段时，从 `Base URL` 拉取模型列表并选择
- `Ctrl+S`：保存
- `q`：取消并返回

模型选择器内：

- 直接输入关键字筛选
- `↑/↓`：移动光标
- `Enter`：选中并写回字段
- `Ctrl+R`：重新拉取
- `Esc`：返回表单

## 模型列表

聚焦到模型字段（`Model`、`Fable Model` 等）按 `Ctrl+L`，会用当前表单里的 `Base URL` 与 `API Key` 拉取可用模型：

- 先请求 `{Base URL}/v1/models`（OpenAI 兼容，返回当前 key 可用的模型）
- 失败时回退到 `{Base URL}/api/pricing`（new-api 模型广场的公开接口，不需要 key）

拉到的列表在同一个表单会话内缓存，切换字段后再次打开不会重复请求，按 `Ctrl+R` 可以强制刷新。

## 供应商字段说明

通用字段：

- `名称`：供应商显示名称
- `Base URL`：接口地址；留空通常表示沿用官方登录或 OAuth 语义
- `API Key`：接口密钥；留空时会尽量保留登录态语义
- `Model`：默认模型
- `Website`：可选，供应商官网
- `Notes`：可选，备注

`Claude` 独有字段：

- `Fable Model` / `Haiku Model` / `Sonnet Model` / `Opus Model`：分别对应 `ANTHROPIC_DEFAULT_FABLE_MODEL`、`ANTHROPIC_DEFAULT_HAIKU_MODEL`、`ANTHROPIC_DEFAULT_SONNET_MODEL`、`ANTHROPIC_DEFAULT_OPUS_MODEL`，用于覆盖对应模型别名解析到的模型
- `Subagent Model`：对应 `CLAUDE_CODE_SUBAGENT_MODEL`，子代理使用的模型
- 以上字段留空时跟随 `Model`（`ANTHROPIC_MODEL`）：只填 `Model` 时这些环境变量写入同一个值；全部留空时不写入这些环境变量
- 保存时会同步写入 `ANTHROPIC_DEFAULT_<别名>_MODEL_NAME`，与对应模型值保持一致

`Codex` 独有字段：

- `Reasoning Effort`：例如 `medium`、`high`

## 默认行为

### 首次启动导入

如果某个应用在数据库里还没有供应商，程序会尝试读取该应用当前正在使用的 live 配置，并自动导入一条记录，例如 `Imported Claude`。

### 启动时同步

启动时会读取每个应用（不含增量模式的 `Opencode`）的 live 配置，与数据库中当前供应商的记录对比；发现不同时以 live 为准更新记录，你在外部手动改过的内容会立即反映到 TUI 中。

### 切换时同步

切换到新供应商前，程序会先尝试读取当前 live 配置，并回写到当前供应商记录中；随后再把目标供应商写入 live 配置文件。

## 高级配置

可以在 `~/.cc-switch/settings.json` 中覆盖默认配置目录：

```json
{
  "claudeConfigDir": "/path/to/.claude",
  "codexConfigDir": "/path/to/.codex",
  "geminiConfigDir": "/path/to/.gemini",
  "opencodeConfigDir": "/path/to/.config/opencode"
}
```

其中“当前供应商”相关字段也会保存在这个文件里，通常不建议手动修改。

## 适用场景

- 在官方和第三方中转之间快速切换
- 分离工作环境与个人环境配置
- 为不同模型服务保留独立的历史配置
- 用统一入口管理多个 AI CLI / 桌面工具的连接参数
