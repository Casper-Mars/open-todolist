# open-todolist

<p align="center">
  <strong>📋 轻量级终端任务管理工具</strong><br>
  <sub>为 AI Agent 和人类用户设计的纯本地 CLI 任务跟踪系统</sub>
</p>

---

## 简介

**open-todolist**（简称 `otl`）是一个在终端里运行的任务管理工具。它把项目（Project）和任务（Task）存在本地的 SQLite 数据库里，不需要注册账号、不需要联网、不需要 Docker。一条命令就能开始用。

适合谁用？

- 🧑‍💻 **开发者**：不想离开终端，习惯 CLI 工作流
- 🤖 **AI Agent**：结构化输出，适合脚本和自动化集成
- 📦 **极简主义者**：一个二进制文件，零依赖，即下即用

### 核心能力

| 能力 | 说明 |
|------|------|
| 📁 **项目管理** | 创建、查看、更新、删除项目 |
| ✅ **任务管理** | 创建、查看、更新、删除任务，支持描述和状态 |
| 🔄 **状态流转** | `pending → in_progress → done/failed`，失败可记录原因 |
| 🔗 **任务依赖** | 任务可以依赖其他任务，自动检测循环依赖 |
| 🎯 **智能推荐** | `next` 命令自动找出下一个可执行的任务 |
| ⚡ **纯静态编译** | `CGO_ENABLED=0`，单二进制，拷贝即用 |

---

## 安装

### 从源码编译

```bash
# 要求 Go 1.23+
git clone https://github.com/Casper-Mars/open-todolist.git
cd open-todolist
CGO_ENABLED=0 go build -o otl .
```

编译完成后把 `otl` 放到 `$PATH` 里就行：

```bash
sudo mv otl /usr/local/bin/
```

### 从 Release 下载

