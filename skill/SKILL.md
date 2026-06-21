---
name: open-todolist
description: |
  使用 open-todolist (otl) CLI 工具管理本地任务和项目。当 Agent 需要创建待办、跟踪任务进度、标记任务完成/失败、管理任务依赖关系、查看下一个可执行任务时使用此 Skill。

  触发场景：
  - 用户要求创建任务、待办、todo、task
  - 用户要求跟踪进度、标记完成、标记失败
  - 用户提到项目管理、任务依赖、状态流转
  - Agent 执行多步骤任务需要跟踪子任务进度
  - 需要查看"接下来做什么"（next 命令）
  - 任何涉及本地任务跟踪和项目管理的场景
---

# open-todolist (otl) — 本地 CLI 任务跟踪

## 概述

`otl` 是一个纯本地、零依赖的 CLI 任务跟踪工具。数据存储在本地 SQLite 数据库中，无需联网、无需注册。面向 AI Agent 和人类用户，两者使用同一套 CLI 命令。

**核心能力**：
- 📁 项目管理（创建、查看、更新、删除）
- ✅ 任务管理（创建、查看、更新、删除，含描述和依赖）
- 🔄 状态流转（pending → in_progress → done/failed）
- 🔗 任务依赖（支持 depends-on，自动循环检测）
- 🎯 智能推荐（next 命令找出下一个可执行任务）

---

## 安装检查

执行前先确认 `otl` 可用：

```bash
which otl || go build -o otl .
```

如果 `which otl` 失败，从源码编译：

```bash
cd /Users/reachlucifer/project/opc/open-todolist
go build -o otl .
```

---

## 核心概念

### Project（项目）

任务是按项目组织的。**所有任务必须归属于一个项目**。项目名全局唯一（不区分大小写）。

### Task（任务）

任务有四种状态，流转规则如下：

```
pending ──→ in_progress ──→ done
                │
                └──→ failed ──→ in_progress / pending

done ──→ in_progress（重开）
```

**禁止的转换**：
- `pending → done`（必须经过 in_progress）
- `done → failed`（已完成的任务不能标记失败）

### 依赖关系

任务可以通过 `--depends-on` 设置前置依赖。系统自动检测循环依赖。`next` 命令会跳过依赖未完成的任务。

### 标识规则

- **创建**：用名称（如 `otl project create "我的项目"`）
- **其他操作**：用 ID（如 `otl task show <task-id>`）
- 创建操作返回 UUID v4 格式的 ID，后续操作使用该 ID

---

## 命令速查

### 全局选项

| 选项 | 说明 |
|------|------|
| `--db <path>` | 指定数据库路径（默认 `~/.open-todolist/data.db`） |
| `--help` | 查看帮助 |
| `--version` | 查看版本 |

### 项目管理

| 命令 | 说明 | 示例 |
|------|------|------|
| `otl project create <name> [-d <desc>]` | 创建项目，返回项目 ID | `otl project create "博客开发" -d "个人博客"` |
| `otl project list` | 列出所有项目（含任务统计） | `otl project list` |
| `otl project show <id>` | 查看项目详情 + 任务列表 | `otl project show abc123` |
| `otl project update <id> [--name <n>] [-d <d>]` | 更新项目名称或描述 | `otl project update abc123 --name "新名称"` |
| `otl project delete <id> [--force]` | 删除项目及所有任务 | `otl project delete abc123 --force` |

### 任务管理

| 命令 | 说明 | 示例 |
|------|------|------|
| `otl task create <pid> <name> [-d <d>] [--depends-on <tid>]` | 创建任务，返回任务 ID | `otl task create abc123 "设计数据库"` |
| `otl task list <pid> [--status <s>]` | 列出项目任务（按依赖排序） | `otl task list abc123 --status pending` |
| `otl task show <tid>` | 查看任务详情 | `otl task show def456` |
| `otl task update <tid> [--name <n>] [-d <d>] [--depends-on <tid>]` | 更新任务字段 | `otl task update def456 --name "新任务名"` |
| `otl task delete <tid> [--force]` | 删除任务（有后继依赖时阻止） | `otl task delete def456 --force` |
| `otl task status <tid> <status> [--reason <r>]` | 设置任务状态 | `otl task status def456 in_progress` |
| `otl task next <pid>` | 查看下一个可执行任务 | `otl task next abc123` |

### 状态值

`<status>` 取值：`pending` | `in_progress` | `done` | `failed`

---

## 典型工作流

### Agent 执行多步骤任务的标准流程

```
1. 创建项目
   otl project create "任务名称" -d "任务描述"
   → 记录返回的 project-id

2. 拆解子任务（按依赖顺序创建）
   otl task create <project-id> "步骤1"
   otl task create <project-id> "步骤2" --depends-on <步骤1的task-id>
   otl task create <project-id> "步骤3" --depends-on <步骤2的task-id>

3. 查看接下来做什么
   otl task next <project-id>
   → 返回所有可执行的任务（依赖已满足的 pending + 所有 failed）

4. 开始执行
   otl task status <task-id> in_progress

5. 执行完成
   otl task status <task-id> done

6. 执行失败（记录原因）
   otl task status <task-id> failed --reason "具体失败原因"

7. 重试失败任务
   otl task status <task-id> in_progress  # 失败原因自动清除

8. 循环步骤 3-7，直到所有任务 done
```

---

## Agent 最佳实践

### 1. 执行前先查 next

**永远不要假设该执行哪个任务**。先运行 `otl task next <project-id>`，它会根据依赖关系和状态自动判断。

```bash
otl task next <project-id>
```

### 2. 失败必须记录原因

标记 `failed` 时必须提供 `--reason`，否则命令会失败：

```bash
# ✅ 正确
otl task status <task-id> failed --reason "API 返回 503"

# ❌ 错误（会报错）
otl task status <task-id> failed
```

### 3. 使用独立数据库避免污染

Agent 执行任务时，建议用 `--db` 指定独立数据库：

```bash
otl --db /tmp/agent-tasks.db project create "自动化部署"
```

这样不会和用户的个人任务数据混淆。

### 4. 创建用名称，操作用 ID

```bash
# 创建：用名称
otl project create "我的项目"
# → 返回: abc123-def456-...

# 后续操作：用 ID
otl task create abc123-def456 "任务1"
otl task show abc123-def456
otl task status abc123-def456 done
```

### 5. 任务名在项目内唯一

同一项目内不能有同名任务（不区分大小写）。创建前可以先用 `otl task list <project-id>` 检查。

### 6. 删除前检查依赖

删除有后继依赖的任务会被阻止。如果需要删除，先删除或更新依赖它的任务。

### 7. 状态流转检查清单

| 当前状态 | 可转到 | 不可转到 |
|---------|--------|---------|
| pending | in_progress | done, failed |
| in_progress | done, failed | pending |
| done | in_progress | failed |
| failed | in_progress, pending | done |

---

## 常见错误处理

| 错误信息 | 原因 | 解决 |
|---------|------|------|
| `project name already exists` | 项目名重复（不区分大小写） | 换一个名字 |
| `task name already exists in this project` | 同一项目内任务名重复 | 换一个名字或先删除旧任务 |
| `project not found` | 项目 ID 不存在 | 用 `otl project list` 确认 ID |
| `task not found` | 任务 ID 不存在 | 用 `otl task list <pid>` 确认 ID |
| `cannot delete task: other tasks depend on it` | 有后继依赖 | 先删除或更新依赖它的任务 |
| `invalid status transition` | 状态流转不合法 | 参考状态流转规则 |
| `fail reason is required when setting status to failed` | 标记 failed 没给原因 | 加 `--reason "原因"` |
| `circular dependency detected` | 依赖形成环路 | 检查依赖链，去掉形成环的那条 |
| `cannot depend on itself` | 任务依赖自己 | 去掉自依赖 |