前往 [Releases](https://github.com/Casper-Mars/open-todolist/releases) 下载对应平台的二进制文件，解压后放到 `$PATH` 即可。

---

## 快速开始

安装完成后，直接运行 `otl` 会初始化数据库（默认在 `~/.open-todolist/data.db`）：

```bash
$ otl
✓ Database initialized at /home/user/.open-todolist/data.db
```

### 3 分钟上手

```bash
# 1. 创建一个项目
otl project create "我的第一个项目" -d "用来学习 otl"

# 2. 在项目里添加任务（复制上面输出的项目 ID）
otl task create <project-id> "阅读文档"
otl task create <project-id> "安装配置"
otl task create <project-id> "写第一个 Demo" --depends_on <阅读文档的task-id>

# 3. 看看接下来该做什么
otl task next <project-id>

# 4. 开始干活
otl task status <task-id> in_progress

# 5. 完成了！
otl task status <task-id> done
```

---

## 命令参考

### 全局选项

| 选项 | 说明 |
|------|------|
| `--db <path>` | 指定数据库文件路径（默认 `~/.open-todolist/data.db`） |
| `--help` | 查看帮助 |
| `--version` | 查看版本 |

### 项目管理

#### `otl project create`

```bash
otl project create <name> [--description <desc>]
```

创建一个新项目。项目名不能为空，不能超过 100 个字符，且不区分大小写（`Demo` 和 `demo` 视为同名）。

```bash
# 示例
otl project create "我的博客" -d "个人博客开发计划"
```

#### `otl project list`

```bash
otl project list
```

列出所有项目，按创建时间倒序排列，显示每个项目的任务数量。

```bash
# 输出示例
ID     NAME       TASKS  CREATED
abc123 我的博客   3      2024-01-15T10:30:00Z
def456 学习计划   0      2024-01-14T08:00:00Z
```

#### `otl project show`

```bash
otl project show <project-id>
```

查看项目详情，包括项目信息和关联的任务列表。

#### `otl project update`

```bash
otl project update <project-id> [--name <new-name>] [--description <new-desc>]
```

更新项目名称或描述，至少需要提供一个参数。

#### `otl project delete`

```bash
otl project delete <project-id> [--force]
```

删除项目及其所有任务。默认会要求确认，加 `--force` 跳过确认。

---

### 任务管理

#### `otl task create`

```bash
otl task create <project-id> <name> [--description <desc>] [--depends_on <task-id>]
```

在指定项目中创建任务。任务名在同一项目内唯一（不区分大小写）。可以通过 `--depends_on` 设置前置依赖。

```bash
# 创建普通任务
otl task create abc123 "设计数据库"

# 创建带依赖的任务
otl task create abc123 "实现 API" --depends_on def456
```

#### `otl task list`

```bash
otl task list <project-id> [--status <status>]
```

列出项目中的所有任务，按依赖关系拓扑排序（无依赖的排前面）。可以用 `--status` 过滤：

- `pending` — 待处理
- `in_progress` — 进行中
- `done` — 已完成
- `failed` — 已失败（会额外显示失败原因列）

#### `otl task show`

```bash
otl task show <task-id>
```

查看任务的完整信息，包括状态、依赖、失败原因、时间戳等。

#### `otl task update`

```bash
otl task update <task-id> [--name <name>] [--description <desc>] [--status <status>] [--depends_on <task-id>] [--fail_reason <reason>]
```

更新任务字段，至少需要提供一个参数。设置依赖时会自动检测循环依赖。

#### `otl task status`

```bash
otl task status <task-id> <status> [--reason <reason>]
```

设置任务状态。支持的状态流转：

```
pending ──→ in_progress ──→ done
                │
                └──→ failed ──→ in_progress / pending
```

规则说明：

- `pending` 只能转为 `in_progress`
- 设为 `failed` 时必须提供 `--reason`（最多 500 字符）
- `done` 的任务不能标记为 `failed`
- 从 `failed` 恢复时自动清除失败原因
- 启动依赖未完成的任务时会给出警告但不阻止

```bash
# 开始任务
otl task status abc123 in_progress

# 标记失败
otl task status abc123 failed --reason "API 接口超时"

# 重试
otl task status abc123 in_progress

# 完成
otl task status abc123 done
```

#### `otl task next`

```bash
otl task next <project-id>
```

智能推荐下一个可以执行的任务。推荐规则：

- 无依赖的 `pending` 任务 → 可执行
- 依赖已完成的 `pending` 任务 → 可执行
- 依赖未完成的 `pending` 任务 → 跳过
- `failed` 任务 → 始终可执行（需要重试）

```bash
# 输出示例
ID     NAME       STATUS   DEPENDS ON   FAIL REASON    CREATED
abc    设计数据库  pending  -            -              2024-01-15T10:00:00Z
def    实现 API   failed   -            API 超时        2024-01-15T11:00:00Z
```

#### `otl task delete`

```bash
otl task delete <task-id> [--force]
```

删除任务。默认会要求确认，加 `--force` 跳过。如果有其他任务依赖它，会阻止删除。

---

## 示例场景

### 场景一：个人博客开发

```bash
# 创建项目
otl project create "个人博客" -d "使用 Hugo + GitHub Pages"

# 添加任务（假设项目 ID 是 p1）
otl task create p1 "搭建 Hugo 环境"
otl task create p1 "选择主题"
otl task create p1 "写第一篇文章" --depends_on <搭建Hugo的task-id>
otl task create p1 "配置 GitHub Actions" --depends_on <搭建Hugo的task-id>
otl task create p1 "绑定自定义域名" --depends_on <配置GitHubActions的task-id>

# 看看先做什么
otl task next p1
# → 搭建 Hugo 环境、选择主题（无依赖，可以直接开始）

# 开始干活
otl task status <搭建Hugo的task-id> in_progress
otl task status <搭建Hugo的task-id> done

# 再次查看下一步
otl task next p1
# → 写第一篇文章、配置 GitHub Actions（依赖已完成）
```

### 场景二：带失败重试的工作流

```bash
# 创建任务
otl task create p1 "调用第三方 API"

# 开始执行
otl task status <task-id> in_progress

# 失败了，记录原因
otl task status <task-id> failed --reason "第三方 API 返回 503"

# 第二天重试
otl task status <task-id> in_progress
# 失败原因自动清除

# 成功了
otl task status <task-id> done
```

### 场景三：AI Agent 集成

```bash
# Agent 可以用 --db 指定独立数据库，避免和用户数据混淆
otl --db /tmp/agent-tasks.db project create "自动化部署"

# 结构化输出便于解析
otl --db /tmp/agent-tasks.db task next <project-id>
# → 表格格式，可以按列解析
```

---

## 技术细节

- **语言**：Go 1.23+
- **CLI 框架**：[cobra](https://github.com/spf13/cobra)
- **数据库**：[modernc.org/sqlite](https://modernc.org/sqlite)（纯 Go 实现的 SQLite，无需 CGO）
- **编译**：`CGO_ENABLED=0` 纯静态编译，单二进制文件
- **数据存储**：默认 `~/.open-todolist/data.db`，WAL 模式，支持并发读

---

## 开发

```bash
# 克隆仓库
git clone https://github.com/Casper-Mars/open-todolist.git
cd open-todolist

# 编译
go build -o otl .

# 运行测试
go test ./test/ -v

# 运行特定测试
go test ./test/ -run TestAC_P01 -v
```

---

## License

[MIT](LICENSE)
