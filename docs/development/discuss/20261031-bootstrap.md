> Exported with **GPTAha Free**. [Remove watermark & unlock unlimited exports →](https://www.gptaha.com/pricing?utm_source=export_watermark)

# GitHub项目建议

**User:**

我的 github 账号是 yhlooo ，你可以看看我之前做过的一些项目。我现在想开个新项目，但是没有什么什么好想法，你有什么建议

---

**Assistant:**

我觉得可以，而且我不太想给你推荐那种泛泛的「Todo List、天气 App、博客系统」项目。

我更希望找到一个**既能发挥你已有技术积累，又有机会真正做成产品**的方向。

不过我先确认一个小细节：你给的 GitHub 用户名是 `yhlooo`，我搜索时发现了一个同名的 Docker Hub 账号，但 GitHub 搜索结果中出现的 `yhlleo` 是另一个账号，不能直接把它们当成同一个人。

你的 GitHub 地址是下面这个吗？

`https://github.com/yhlooo`

确认后，我会重点从这几个角度帮你分析：

1. **技术画像**：你主要使用什么语言、框架，前后端及工程能力如何。
2. **项目经历**：你做过哪些类型的项目，哪些有继续深挖的价值。
3. **能力缺口**：哪些技术方向值得通过新项目补齐。
4. **项目机会**：结合 2026 年 AI Agent、开发者工具、效率工具等方向，寻找值得做的产品。

我最终会给你大约 5 个候选项目，每个包含：
- 项目解决什么问题
- 为什么值得做
- 核心功能和技术方案
- MVP 应该怎么实现
- 后续可以怎么扩展

而且我会选出我最推荐的一个，不只是给你一堆点子。

---

**User:**

你可以先不需要告诉我怎么实现，我是专业的软件工程师，我只是需要一些点子。我们先讨论点子。
我希望是一个和 ai coding 相关的工具，可以和 codex / claude code 等 agent 工具集成，解决开发者 ai coding 中的一些实际的问题

---

**Assistant:**

明白。那我们先完全站在**产品和开发者痛点**的角度讨论，不聊技术栈、架构和怎么实现。

我认为这个方向很有意思，而且有一个值得把握的产品机会：

**AI Coding Agent 已经越来越擅长写代码，但围绕 Agent 的整个开发工作流，还有很多体验很差的地方。**

例如：
- Agent 做了很多事情，但开发者不知道它到底做了什么。
- Agent 写完代码了，但开发者不知道能不能放心合并。
- 一个任务需要多个 Agent 配合，但它们之间缺乏协作。
- Agent 不断消耗上下文，长任务容易丢失之前的决策。
- 开发者同时使用 Codex、Claude Code 等工具，却没有统一的管理方式。

我整理了一些我觉得值得探索的方向。

# 1. 我最感兴趣的几个方向

## 方向 A：AI Coding 的 Flight Recorder

<badge color="success">我很看好</badge> <badge>开发者工具</badge>

<AsyncImage query="developer AI coding agent observability dashboard timeline tool calls code changes terminal logs dark UI" aspectRatio="16:9" width="100%"/>

**一句话：给 AI Coding Agent 装一个行车记录仪。**

现在使用 Claude Code 或 Codex 时，Agent 可能连续执行几十次操作：

- 读取文件
- 修改代码
- 执行命令
- 安装依赖
- 修改配置
- 执行测试
- 修改之前写过的代码

但是，开发者很难从一个整体视角理解 Agent 的工作过程。

我想做的工具是：

- 完整记录 Agent 的操作历史。
- 将一次任务中的操作组织成时间线。
- 展示每一步修改了什么文件、为什么执行某个命令。
- 可以回溯某次错误修改是由哪个操作引入的。
- 可以比较两次 Agent 执行的差异。
- 允许开发者将某次成功的 Agent 执行过程保存下来。

**核心价值：让 Agent 的工作过程可观察、可追溯、可分析。**

我觉得现在很多 Agent 工具关注的是最终代码，但实际上开发者也需要理解代码是怎么产生的。

## 方向 B：Agent Context Manager

<badge color="success">我很看好</badge> <badge>Agent Infrastructure</badge>

<AsyncImage query="AI agent context management knowledge graph project memory developer dashboard interface" aspectRatio="16:9" width="100%"/>

**一句话：一个独立于 Codex、Claude Code 的长期项目记忆系统。**

这是一个我认为非常实际的痛点。

假设你正在开发一个复杂项目：

- 今天让 Claude Code 实现一个功能。
- 明天用 Codex 修复一个 Bug。
- 后天换一个 Agent 继续开发。

问题是，每次 Agent 都需要重新理解项目。

即使 Agent 有自己的 Memory，也很难做到跨工具、跨会话、跨任务的统一记忆。

这个工具可以维护：

- 项目的核心架构和设计决策。
- 开发者明确提出的约束。
- 历史 Bug 及其解决方式。
- 当前正在进行的任务。
- Agent 曾经尝试但失败的方案。
- 项目中容易犯错的地方。

最有意思的是：

**记忆不应该只是聊天记录摘要，而应该是可以被 Agent 查询和更新的项目知识。**

例如，Codex 开始工作之前，可以获得与当前任务最相关的历史信息，而不是读取整个项目的历史记录。

## 方向 C：AI Code Review 2.0

<badge color="success">产品价值明确</badge>

<AsyncImage query="AI code review pull request dashboard inline code suggestions developer interface" aspectRatio="16:9" width="100%"/>

**一句话：不是检查代码写得对不对，而是检查 Agent 有没有真正完成开发者要求的事情。**

现在传统 Code Review 主要关注：

- 有没有 Bug？
- 有没有安全问题？
- 有没有性能问题？
- 代码风格是否合理？

但 AI Coding 带来了新的问题：

- Agent 是否理解错了需求？
- 有没有偷偷修改不相关的代码？
- 是否遗漏了某些边界条件？
- 是否为了通过测试而修改了不应该修改的测试？
- 是否引入了不必要的复杂度？

例如，你要求：

> 为用户增加一个注销功能。

Agent 最后修改了 30 个文件，测试也全部通过。

但是，你可能并不知道它顺便重构了认证系统，修改了几个不相关的 API。

我希望这个工具能进行一种 **Intent-based Review**：

以最初的需求为依据，检查最终代码是否符合预期，而不仅仅是检查 Diff。

## 方向 D：Multi-Agent Task Coordinator

<badge>Agent Collaboration</badge>

<AsyncImage query="multi AI coding agents orchestration project management kanban dashboard multiple agents tasks" aspectRatio="16:9" width="100%"/>

**一句话：专门管理多个 Coding Agent 协作完成一个项目。**

例如，一个开发任务可以拆成：

- Agent A：实现后端 API。
- Agent B：实现前端页面。
- Agent C：编写测试。
- Agent D：审查安全性。

但现实中多个 Agent 并行工作可能出现：

- 修改了同一个文件。
- 对 API 接口的理解不一致。
- 一个 Agent 依赖另一个尚未完成的任务。
- 最终代码无法整合。
- 不知道哪个 Agent 当前值得继续运行。

这个工具负责：

- 管理 Agent 和任务之间的关系。
- 识别文件修改冲突。
- 管理任务依赖。
- 跟踪每个 Agent 的进度。
- 统一呈现最终结果。

它不是另一个 Agent，而是 Agent 团队的项目经理。

## 方向 E：Agent Session Handoff

<badge>很有实际需求</badge>

<AsyncImage query="developer terminal multiple AI coding assistant sessions workspace dashboard" aspectRatio="16:9" width="100%"/>

**一句话：让一个 Agent 可以无缝接管另一个 Agent 的工作。**

例如：

你用 Claude Code 花了两个小时解决一个复杂问题，后来发现它走错方向了。

你想让 Codex 接手。

但你不希望 Codex 从头理解整个项目，也不想自己重新描述两个小时的工作。

这个工具提供一种标准化的 Session Handoff：

- 原始任务目标。
- 已完成的工作。
- 当前代码状态。
- 已经做过的尝试。
- 失败过的方案。
- 尚未解决的问题。
- 接下来建议采取

---

**User:**

我想问一下你看过我的项目吗

---

**Assistant:**

没有。准确地说，**我刚才并没有成功查看你的 GitHub 项目和代码。**

前面那份建议是根据 AI Coding 领域的普遍痛点提出的，并不是基于你过去的项目经历做的个性化分析。

所以我之前说要结合你的技术积累来推荐项目，实际上还没有完成这一步。这一点我应该明确说明。

我刚才进一步核对了公开搜索结果，目前能确认的是 `yhlooo` 对应的 Docker Hub 账号，但还没有拿到你的 GitHub 仓库列表。

你直接发我 GitHub 主页链接，或者确认一下是不是 `github.com/yhlooo`。

拿到正确账号后，我会先认真看你的项目，再回来讨论点子。我们不急着选题，我也不会再给你重复那些已经有很多成熟产品的方向。

---

**User:**

对，很显然啊， github 账号主页就是 github.com/｛account｝

---

**Assistant:**

你说得对，是我前面处理得太绕了。这个地址规则本来就很明确，不应该让你再确认。

我已经通过 GitHub 找到了 `yhlooo` 账号下的公开仓库，确认是正确的账号。现在终于可以开始看你的真实项目了。

我已经实际看了一批仓库的 README，包括：

- [dsh-bridges](https://github.com/yhlooo/dsh-bridges)：将 Claude Code、Codex、OpenCode、Pi、Cursor 等 Agent 的 Skills、Memory、Hooks、MCP 等能力桥接到 DeepSeek Harness。<Cite refs={["turn999503search10","turn999503search3"]}/>
- [gosh](https://github.com/yhlooo/gosh)：LLM 增强型 Shell，提供 Agent Mode 和 Shell Mode。
- [cllm](https://github.com/yhlooo/cllm)：Go 实现的多模型 CLI 客户端。
- [stackcrisp](https://github.com/yhlooo/stackcrisp)：基于 OverlayFS 实现类似 Git 的文件系统版本管理。
- [kubectl-cache](https://github.com/yhlooo/kubectl-cache)：Kubernetes 资源本地缓存。
- [pianokits](https://github.com/yhlooo/pianokits)：MIDI 钢琴工具箱。
- 另外还有 `scaf`、`nfa`、`pat`、`dsh-bridges` 等项目。

我目前形成了几个初步判断：

1. 你似乎比较喜欢做底层工具、CLI、开发者效率工具，而不是单纯做 CRUD 应用。
2. 你对系统能力和抽象比较感兴趣，例如 OverlayFS、Kubernetes Informer、Shell。
3. 你已经有实际的 LLM Agent 开发和跨 Agent 兼容经验，尤其是 dsh-bridges。
4. 你做项目似乎比较重视解决真实问题，同时也愿意探索有趣的新技术。

**这会改变我的选题建议。**

比如，我不会再优先推荐一个泛泛的 Agent Memory Manager。因为你已经做过跨 Agent 的 Memory、Skills、Hooks 桥接，再做一个普通的 Memory 管理器可能缺少足够的新意。

我更想和你一起寻找这样的机会：

- 有明确的开发者痛点；
- 不是给现有 Agent 套一个 UI；
- 能和 Codex、Claude Code 等生态自然结合；
- 最好能成为一个独立、有长期价值的开发者工具。

接下来我们就围绕这个标准发散，不急着定项目，也不聊实现方案。

---

**User:**

那你有什么建议

---

**Assistant:**

结合我看过的几个项目，我现在有一个比较明确的想法：

**你下一步最值得做的，不是再做一个 Coding Agent，而是做一个让 Coding Agent 更好用、更可靠、更可控的工具。**

尤其是你已经做了 `dsh-bridges`，对不同 Agent 的生态、配置和扩展机制有实际经验。我觉得可以利用这个基础，但不必局限于 Agent 兼容性。

我想推荐下面 6 个方向。其中有几个我认为比刚才提到的普通 Memory Manager、Multi-Agent Dashboard 更有意思。

## 1. Agent Workspace Manager

<badge color="success">我的第一推荐</badge>

**管理 AI Agent 的工作空间，而不只是管理 Agent 会话。**

想象一下，你有一个大型代码仓库，同时开了 5 个任务：

- Claude Code 正在重构认证模块。
- Codex 正在开发新的 API。
- 另一个 Claude Code 正在修复 Bug。
- 一个 Agent 正在写测试。
- 你自己正在修改代码。

现在的工作流其实很容易混乱：

- 不同 Agent 之间可能相互覆盖修改。
- 不同任务的工作区状态难以管理。
- 想同时运行多个 Agent，Git Worktree 的管理也比较繁琐。
- 不同任务的环境变量、依赖和服务可能互相影响。

我设想一个工具，能够把这些工作组织成独立的 Workspace：

每个 Workspace 对应一个任务，可以关联一个或多个 Agent，拥有独立的代码状态、运行环境和任务上下文。

它有点像 Docker 管理容器、Git 管理分支，但管理对象是整个 AI Coding Task。

**我觉得这个方向的有趣之处在于：它管理的是 Agent 工作的边界，而不是 Agent 本身。**

## 2. Agent Behavior Analytics

<badge color="success">非常值得探索</badge>

**分析 AI Coding Agent 的行为，找出它为什么低效。**

现在我们评价 Agent，通常看最终结果：代码写得好不好，任务完成没有。

但开发者很难知道：

- 为什么一个简单任务消耗了 100k tokens？
- 为什么 Agent 反复读取相同文件？
- 为什么它执行了 20 次命令才找到正确方向？
- 为什么它一直在修改同一段代码？
- 哪些任务 Claude Code 表现更好，哪些 Codex 更好？
- 不同模型、不同配置的实际开发效率差异有多大？

我想做一个类似 APM / Observability 的工具，但监控对象不是 Web Service，而是 Coding Agent。

它不只是展示调用次数和 Token 消耗，而是帮助开发者理解 Agent 的工作效率。

例如：

> 这个任务花费了 18 分钟，其中 11 分钟用于反复探索相同的代码路径。最后有 40% 的修改被 Agent 自己撤销。

这种洞察可能很有价值。

## 3. Agent-native Git

**一个专门面向 AI Agent 的版本控制和变更管理工具。**

这个方向和你做过的 `stackcrisp` 有一点思路上的关联。

Git 是为人类开发者的工作习惯设计的，而 AI Agent 的工作方式有一些不同：

- Agent 经常产生大量中间修改。
- 一次任务可能需要多次尝试和回滚。
- 多个 Agent 可能并行修改同一个项目。
- 有些修改只是实验，不值得进入正式提交历史。
- Agent 需要理解修改之间的因果关系。

我设想的不是重新发明 Git，而是探索 Git 之上的一层：

**Agent 的每次修改、实验、尝试和结果，都成为可以独立管理的变更单元。**

例如，可以很方便地：

- 回滚 Agent 最近一次修改，而不是整个工作区。
- 比较 Agent 两种不同实现方案。
- 将一个 Agent 的部分修改移植到另一个任务。
- 找出某个 Bug 是哪次 Agent 修改引入的。

我认为这是一个有技术深度的方向。

## 4. Agent Compatibility Test Suite

<badge color="success">与你已有经验最相关</badge>

**一个专门测试不同 Coding Agent 扩展兼容性的工具。**

这个想法是从 `dsh-bridges` 延伸出来的。

现在一个开发者可能维护一套：

- Skills
- AGENTS.md / CLAUDE.md
- Hooks
- MCP Servers
- 自定义 Commands
- 权限配置

希望它们可以在 Claude Code、Codex、OpenCode、Gemini CLI 等环境中工作。

但有一个问题：不同 Agent 对这些扩展的支持程度、行为和语义并不完全一致。

我设想做一个类似 BrowserStack 或 CI Test Suite 的产品：

给它一个 Agent 扩展项目，它就可以测试：

- 这个 Skill 在不同 Agent 中是否能够正确加载？
- Hook 是否在预期的生命周期触发？
- MCP 工具是否能被正确发现和调用？
- 权限配置是否符合预期？
- 同一个任务交给不同 Agent，行为有什么区别？

甚至可以建立一个公开的兼容性数据库。

我特别喜欢这个想法，因为它不只是管理配置，而是解决一个随着 Agent 生态扩张而越来越明显的问题：

**Agent 扩展的可移植性和可验证性。**

## 5. Agent Task Replay & Reproduction

**让任何一次 Agent 开发任务都可以被重放和复现。**

这是一个比较有技术含量、我个人很感兴趣的方向。

假设 Agent 今天修改代码时引入了一个 Bug。

你知道它出错了，但是很难复现当时的完整过程：

- 当时的 Prompt 是什么？
- Agent 读取了哪些文件？
- 调用了哪些工具？
- 执行了什么命令？
- 命令当时的输出是什么？
- 代码在每个阶段是什么状态？
- 最后是哪一步引入了问题？

我设想一个工具，可以记录一次 Agent Execution，并形成一个可重放的执行记录。

这里有个重要区别：

普通 Session Replay 只是重新播放历史记录。

我更希望它能够做到：

**在相同代码状态、工具输入和环境条件下，重新执行 Agent 曾经进行的某个步骤，帮助开发者定位问题。**

当然，涉及外部 API、时间、随机数等因素时，完全确定性重放并不现实，但可以通过记录输入输出，实现部分重放和差异分析。

它有点像给 Coding Agent 做 Debugger。

## 6. Agent Task Specification & Verification

<badge color="success">产品差异化潜力很大</badge>

**把自然语言需求变成可以验证的开发任务。**

我觉得这是一个很容易被忽视的问题。

开发者给 Agent 的任务通常是：

> 增加一个支持批量删除的 API。

Agent 完成之后说：

> 已经完成，测试全部通过。

但是，谁来判断它是否真的完成了需求？

传统软件工程中，我们有需求文档、验收标准、测试用例、CI。

AI Coding 中，这些东西经常被简化成一句 Prompt。

我设想有一个独立于 Agent 的验证层：

- 任务开始前，明确需求和验收条件。
- 开发过程中，持续检查实现是否偏离需求。
- Agent 声称完成后，独立验证验收条件。
- 最后形成一份任务完成报告，列出满足、未满足和无法验证的条件。

它甚至可以发现：

> Agent 修改了测试用例，使测试通过，但并没有真正实现原始需求。

这个工具的核心不是再增加一个 Code Reviewer，而是建立一个独立于 Coding Agent 的验收机制。

---

# 2. 我对这几个方向的真实判断

我顺便看了一下目前 2026 年的产品生态，有个发现值得特别注意：

**Agent Workspace / Orchestration 已经相当拥挤。**

现在已经有 Birch、Daintree、Braid、Padu 等产品在做多 Agent、Worktree 隔离、任务调度、Session 管理和代码审查。<Cite refs={["turn219154search5","turn219154search8","turn219154search10","turn219154search11"]}/>

Agent Observability 也已经有 Claude 官方 Session Viewer、OpenTelemetry 方案和其他开源工具。<Cite refs={["turn219154search1","turn219154search2","turn219154search12"]}/>

所以我会这样排序：

| 方向 | 我的评价 |
|---|---|
| Agent Workspace Manager | 有需求，但竞争激烈 |
| Agent Behavior Analytics | 有价值，但容易变成数据仪表盘 |
| Agent-native Git | 有技术探索价值，产品定位需要打磨 |
| Agent Compatibility Test Suite | 很有意思，生态型机会 |
| Agent Task Replay | 技术深度高，差异化空间大 |
| Task Specification & Verification | 产品价值明确，值得深入探索 |

# 3. 如果让我选，我会重点探索这三个

<box border radius="lg" gap={3} padding={3}>
  <box gap={1}>
    <row align="center" justify="between">
      **第一名：Agent Compatibility Test Suite**
      <badge color="success">最推荐</badge>
    </row>
    这是我觉得最适合从你现有项目积累出发的方向。你已经接触过多 Agent 的扩展机制，可以从兼容性问题中寻找更大的产品机会。
  </box>
  <divider color="subtle"/>
  <box gap={1}>
    **第二名：Agent Task Replay**

    我喜欢它的技术深度。它可能成为一种新的开发者基础工具，而且不必局限于某一个 Agent。
  </box>
  <divider color="subtle"/>
  <box gap={1}>
    **第三名：Task Specification & Verification**

    我认为这是 AI Coding 工作流中一个很重要的缺口：Agent 可以声称完成任务，但如何独立证明它真的完成了？
  </box>
</box>

# 4. 我还想和你讨论一个更根本的问题

我觉得我们可以暂时不急着在这六个方向里选。

我更想知道你对 AI Coding 的哪一类问题最感兴趣。

{@body const [interest,setInterest] = DIL.useState("workflow")}
{@body const [scope,setScope] = DIL.useState("independent")}
<box border radius="lg" padding={3} gap={3}>
  **问题一：你最想解决哪一类问题？**
  <radio-group direction="col" value={interest} onChange={setInterest}>
    <radio value="workflow">工作流问题：Agent 如何更好地融入日常开发</radio>
    <radio value="reliability">可靠性问题：如何让 Agent 写的代码更可信</radio>
    <radio value="infrastructure">基础设施问题：Agent 的运行环境、上下文、状态管理</radio>
    <radio value="ecosystem">生态问题：不同 Agent 之间的互操作性和扩展能力</radio>
    <radio value="debugging">调试问题：理解、追踪、复现 Agent 的行为</radio>
    <radio value="unexpected">都可以，我更想发现一些目前还没人认真解决的问题</radio>
  </radio-group>
  <divider color="subtle"/>
  **问题二：你希望新项目是什么形态？**
  <radio-group direction="col" value={scope} onChange={setScope}>
    <radio value="independent">独立的开发者工具，CLI / Desktop / Web 均可</radio>
    <radio value="extension">Agent 的插件、Hook、MCP 或扩展生态</radio>
    <radio value="infrastructure">底层基础设施或开发者 SDK</radio>
    <radio value="product">有机会发展成商业化 SaaS 产品</radio>
    <radio value="open-source">主要考虑有趣、有技术挑战的开源项目</radio>
  </radio-group>
  <button block onClick={()=>GenUI.issueNewTurn("我对 AI Coding 项目的偏好：\n最感兴趣的问题："+interest+"\n项目形态："+scope+"\n请基于这些继续讨论项目点子。")}>提交</button>
</box>

我希望最后找到的不是一个「看起来能做」的项目，而是一个让你看到就觉得：**这个问题我自己也经常遇到，而且目前的工具确实没有很好地解决它。**

---

**User:**

我对 AI Coding 项目的偏好：
最感兴趣的问题：unexpected
项目形态：open-source
请基于这些继续讨论项目点子。

---

**Assistant:**

我觉得这个回答反而打开了更多可能性。

既然你更在意「发现目前还没有被认真解决的问题」，而且主要目标是做一个有技术含量的 Open Source 项目，我建议我们换一种思考方式：

**不要从现有 AI Coding 产品的功能列表出发，而是从 AI Agent 改变软件开发方式之后，哪些原本不存在的问题开始出现出发。**

这类问题往往更有意思。因为它们不是把传统开发工具加上 AI，而是 AI Coding 带来的新问题。

我想从几个不太常规的角度发散。

# 一、我认为值得探索的 7 个非传统点子

## 1. Agent Drift Detector：Agent 偏航检测器

<badge color="success">我最感兴趣</badge>

<AsyncImage query="AI software agent task progress monitoring timeline intent drift visualization developer tool" aspectRatio="16:9" width="100%"/>

**问题：Agent 并不一定会突然失败，它可能在完成任务的过程中逐渐偏离目标。**

举个例子：

你要求：

> 给系统增加一个用户头像上传功能。

Agent 最初理解得很好，但执行过程中：

1. 发现现有图片处理逻辑不太好。
2. 开始重构图片处理模块。
3. 发现用户资料模块也有一些问题。
4. 开始修改用户资料模块。
5. 最后完成了上传功能，但顺便修改了 20 多个无关文件。

最终测试通过了，但这真的是你想要的吗？

现在大多数工具更关注最终结果，而不是任务执行过程中是否发生偏航。

我想做一个独立的 Agent Monitor，持续分析：

- 当前 Agent 正在解决什么问题？
- 它现在的工作和原始需求有什么关系？
- 它是否开始处理没有被要求的事情？
- 它是否反复尝试相同的失败方案？
- 它是否忘记了之前明确提出的约束？

甚至可以在 Agent 还没有严重偏航时提醒开发者。

这个方向有一个很有意思的研究依据：2026 年一项针对 20,574 个真实 Coding Agent Session 的研究发现，Agent 与开发者之间的错位经常表现为开发者需要纠正 Agent 的行为，而不是直接造成不可恢复的破坏。<Cite refs={["turn878000academia37"]}/>

我觉得这里有个非常值得研究的问题：

**能不能从 Agent 的行为序列中，提前预测开发者即将说出「不对，我不是让你这么做的」？**

---

## 2. Repo Archaeologist：代码库考古学家

<badge color="success">很有产品想象力</badge>

<AsyncImage query="software repository architecture dependency graph code exploration visualization dark developer interface" aspectRatio="16:9" width="100%"/>

**问题：Agent 每次进入一个陌生代码库，都需要重新探索；但代码库其实有自己的历史和隐含知识。**

想象一个大型项目。

某个模块为什么这么设计？

某个奇怪的兼容逻辑为什么不能删除？

为什么这里没有使用看起来更合理的第三方库？

为什么一个看起来多余的重试逻辑不能简化？

这些信息往往不在 README、代码注释或架构文档里。

它们可能散落在：

- Git 历史。
- 已关闭的 Issue。
- Pull Request 讨论。
- Commit Message。
- 历史代码。
- 已经废弃的实现。
- 开发者过去的决策。

我想做一个工具，把这些信息组织成代码库的历史知识层。

当 Agent 试图修改某段代码时，可以获得：

> 这个逻辑在 2024 年曾经因为某个线上 Bug 被特意保留，不建议直接删除。

这和普通 RAG 不一样。

普通 RAG 是让 Agent 找到相关资料。

Repo Archaeologist 是让 Agent 理解：

**为什么这个代码库会变成今天这个样子。**

---

## 3. Agent Negative Memory：失败经验数据库

<badge>非常值得探索</badge>

**问题：Agent 会记住成功经验，却很难有效记住失败经验。**

假设某个项目中：

- Agent 曾经尝试升级某个依赖，但导致 ABI 不兼容。
- 曾经尝试替换一个数据库查询方式，但性能更差。
- 曾经重构某个模块，但导致循环依赖。
- 曾经使用某个测试方式，但无法覆盖实际问题。

下次换一个 Session，甚至换一个 Agent，这些失败可能又会重新发生。

我想做一个专门记录负面知识的系统。

它保存的不是普通的聊天记录，而是：

| 记录 | 价值 |
|---|---|
| 失败的尝试 | 不要重复走这条路 |
| 不适用的方案 | 说明适用条件 |
| 被回滚的修改 | 保留失败原因 |
| 历史 Bug | 了解容易出错的地方 |
| 被拒绝的设计 | 保留设计决策背景 |

最有意思的是，它可以跨 Agent 工作。

Claude Code 失败过的方案，Codex 不需要重新踩一遍。

我觉得这可能比普通 Memory 更有实际价值。

---

## 4. Agent Sandbox Reality Check：环境真实性检测

**问题：Agent 可能在一个不真实的开发环境里成功完成任务。**

比如：

- Agent 修改了代码，但依赖的服务其实没有真正运行。
- 测试通过了，但测试使用的是 Mock，实际 API 不可用。
- Agent 在本地环境中运行成功，但 CI 环境无法复现。
- Agent 使用了本地缓存中的旧数据。
- Agent 的修改依赖某些没有提交的本地文件。

也就是说：

**Agent 验证的是自己所处的环境，而开发者真正关心的是代码在真实环境中是否成立。**

这个工具可以成为开发环境与 Agent 之间的独立验证层。

我觉得它特别适合开源，因为不同语言、框架、数据库、CI 系统都可能遇到类似问题。

---

## 5. Agent Decision Archaeology：决策溯源

<badge>技术深度高</badge>

**问题：Agent 可以解释它现在的代码，但不一定能解释代码是如何一步步演变到现在的。**

例如，一个函数最终有 150 行。

你想知道：

- 为什么采用这个算法？
- 曾经考虑过其他实现吗？
- 哪些方案被放弃了？
- 为什么最后选择了这个依赖？
- 哪个修改是 Agent 自主决定的，哪个是开发者要求的？

我想做一个能够建立代码与开发决策之间关联的工具。

不是简单的 Git Blame，而是类似：

`需求 → Agent 决策 → 文件修改 → 测试结果 → 开发者反馈 → 最终实现`

最终你可以针对代码中的任意一段，追溯它背后的决策历史。

---

## 6. Agent Contract：Agent 行为契约

**问题：开发者很难精确限制 Agent 的行为边界。**

例如：

> 你可以修改业务代码，但不能修改数据库 Schema。

或者：

> 可以增加依赖，但不能升级已有依赖。

又或者：

> 只允许修改 src/payment 目录，不允许修改其他目录。

现在这些要求通常通过 Prompt、Rules 或权限配置表达。

但自然语言约束很难做到完整、精确和一致。

我想探索一种独立于 Agent 的行为契约：

- 允许做什么。
- 禁止做什么。
- 哪些行为需要审批。
- 哪些文件允许修改。
- 哪些依赖允许引入。
- 哪些操作必须经过验证。

而且不依赖特定 Agent。

可以理解为 Coding Agent 的 Policy Engine。

---

## 7. Agent Swarm Debugger：多 Agent 死锁与冲突分析

**问题：多个 Agent 协作时，不只是文件冲突，还有认知和任务依赖冲突。**

例如：

- A 等待 B 完成接口。
- B 认为 A 会先定义接口。
- C 修改了 A 和 B 都依赖的公共类型。
- D 根据旧接口继续开发。

最后大家都在工作，但整个系统没有进展。

传统的 Multi-Agent Dashboard 只能展示各 Agent 的状态。

我想做的是一个可以分析 Agent 协作关系的 Debugger。

它关注：

- 任务之间的真实依赖关系。
- Agent 是否在等待不存在的前置条件。
- 不同 Agent 是否对同一个接口形成了不同认知。
- 是否出现循环依赖。
- 哪些冲突已经影响了整体进度。

有点像分布式系统的死锁检测和分布式追踪，只不过分析对象变成了 Coding Agent 团队。

---

# 二、我会怎么选择？

如果考虑到你的项目经历、开源属性、技术探索空间，以及能否形成一个真正有辨识度的项目，我会这样排序：

| 项目 | 技术探索 | 差异化潜力 | 我的兴趣 |
|---|---|---|---|
| Agent Drift Detector | 高 | 很高 | ★★★★★ |
| Repo Archaeologist | 高 | 很高 | ★★★★★ |
| Agent Negative Memory | 高 | 高 | ★★★★★ |
| Agent Sandbox Reality Check | 很高 | 高 | ★★★★ |
| Agent Decision Archaeology | 很高 | 高 | ★★★★ |
| Agent Contract | 高 | 中高 | ★★★★ |
| Agent Swarm Debugger | 很高 | 中高 | ★★★★ |

有一点需要说明：这些是值得验证的产品假设，并不代表目前完全没有类似项目。

例如，Agent 行为观测、Session Replay 已经有一些开源实现；Anthropic 也已经专门研究过长时间运行 Agent 的上下文衔接问题。<Cite refs={["turn878000search4","turn878000search0"]}/>

我更希望找到这些现有工具尚未充分解决的下一层问题。

# 三、我有一个更大胆的想法

其实上面 1、3、5 三个方向可以产生一个更有趣的概念：

<box border radius="xl" padding={3} gap={2}>
  <badge color="success">值得重点讨论</badge>
  <title size="2xl">Agent Black Box</title>
  **Coding Agent 的黑匣子与行为分析系统**

  <AsyncImage query="futuristic software debugging black box flight recorder timeline code execution visualization dark interface" aspectRatio="16:9" width="100%"/>

  不只是记录 Agent 做了什么，而是研究 Agent 为什么这么做、什么时候开始偏离，以及它从过去的失败中学到了什么。

  我想象它有三个核心概念：

  - **Observe**：观察 Agent 的完整行为。
  - **Understand**：分析行为背后的决策、偏航和失败。
  - **Learn**：把成功和失败经验转化成可复用知识。

  它可以作为 Claude Code、Codex、OpenCode 等工具的独立配套设施。

</box>

这个方向甚至可以逐步演变成一个研究和实验平台：收集真实 Agent 行为数据，分析不同模型、Harness、项目类型下的行为差异。

我觉得它有机会成为一个有技术影响力的开源项目，而不仅仅是一个小工具。

---

# 四、我想进一步了解你的兴趣边界

我现在不想继续给你堆几十个点子了。我想先判断你对哪种创新最有兴趣。

下面三个问题可能很有帮助：

**1. 你更喜欢哪种项目？**

- A. 一个小而精的 Unix-style 工具，解决一个以前没人注意到的问题。
- B. 一个通用基础设施，成为其他 Agent 工具可以集成的底层组件。
- C. 一个完整的开发者产品，有自己的交互界面和生态。
- D. 一个比较实验性的新概念，甚至需要探索新的软件工程范式。

**2. 你是否愿意做 Agent 之外的配套基础设施？**

例如文件系统、版本管理、进程管理、沙箱、协议、调试器等。Agent 只是它的使用者之一。

**3. 你希望项目的创新程度是哪一种？**

- A. 现有问题，但目前没有好用的解决方案。
- B. 现有工具都没意识到的问题，需要重新定义。
- C. 探索一种 AI Coding 时代才会出现的新开发模式。

我个人尤其希望探索第二种和第三种。

因为我觉得 AI Coding 最值得做的开源项目，未必是让 Agent 写代码更快，而可能是**重新定义我们应该如何管理、理解和验证机器参与的软件开发过程。**

---

**User:**

你最开始提到一种 intend-base 的 code review 工具，这个东西业界有什么类似的项目吗

---

**Assistant:**

有，而且我查了一圈之后发现一个挺有意思的情况：

**你最开始提到的 Intent-based Code Review，已经不是一个纯粹的空白市场了。2026 年已经出现了一批非常接近这个想法的产品。**

但它们的产品定位、Intent 的来源，以及如何验证实现，存在明显区别。

我认为这个方向仍然有探索空间，不过需要把差异化定义得更精确。

# 1. 最接近的竞品

下面是我认为最值得关注的几个。

## ① <Entity value="Revue" category="software" disambig="AI-powered code review product"/> — 最接近传统意义上的 Intent-based Review

<badge color="success">高度相关</badge>

<Link url="https://revue.sh/" title="revue.sh"/>

它有一个非常接近我们讨论的功能：**Intent Drift Detection**。

它的做法是：

- 从 PR 描述中读取 Acceptance Criteria。
- 对每一项 Acceptance Criteria 检查代码 Diff。
- 判断实现是符合、冲突还是无法确认。
- 每个判断都关联到具体代码位置。
- 将结果汇总到 PR Review 中。

例如 PR 描述：

```text
AC1: Login API must rate limit to 5/min.
AC2: Rejected requests must be logged with client IP.
```

它可能给出：

```text
AC1: Aligned
  limits.py:42

AC2: Contradicts
  auth.py:88
```

还有一个我认为非常重要的设计：

它不依赖 Coding Agent 的 Session，而是以 PR 中明确声明的 Acceptance Criteria 为准。

这避免了 Agent 自己解释自己的工作。

<Cite refs={["turn716897search9"]}/>

## ② <Entity value="Aviator Verify" category="software" disambig="AI software delivery and code verification platform"/> — 从 Intent 到可验证证据

<badge color="success">高度相关</badge>

<Link url="https://www.aviator.co/verify" title="Aviator Verify"/>

它比普通 AI Code Review 更进一步。

它将开发任务中的 Intent 作为一个 Contract，然后针对每项标准提供验证证据。

它的理念是：

- Intent：最初约定要实现什么。
- Evidence：每个要求对应的验证结果。
- Decisions：开发者根据证据做最终判断。

例如要求增加 API Rate Limiting：

- 是否限制每个用户的请求频率？
- 超出限制是否返回 429？
- 是否使用参数化 SQL？
- 是否违反团队约定的架构规则？

它不仅让 LLM 阅读 Diff，还尝试使用确定性检查、场景验证和代码扫描。

我认为它已经非常接近我们想象中的产品了。<Cite refs={["turn716897search5","turn716897search13"]}/>

## ③ <Entity value="codingassist.bot" category="software" disambig="Acceptance-criteria-based AI code review"/> — 基于需求工单的 Code Review

<Link url="https://www.codingassist.bot/en" title="codingassist.bot"/>

它的定位比较明确：

**PR 是否真正满足对应 Jira / Azure Boards 工单里的 Acceptance Criteria？**

它会：

1. 获取 PR。
2. 找到关联的 Work Item。
3. 提取需求及验收标准。
4. 对照代码变更进行分析。
5. 在 PR 上发布总结、行内问题和 Status Check。

这个产品说明，Intent-based Review 不只是一个概念，已经有团队在将它作为独立商业产品销售。<Cite refs={["turn716897search4"]}/>

## ④ <Entity value="Norma" category="software" disambig="AI specification-to-code verification tool"/> — 检查实现与 Spec 是否一致

<Link url="https://norma.sh/" title="norma.sh"/>

这个产品的 Slogan 很直接：

> The PR passed. The requirement didn't.

它的核心是：

- 输入软件需求或 Spec。
- 分析当前代码实现。
- 发现 Spec 与代码之间的偏差。
- 给出修复建议，甚至生成修复 Prompt。

它举的例子是：

需求规定每日提款上限为 $500，但实现中写成了 $5,000。

普通测试和 Code Review 可能都没有发现这个问题，但 Spec Fidelity 检查可以发现。

这其实就是我们之前讨论的核心价值：代码本身看起来没问题，不代表它实现了正确的东西。<Cite refs={["turn716897search6"]}/>

## ⑤ <Entity value="Tiden" category="software" disambig="Independent verification platform for coding agents"/> — 从需求到交付的闭环

<Link url="https://www.tiden.ai/" title="tiden.ai"/>

它的范围比 Code Review 更大。

它尝试维护一个长期存在的 Requirements Repository，并把需求与代码、测试关联起来。

它的工作流包括：

- 捕获开发者的原始 Intent。
- 更新和维护 Spec。
- 将 Diff 映射到相关需求。
- 找出测试覆盖缺口。
- 执行验证。
- 根据验证结果决定是否满足交付条件。
- 将生产环境中的异常反馈到需求和测试中。

它支持 Claude Code、Codex、Cursor 等工具。

我觉得它值得研究，因为它不把 Intent 当作一次性的 Prompt，而是作为持续演进的产品知识。<Cite refs={["turn716897search1"]}/>

## ⑥ <Entity value="Reqode" category="software" disambig="Requirements and verification platform for AI coding agents"/> — 产品模型与实现的一致性

<Link url="https://reqode.com/" title="reqode.com"/>

它把产品需求、API、数据模型、UI、架构等组织成关联的产品模型。

Agent 开始开发时获得相关上下文，开发完成后再由 Verifier 检查实现是否符合这个模型。

它关注的不仅是单个 PR，还包括长期的 Architecture Drift。

<Cite refs={["turn716897search11"]}/>

---

# 2. 开源生态也已经有一些相关项目

如果你考虑做 Open Source，这部分尤其值得看。

| 项目 | 主要关注点 | 与我们想法的关系 |
|---|---|---|
| <Entity value="GitHub Spec Kit" category="software" disambig="Open-source spec-driven development toolkit"/> | Spec-driven development | 需求先行 |
| <Entity value="OpenSpec" category="software" disambig="Open-source specification-driven development toolkit"/> | 维护和验证变更 Spec | Intent 的结构化 |
| <Entity value="SpecD" category="software" disambig="Open-source spec-driven development framework"/> | Spec、代码图、Agent 工作流 | 需求与代码关联 |
| <Entity value="Supersigil" category="software" disambig="Open-source specification and verification tool"/> | Spec、证据、CI Drift Detection | 持续验证 |

其中我建议你重点看：

- [GitHub Spec Kit](https://github.com/github/spec-kit)
- [SpecD](https://github.com/specd-sdd/SpecD)
- [Supersigil](https://supersigil.org/)

SpecD 比较有意思，它把 Spec、代码和依赖关系组织成可查询的图，并且提供 CLI、MCP 和 Agent 插件接口。Supersigil 则强调 Spec 与代码一起版本管理，以及 CI 中的 Drift Detection。<Cite refs={["turn716897search14","turn716897search3","turn716897search16"]}/>

注意，这些更多属于 Spec 管理和 Spec-driven Development，不完全等价于 PR 阶段的 Intent-based Code Review。

# 3. 这些产品实际上有什么区别？

我觉得最关键的区别不是它们使用什么模型，而是三个维度：

| 产品 | Intent 从哪里来 | 主要验证方式 | 主要定位 |
|---|---|---|---|
| Revue | PR 描述 | LLM + 代码证据 | PR Review |
| Aviator Verify | 开发任务 Contract | 确定性检查 + 场景验证 | 交付验证 |
| codingassist.bot | Jira / Azure Boards | LLM + 规则 | 需求验收 |
| Norma | Spec | LLM 分析 | Spec Fidelity |
| Tiden | Agent Session + 长期 Spec | 测试 + 验证循环 | 完整交付闭环 |
| Reqode | 产品模型 | AI Verifier | 架构与需求一致性 |

我认为这里还有一个值得研究的问题：

**这些产品虽然都在验证 Intent，但 Intent 本身究竟应该是什么？**

比如同样一个任务：

> 增加用户注销功能。

它实际上包含很多不同层面的约束：

- Functional Intent：用户能够注销。
- Behavioral Intent：注销后旧 Token 失效。
- Security Intent：不能通过伪造请求注销其他用户。
- Architectural Intent：不能绕过现有认证模块。
- Negative Intent：不能删除用户的历史订单。

只对照功能需求进行 Review，很容易遗漏其他约束。

# 4. 如果是我来做开源项目，我会怎么寻找差异化？

我不会直接做另一个 Revue。

因为单纯的：

`PR Diff + Acceptance Criteria + LLM Review`

已经有相当接近的产品了。

我更感兴趣的是下面三个尚值得深入验证的方向：

<box border radius="lg" padding={3} gap={3}>
  <box gap={1}>
    <badge color="success">方向 A</badge>
    **Intent Traceability**

    不仅知道需求有没有实现，还能追踪每一项需求最终对应哪些代码、测试和行为。

    比如：

    `Requirement → Code → Test → Evidence`

    当某个需求被修改时，能够识别哪些实现和测试需要重新验证。
  </box>
  <divider color="subtle"/>
  <box gap={1}>
    <badge color="success">方向 B</badge>
    **Intent Integrity**

    检查 Agent 有没有在开发过程中悄悄改变需求本身。

    例如，原本要求 Token 失效，但 Agent 为了让测试通过，把对应测试删掉了。

    这不仅是代码错误，而是需求与验收标准被破坏。
  </box>
  <divider color="subtle"/>
  <box gap={1}>
    <badge color="success">方向 C</badge>
    **Independent Behavioral Verification**

    不相信 Agent 的完成报告，也不完全依赖它自己编写的测试。

    从需求中独立推导验证场景，再检查真实程序行为。

    特别关注边界条件、异常路径、负向约束，以及 Agent 没有主动提及的行为。
  </box>
</box>

我个人最看好 B + C 的结合。

因为传统 Code Review 主要检查代码，传统测试主要检查预先定义的行为，而 AI Coding 带来了一个新的风险：

**写代码的 Agent 同时也在影响它用来证明自己正确的证据。**

这值得专门研究。

---

# 5. 我的建议：先深入研究这三个产品

如果你准备认真考虑这个方向，我建议先实际体验：

1. [Revue](https://revue.sh/)：研究最直接的 Intent Drift Review。
2. [Aviator Verify](https://www.aviator.co/verify)：研究如何将 Intent 转换成可审查的验证证据。
3. [SpecD](https://github.com/specd-sdd/SpecD)：研究开源 Spec、代码图和 Agent 集成方式。

我想特别强调：目前看到的产品介绍和公开资料还不足以证明它们在复杂真实项目中的验证准确率。不要仅凭 Demo 判断谁已经解决了这个问题。

**我想反过来问你一个问题：**

你觉得下面哪种产品更有吸引力？

- **A. Intent Review**：PR 创建后，检查代码是否符合原始需求。
- **B. Intent Verification**：不仅 Review，还独立生成验证场景，证明实现符合需求。
- **C. Intent Integrity**：追踪整个 Agent Coding 过程，确保 Agent 没有偷换需求、修改验收标准或规避约束。

这三个方向看起来相似，但实际上是三种不同的产品。我觉得选对切入点比先决定用什么技术重要得多。

---

**User:**

从我的感受来说。我 code review 的时候感受到最大的问题是我看 diff 没有层次。因为我自己写代码肯定不是从第一个文件第一行开始改，到最后一行，然后改第二个文件。而是比如说我要 db 加个字段，然后改好几个文件，好几个位置。然后我要接口加个字段，然后改好几个文件，好几个位置。如果 AI 写代码时也是这样，那我希望能看到他一批批的修改，每一批应该告诉我为什么要改这几个东西。然后所有批次 diff 叠起来等于最终的 diff ，我每个批次看了没问题那最终的 diff 就没问题。现在又类似这样的东西吗

---

**Assistant:**

我觉得你这次描述的东西，比我们之前讨论的 Intent-based Code Review 更具体，而且我认为是一个非常好的产品切入点。

我先说我的理解：你真正不满意的不是 Diff 太多，而是**Diff 的组织方式不符合人类理解软件变更的方式。**

现在 Git Diff 通常按照文件组织：

```text
src/models/user.go
src/api/user.go
src/service/user.go
src/repository/user.go
src/tests/user_test.go
```

但开发者理解一个功能时，通常按照变更意图组织：

```text
Batch 1: 为 User 增加 age 字段
  ├── 修改数据库 Schema
  ├── 修改 User Model
  ├── 修改 Repository
  └── 增加数据库迁移测试

Batch 2: 在用户查询 API 中增加 age 字段
  ├── 修改 API Response
  ├── 修改 DTO
  ├── 修改序列化逻辑
  └── 增加 API 测试

Batch 3: 增加 age 的校验逻辑
  ├── 修改 Validator
  └── 增加边界测试
```

你想要的是：

1. Agent 把最终修改拆成有意义的批次。
2. 每个批次都有明确的修改意图和原因。
3. 每个批次有独立的 Diff，可以单独 Review。
4. 所有批次合并后，严格等于最终工作区的完整 Diff。
5. Review 一个批次时，可以理解它与其他批次的关系。

我会把这个概念叫作：

**Intent-based Change Sets（基于意图的变更集）。**

它不是简单的文件分组，而是把一次开发任务产生的代码修改重新组织成一系列可审查的逻辑变更。

这和传统 Code Review 有一个本质区别：

传统 Review 问的是：这些文件改了什么？

你的想法问的是：**为了完成这个开发任务，开发者做了哪些逻辑上独立的事情？**

# 1. 找到了两个非常接近你想法的项目

而且其中一个几乎就是你描述的产品。

## AgentDiff

<badge color="success">最接近你的想法</badge>

<row align="center" gap={2}>
  <AsyncImage query="GitHub AgentDiff natiixnt project logo" aspectRatio="1:1" maxWidth="64px"/>
  <box flex="1" gap={0}>
    **<Entity value="AgentDiff" category="software" disambig="Open-source AI coding agent diff viewer"/>**
    <text color="secondary" size="xs">Open Source · MIT License</text>
    <Link url="https://github.com/natiixnt/AgentDiff" title="GitHub Repository"/>
  </box>
</row>

它的项目介绍几乎直接命中了你的痛点：

> Turning raw line diffs into higher-level change intent.

它目前支持：

- 将多个文件的修改分成逻辑上的 Change Sets。
- 识别 Rename、Refactor、Extraction、Behavior Change 等修改类型。
- 为每组修改提供风险标记和置信度。
- 推荐 Review 顺序。
- 将执行计划和最终 Diff 对照，发现计划中提到但没有修改的内容，以及没有计划却发生的修改。
- 将相关文件放在一起展示。

例如，它的 Demo 会把认证相关修改放在一起，把 Schema 和 Config 修改识别出来。<Cite refs={["turn444614view0"]}/>

但有一个非常重要的区别：

**AgentDiff 目前主要是在最终 Diff 上进行语义分组，而不是严格地将 Agent 的整个修改过程拆成多个独立、可累加的 Patch。**

从 README 看，它的核心流程是：

`Git Diff → Pattern Detection → Classification → Grouping → Review UI`

它也支持传入 Execution Plan，但目前没有证据表明它能够保证每组 Patch 都是精确、互不重叠、合并后严格等于原始 Diff 的变更集。

而且它还处于很早期的阶段：公开仓库目前只有 15 个 Commits，README 中的截图还是占位内容。<Cite refs={["turn444614view0"]}/>

## Agent-Diff-Visualizer (ADV)

<badge color="success">也非常接近</badge>

<row align="center" gap={2}>
  <AsyncImage query="Visual Studio Code extension icon code diff review" aspectRatio="1:1" maxWidth="64px"/>
  <box flex="1" gap={0}>
    **<Entity value="Agent-Diff-Visualizer" category="software" disambig="Open-source VS Code AI code review extension"/>**
    <text color="secondary" size="xs">Open Source · MIT License</text>
    <Link url="https://marketplace.visualstudio.com/items?itemName=Galadriel-Tech-Solutions.agent-diff-visualizer" title="VS Code Marketplace"/>
  </box>
</row>

这个项目的功能甚至更接近你想要的交互方式：

- Semantic Task Clustering：按逻辑任务组织修改。
- Intent Mapping：从 Cline、Aider 日志提取原始 Prompt 和 Agent Reasoning。
- Intent Drift Detection：发现修改超出原始任务范围。
- Review Status：每组修改可以标记 Pending、Approved、Rejected、Read。
- Atomic Reversion Timeline：按 Agent 执行步骤回溯和回滚。

尤其值得注意的是，它已经有一个 Review Panel，左边是语义分组，右边是 Intent、依赖关系和执行时间线。<Cite refs={["turn444614view1"]}/>

不过它的 README 将 Multi-step Diff Preview 和 Step-level Edit 列为后续规划功能。这意味着它目前也没有完全实现你所说的那种严格的分批 Patch Review。

---

# 2. 还有两个容易混淆的相关概念

| 工具 | 做什么 | 与你想法的区别 |
|---|---|---|
| <Entity value="Graphite" category="software" disambig="Stacked pull request code review platform"/> | Stacked PR | 人为将代码拆成多个依赖的 PR |
| <Entity value="GitButler" category="software" disambig="Git branch and change management tool"/> | Virtual Branches | 人为将文件和代码块分配到不同分支 |

Graphite 的 Stacked Diffs 很值得研究。它把一个大任务拆成多个有依赖关系、可以分别 Review 的小变更。<Cite refs={["turn566546search0","turn566546search6"]}/>

GitButler 更有意思：它允许在同一个工作区里同时维护多个 Virtual Branch，甚至可以将同一个文件的不同代码块分配给不同分支。<Cite refs={["turn566546search4"]}/>

但这两者主要解决的是版本控制和变更组织问题，并不是自动理解 Agent 的修改意图。

# 3. 有一个发现，我觉得特别重要

我查到了 JetBrains Research 在 **2026 年 10 月**刚发布的一篇研究文章：

[Our Framework for Reviewing AI-Generated Code](https://blog.jetbrains.com/research/2026/10/review-ai-generated/)

它和你的观察高度一致。

文章提出，传统 Diff Viewer 已经不太适合 AI 生成的大规模代码修改。研究团队提出了一种三层 Review 模式：

1. **Overview**：先理解整体修改意图。
2. **Zoom & Filter**：选择需要重点检查的修改部分。
3. **Details on Demand**：深入查看具体代码。

文章还特别指出，Graphite 的 Stacked PR 是目前比较接近的 Chunk Review 方案，但它需要把修改拆成多个 PR，而不是自动把一个 Agent 产生的完整修改拆成逻辑批次。

研究基于 17 位开发者参与的设计研究，以及 43 位软件从业者的后续调查。<Cite refs={["turn444614view2"]}/>

这让我觉得你的直觉很值得认真对待。

# 4. 我认为真正的产品机会在哪里？

我会把你的需求进一步定义成：

<box border radius="lg" padding={3} gap={3}>
  <title size="xl">Semantic Patch Review</title>
  <text color="secondary" size="sm">一种面向 AI Coding Agent 的新型代码审查方式</text>
  <box border radius="md" padding={3} gap={1}>
    **Final Diff**
    <text color="secondary" size="xs">整个 Agent 任务产生的完整修改</text>
    <title size="xl" tabularNums>+1,284 / -326</title>
  </box>
  <box align="center">
    <icon name="arrow-down" color="secondary" size="xl"/>
  </box>
  <box border radius="md" padding={3} gap={2}>
    <row justify="between" align="center">
      **Batch 1 · Database Schema**
      <badge color="success">Approved</badge>
    </row>
    增加用户年龄字段
    <text color="secondary" size="xs">3 files · Model / Migration / Repository</text>
    <divider color="subtle"/>
    <row justify="between" align="center">
      **Batch 2 · API Contract**
      <badge color="success">Approved</badge>
    </row>
    用户查询接口增加 age 字段
    <text color="secondary" size="xs">4 files · DTO / Handler / Response / Tests</text>
    <divider color="subtle"/>
    <row justify="between" align="center">
      **Batch 3 · Validation**
      <badge color="warning">Pending</badge>
    </row>
    增加年龄字段校验
    <text color="secondary" size="xs">2 files · Validator / Tests</text>
  </box>
  <box align="center">
    <icon name="arrow-down" color="secondary" size="xl"/>
  </box>
  <box background="surface-secondary" radius="md" padding={3} gap={1}>
    <row align="center" gap={2}>
      <icon name="check-circle" color="success"/>
      **完整修改集**
    </row>
    <text color="secondary" size="xs">所有 Batch 的变更精确组合，还原原始 Final Diff</text>
  </box>
</box>

我认为有三个关键设计原则：

**第一，完整性。**

每一行修改都必须归属一个 Batch，不能遗漏，也不能重复计算。所有 Batch 合并后必须精确还原原始 Diff。

**第二，逻辑性。**

一个 Batch 可以跨越多个文件、多个目录、多个代码层次。不能简单按文件分组。

**第三，可审查性。**

每个 Batch 都需要解释为什么这些修改属于同一件事，以及它与其他 Batch 有什么依赖关系。

尤其是第三点：如果只是 AI 给文件分组，价值有限。真正有用的是它能解释一个逻辑变更的完整上下文。

---

# 5. 我想追问你一个关键问题

这会直接决定这个项目应该怎么定位。

假设 Codex 完成一个任务，修改了 20 个文件。

你更希望它如何产生这些 Batch？

**A. 开发过程中实时产生**

Agent 每完成一个逻辑变更，就产生一个 Batch。

例如 Agent 先完成数据库字段变更，保存 Batch 1；再完成 API 修改，保存 Batch 2。

**B. 开发完成后自动分析**

Agent 正常工作，不需要改变现有工作流。完成后，工具分析最终 Diff，自动识别逻辑变更并分组。

**C. 开发过程中记录，完成后重新组织**

工具记录 Agent 每一步的修改、Prompt、工具调用和上下文，最终根据这些信息，将整个任务整理成逻辑 Batch。

**我个人最看好 C。**

因为 B 只能从最终代码推测开发意图，而 C 有机会知道 Agent 当时为什么修改这些文件。

但我也很好奇：你是否希望一个 Batch 就是一个可以独立 Commit、独立回滚的 Git Changeset，还是它只需要是一个 Review 分组，不改变实际 Git 历史？

这两个问题的答案，会让产品走向很不一样的方向。

---

**User:**

没错，你最后问我的问题正是这个想法的关键之处。我也没有完全想好，但是我可以跟你说下我的想法：
首先分批动作肯定是在代码编写过程中进行，而不是编写完成后事后分析。因为事后分析不一定能还原原始动机。
然后一种简单的方法是通过 hook 捕获 agent 工具每次 write / edit / bash 等可能导致文件变化的工具调用，计算工具调用前后的 diff ，这就是自然分批，很多 agent 工具在调用工具时自然会提供一个为什么要调用的原因说明，这就是个意图可以采集下来。这种方式几乎不影响 agent 工作。但是由于 agent 不知道这些意图和分批是用于 cr 的，可能不会组织得非常有条理。原因可能过于简短，分批也可能过于零碎。比如有几个文件的编辑其实是属于同一个意图的，但是 agent 可以分几次进行编写。这会导致分批太零碎，最后很难 review 。
另一个极端是所有 diff 和原因要求 agent 生成，通过在适当位置添加 hook ，要求 agent 按固定格式记录 diff 和原因，并且工具与实际 diff 进行对比检查，不通过的话要求 agent 改正。这样可能会产出更好的分批和原因说明，但是这过度干扰了 agent 的工作，而且浪费 token 。更重要的是这其实仍然是事后补充，只不过这个“事后”更提前一点。 agent 完全可能为了显得合理编一个原因说明，而不是如实记录它最初的动机。
所以最终合理的方式可能是这两者折衷。静默采集原始数据，然后适当时候提示 agent review ，进行局部合并和梳理再提交。我觉得可能这样更好。
你有什么想法。

---

**Assistant:**

我非常赞同你的折衷方案。而且我觉得你已经触及了这个产品最核心的设计问题：

**如何在不改变 Agent 原有工作习惯的前提下，获得足够真实的开发过程信息，并将它转化为高质量的 Review 单元。**

我尤其认同你对两个极端的批评：

- 纯 Hook 记录：数据真实，但缺少组织。
- 强制 Agent 记录：组织可能很好，但容易干扰工作，而且记录本身可能失真。

不过，我想在你的折衷方案上再推进一步。我认为这里有一个很重要的设计原则：

**原始事实的采集，与 Review 结构的组织，应该是两个完全独立的过程。**

前者必须尽可能客观、自动、不可篡改；后者可以由 Agent 参与，可以合并、拆分、修改，甚至重新组织。

我会把整个系统分成三个层次。

# 1. 第一层：Raw Change Events

<badge color="success">必须保证真实性</badge>

这一层就是你说的静默采集。

每次 Agent 执行可能改变文件的操作时，记录一个 Change Event。

例如：

```text
Event #001
Tool: Edit
File: models/user.go

Before:
  type User struct {
      ID int
  }

After:
  type User struct {
      ID  int
      Age int
  }

Agent Message:
  Add age field to User model.
```

接下来：

```text
Event #002
Tool: Edit
File: migrations/001.sql

Before:
  CREATE TABLE users (id INTEGER);

After:
  CREATE TABLE users (
      id INTEGER,
      age INTEGER
  );

Agent Message:
  Add age column to users table.
```

再接下来：

```text
Event #003
Tool: Edit
File: repository/user.go

...
```

这些事件不需要 Agent 特意为 Review 做任何事情。

我认为这一层应该遵循几个原则：

- 尽量不增加 Agent 的 Token 消耗。
- 不要求 Agent 修改工具调用习惯。
- 原始修改内容以实际文件变化为准，而不是相信 Agent 的描述。
- 保留时间顺序和原始工具调用信息。
- 原始记录一旦产生就不被后续整理操作覆盖。

有一个细节我觉得特别重要：

**Agent 提供的修改原因，只是证据的一种，而不是事实本身。**

例如：

```text
Agent 声明：
Add age field to User model.
```

这可以作为原始意图记录。

但如果 Agent 没有提供原因，也不能因此丢失这个 Change Event。

还有一点：这里应该记录 Agent 明确输出的工具调用说明、Prompt 和相关上下文，而不是试图获取模型隐藏的内部推理过程。我们需要的是可审计的开发证据，不是模型的私有思维链。

另外，Bash 操作比较特殊。它可能修改多个文件，甚至启动后台进程。因此不能只关注 Edit / Write 工具，还需要关注实际工作区变化，否则会产生采集盲区。

---

# 2. 第二层：Intent Groups

<badge color="success">真正产生产品价值的地方</badge>

这一层才是我认为需要 Agent 参与的地方。

原始事件可能有 100 个，但最终 Review 可能只需要 8 个 Batch。

例如：

<box border radius="lg" padding={3} gap={2}>
  **Raw Change Events**
  <text color="secondary" size="xs">按照真实发生顺序记录</text>
  {#each [{id:"E01",name:"修改 User Model"},{id:"E02",name:"修改 User Migration"},{id:"E03",name:"修改 User API DTO"},{id:"E04",name:"修改 User Repository"},{id:"E05",name:"增加 User Model 测试"},{id:"E06",name:"修改 User Response"},{id:"E07",name:"修改 Age Validator"}] as e}
    <row align="center" gap={2}>
      <badge>{e.id}</badge>
      <text size="sm">{e.name}</text>
    </row>
  {/each}
  <divider color="subtle"/>
  <row justify="between" align="center">
    **整理后的 Intent Groups**
    <badge color="success">3 Batches</badge>
  </row>
  <box background="surface-secondary" radius="md" padding={2} gap={1}>
    **Batch 1 · Database Schema**
    <text color="secondary" size="xs">E01、E02、E04</text>
    增加用户年龄字段及其持久化支持。
  </box>
  <box background="surface-secondary" radius="md" padding={2} gap={1}>
    **Batch 2 · API Contract**
    <text color="secondary" size="xs">E03、E06</text>
    让用户查询接口能够返回年龄字段。
  </box>
  <box background="surface-secondary" radius="md" padding={2} gap={1}>
    **Batch 3 · Validation**
    <text color="secondary" size="xs">E05、E07</text>
    为年龄字段增加校验和测试。
  </box>
</box>

这里有一个重要区别：

Agent 不再负责解释每一次修改，而是负责理解多个已经发生的修改之间的关系。

它需要回答：

1. 哪些 Change Events 实际上属于同一个开发意图？
2. 为什么这些修改应该放在一起？
3. 不同 Batch 之间有什么依赖关系？
4. 有没有无法归类的修改？
5. 是否存在一个 Batch 内部其实包含了多个独立意图？

我认为这比要求 Agent 每次修改文件时都写一份完整说明合理得多。

**它不是要求 Agent 记录开发过程，而是让 Agent 帮助整理已经记录下来的开发过程。**

# 3. 第三层：Review Projection

这一层我认为是整个产品最需要认真设计的部分。

它的任务是把 Intent Groups 转换成真正可以 Review 的 Diff。

这里有一个很重要的要求：

\[
\bigcup_{i=1}^{n} BatchDiff_i = FinalDiff
\]

并且每个最终修改只能被归属一次，不能重复，也不能遗漏。

但是这里有个容易忽视的问题。

**Raw Change Events 的 Diff，并不能直接拿来相加。**

例如：

```text
Event 1:
  将 timeout 从 10 改为 20

Event 2:
  将 timeout 从 20 改为 30
```

最终 Diff 其实只有：

```diff
- timeout = 10
+ timeout = 30
```

而不是两个独立的 Diff。

再比如：

```text
Event 1:
  新增一个函数 A

Event 2:
  删除函数 A
```

最终 Diff 中根本没有这个函数。

因此我建议明确区分：

- Raw Event Diff：记录每次工具调用实际产生的变化。
- Final Diff：任务最终相对于基准版本的变化。
- Batch Diff：从 Final Diff 中分配给某个逻辑意图的变化。

最终 Review 应该以 Batch Diff 为准，而 Raw Event Diff 用来解释这些修改是怎么产生的。

这样就能同时满足两个要求：

1. 保留真实的历史过程。
2. 保证所有 Batch 精确还原最终代码修改。

另外，Batch 之间可能有依赖关系。

例如：

```text
Batch 1: Database Schema
    ↓
Batch 2: Repository
    ↓
Batch 3: API
```

这时我不建议假装它们是完全独立的 Patch，而是允许形成一个有序的 Changeset Stack。

每个 Batch 都有自己的 Diff，但也可以看到它依赖的前置 Batch。

这和 Git Stacked Diffs 有点相似，不过这里的分组是 Agent 工作过程中自然产生的，而不是要求开发者自己维护多个分支。

---

# 4. 什么时候让 Agent 参与整理？

这是你提出的折衷方案里，我觉得还可以进一步优化的地方。

我不建议每隔几次 Tool Call 就让 Agent 整理一次。

这会让 Agent 不断切换工作模式，而且容易打断正常开发。

我更倾向于使用几个自然的时机：

| 时机 | 行为 |
|---|---|
| Agent 完成一个明显的子任务 | 可选地整理当前事件 |
| Agent 即将结束整个任务 | 执行一次完整整理 |
| Agent 即将进行上下文压缩 | 保存当前阶段的结构化进度 |
| 用户主动请求 Review | 立即生成 Review 视图 |

其中最重要的是任务结束时。

例如 Agent 完成开发，准备向用户汇报：

```text
Implementation completed.
```

此时工具可以让它进入一个轻量的 Review Preparation 阶段：

```text
I have collected 37 change events.

Please organize them into logical review batches.

For each batch:
- Identify its intent.
- Reference the original change events.
- Explain why they belong together.
- Identify dependencies on other batches.
- Flag ambiguous or unrelated changes.

Do not modify source files.
Do not invent historical reasons.
```

注意，我认为这个阶段应该是只读的。

Agent 可以整理分组、编写说明，但不能修改原始代码，也不能改变原始事件记录。

而且，如果 Agent 给出的理由只是根据代码推测出来的，就应该明确标注为推断，而不是原始动机。

这样就不会让 Review Preparation 反过来污染开发过程。

# 5. 我还想增加一个设计：Intent 的可信度

我认为 Agent 生成的 Batch 说明，不能全部被当作同等可信的事实。

例如：

<box border radius="lg" padding={3} gap={3}>
  <box gap={1}>
    <row align="center" justify="between">
      **Explicit Intent**
      <badge color="success">可信度高</badge>
    </row>
    Agent 在执行工具调用时明确提供了说明：

    > Add age field to User model.
  </box>
  <divider color="subtle"/>
  <box gap={1}>
    <row align="center" justify="between">
      **Context-derived Intent**
      <badge color="info">有上下文依据</badge>
    </row>
    Agent 没有明确说明，但原始任务要求增加用户年龄，且这些修改与任务相关。
  </box>
  <divider color="subtle"/>
  <box gap={1}>
    <row align="center" justify="between">
      **Inferred Intent**
      <badge color="warning">需要人工确认</badge>
    </row>
    Agent 在任务结束后，根据多个文件的修改推测：

    > These changes appear to implement user age validation.
  </box>
</box>

我认为这会是一个很好的产品特性。

因为 Review 的一个重要价值，就是让开发者知道自己正在审查什么，以及为什么要修改这些代码。

如果连修改原因都是事后编出来的，那么这个产品就失去了部分意义。

# 6. 我觉得还可以做一个很有意思的交互

假设 Agent 已经完成了任务，生成了 8 个 Batch。

你 Review 到 Batch 3，发现它有问题。

你可以：

- Approve：接受这个逻辑变更。
- Request Changes：要求 Agent 修改这个 Batch。
- Split：把一个 Batch 拆成两个。
- Merge：把两个 Batch 合并。
- Reassign：将部分 Diff 移到其他 Batch。

尤其是 Request Changes：

你不必再对 Agent 说：

> 请修改 user.go 第 42 行和 repository.go 第 18 行。

而是直接说：

> Batch 3 的年龄校验逻辑不正确，应该允许 null，但不允许负数。

Agent 可以针对这个逻辑变更继续工作。

修改完成后，原来的 Batch 更新，同时保留修改前后的 Review 历史。

这样 Review 就不再只是开发结束后的检查，而成为 Agent Coding 工作流中的一个环节。

---

# 7. 我现在对这个项目的定位

我觉得我们已经可以给它一个比较准确的定义：

<box border radius="xl" padding={3} gap={2}>
  <title size="2xl">Intent-based Code Review</title>
  <title size="lg" color="secondary" weight="medium">Review code the way developers think.</title>
  把 AI Agent 的原始代码修改过程，转化成一系列有明确意图、可独立审查、可追溯的逻辑变更集。

  核心能力：

  1. **Capture** — 静默捕获 Agent 的实际代码修改。
  2. **Organize** — 将零散修改组织成逻辑 Batch。
  3. **Review** — 按开发意图而不是文件顺序审查 Diff。
  4. **Verify** — 保证 Batch 完整覆盖最终 Diff。
  5. **Feedback** — 将 Review 意见反馈给 Agent，形成迭代。

</box>

我甚至觉得，这个产品不一定需要自己实现复杂的 Code Review UI。

它可以首先作为一个独立的 CLI / Hook 工具，采集和组织修改，再通过一个轻量 Web UI 展示 Review。

支持 Codex、Claude Code、OpenCode 等工具很重要，但更重要的是建立一套独立于具体 Agent 的 Change Event 和 Intent Batch 数据模型。

这样它才有机会成为一个通用的开源工具。

# 8. 最后，我想讨论一个我认为比前面所有问题都重要的细节

**一个 Batch 究竟应该有多大？**

假设 Agent 完成一个功能：

```text
Batch 1: Database Schema
Batch 2: Repository
Batch 3: API DTO
Batch 4: API Handler
Batch 5: Tests
```

这是按代码层次拆分的。

但也可以是：

```text
Batch 1: Add user age field (Database + Repository + API)
Batch 2: Validate user age (Validator + Tests)
```

这是按开发意图拆分的。

我认为第二种更符合你的初衷，但实际情况可能更复杂：

- 一个 Batch 太大，Review 仍然很困难。
- 一个 Batch 太小，就失去了跨文件理解的价值。
- 有些修改天然属于同一意图，但又存在明显的先后依赖。

所以我不确定是否应该让 Agent 自由决定分组，还是让系统提供某种粒度约束。

我目前倾向于：

**以完整的逻辑意图为主 Batch，在 Batch 内部再允许划分 Sub-changes。**

这样既保留了跨文件的整体语义，也不会强迫开发者一次阅读过多代码。

我想听听你的看法：你更希望一个 Batch 对应一个完整的功能性意图（例如「增加用户年龄支持」），还是对应一个可以独立理解和验证的小修改（例如「增加数据库字段」）？

这个决定会直接影响我们这个项目最核心的数据模型和 Review 体验。

---

**User:**

我觉得分批的粒度问题我现在很难回答你，我得看到具体的效果才能评估，我觉得这可以作为一个后续优化点，在 MVP 中可以不考虑这个问题。甚至后面可以做到用户自由选择粒度，基于 raw change 产出不同粒度合并结果也不是不可能。
不如这样，你把上面我们讨论的关于这个项目的想法整理为一份 markdown 格式的设计文档给我，我想想怎么把它落地。

---

**Assistant:**

我把我们讨论的内容整理成了一份初步产品设计文档。重点保留了你的原始思路，没有过早确定实现架构，也没有把 Batch 粒度问题强行定下来。

我把它定位为 **Product Design Draft / MVP Proposal**，方便你后续继续修改和补充。

<WritingBlock id="83162" variant="document">
# Intent-based Code Review

> Review code the way developers think.

**文档类型：** Product Design Draft  
**项目状态：** Concept / Pre-MVP  
**项目定位：** 面向 AI Coding Agent 的语义化代码审查工具

---

## 1. 项目背景

### 1.1 当前 AI Coding 的 Code Review 问题

随着 Codex、Claude Code、OpenCode 等 AI Coding Agent 的普及，开发者越来越频繁地需要 Review Agent 产生的代码修改。

目前主流的 Code Review 工具基本都以 Git Diff 为核心，按照文件和代码行展示修改。

但这种组织方式并不符合开发者理解软件变更的习惯。

例如，开发者需要为用户增加一个 `age` 字段，可能需要修改：

- 数据库 Schema
- Database Migration
- User Model
- Repository
- API DTO
- API Handler
- Tests

这些修改虽然分散在多个文件、多个位置，但实际上共同完成了一件事。

开发者理解这个变更时，通常会将它们视为一个完整的逻辑修改，而不是七个独立的文件修改。

传统 Diff Viewer 无法很好地表达这种逻辑关系。

### 1.2 核心问题

**Git Diff 按照文件组织代码修改，而开发者按照开发意图理解代码修改。**

AI Agent 产生的代码修改可能涉及大量文件和代码位置。如果仍然按照传统文件顺序进行 Review，开发者需要不断在不同文件之间切换，自行理解这些修改之间的关系。

这增加了 Review 的认知负担，也容易遗漏跨文件的逻辑问题。

### 1.3 产品愿景

提供一种新的 Code Review 方式：

**将 AI Agent 一次开发任务产生的完整代码修改，组织成一系列具有明确开发意图的逻辑变更批次（Intent Batches），让开发者按照逻辑变更而不是文件顺序进行 Review。**

例如：

```text
Final Diff
  ├── models/user.go
  ├── migrations/001.sql
  ├── repository/user.go
  ├── api/user_dto.go
  ├── api/user_handler.go
  └── tests/user_test.go
```

经过组织后：

```text
Batch 1: Add user age field
  ├── models/user.go
  ├── migrations/001.sql
  └── repository/user.go

Batch 2: Expose age in user API
  ├── api/user_dto.go
  └── api/user_handler.go

Batch 3: Add age validation
  └── tests/user_test.go
```

每个 Batch 都有明确的意图说明和独立的 Diff。

开发者可以逐个 Review 这些 Batch。所有 Batch 组合起来，必须完整、准确地还原 Agent 最终产生的代码修改。

---

## 2. 核心设计原则

### 2.1 Intent-oriented，而不是 File-oriented

Review 的基本单位不再是文件，而是具有完整逻辑意义的代码变更。

一个 Batch 可以包含：

- 多个文件
- 同一文件中的多个代码片段
- 不同目录下的代码修改
- 不同软件层次的修改

### 2.2 Capture first, organize later

代码修改的原始记录与逻辑分组是两个独立过程。

原始记录负责忠实记录实际发生的修改。

逻辑分组负责理解这些修改之间的关系。

不能为了获得更好的分组而牺牲原始记录的真实性。

### 2.3 不干扰 Agent 正常工作

工具应该尽可能以透明的方式集成到现有 Coding Agent 工作流中。

不应该要求 Agent 为了 Code Review 而改变原有的开发习惯。

尤其应该避免：

- 每次修改文件都要求 Agent 额外生成结构化说明。
- 频繁打断 Agent 的正常工作。
- 大量增加 Token 消耗。
- 强制 Agent 按照预先定义的 Batch 顺序开发。

### 2.4 最终修改完整性

所有 Batch 的修改必须完整覆盖最终 Diff。

不能遗漏修改，也不能重复计算修改。

### 2.5 Intent 可追溯

每个 Batch 应该能够追溯到产生它的原始 Change Events。

Agent 后续整理的意图说明不能覆盖原始记录。

---

## 3. 核心概念

### 3.1 Raw Change Event

Raw Change Event 是 Agent 执行一次可能影响代码文件的操作后，实际产生的文件变化记录。

例如：

```text
Event #001

Tool: Edit
File: models/user.go

Before:
  type User struct {
      ID int
  }

After:
  type User struct {
      ID  int
      Age int
  }

Agent Tool Description:
  Add age field to User model.
```

Event 记录应该尽可能包含：

| 字段 | 说明 |
|---|---|
| Event ID | 唯一标识 |
| Timestamp | 发生时间 |
| Session ID | Agent Session |
| Tool Name | 工具名称 |
| Tool Call ID | 原始工具调用标识 |
| File Changes | 实际发生的文件变化 |
| Before / After | 修改前后的内容或快照引用 |
| Raw Diff | 本次操作产生的 Diff |
| Tool Description | Agent 明确提供的工具调用说明 |
| Context Reference | 相关任务、Prompt 或上下文引用 |

Event 的内容应以实际文件变化为准，而不是直接相信 Agent 对修改结果的描述。

需要注意：

- 一次工具调用可能修改多个文件。
- 一次工具调用可能没有产生任何文件变化。
- 多次工具调用可能反复修改同一个文件。
- 某些工具调用可能修改文件后又恢复原状。

因此，Raw Change Event 记录的是历史过程，不代表每个 Event 最终都会对应一部分 Final Diff。

### 3.2 Intent Group / Batch

Intent Group 是由一个或多个 Raw Change Events 组织而成的逻辑变更单元。

它代表一次开发任务中的某个具体意图。

例如：

```text
Batch 1: Add user age field

Intent:
  Add age field to the User entity and
  persist it in the database.

Source Events:
  - Event #001
  - Event #002
  - Event #004

Files:
  - models/user.go
  - migrations/001.sql
  - repository/user.go
```

一个 Batch 应该包含：

- Batch ID
- Intent Title
- Intent Description
- Source Events
- Related Files
- Batch Diff
- Dependencies
- Intent Provenance
- Review Status

Intent Provenance 用于说明意图描述的来源：

1. Explicit：Agent 在原始工具调用中明确提供。
2. Context-derived：可以从原始任务和上下文中找到依据。
3. Inferred：根据实际修改事后推断。

不应该把推断出来的原因伪装成 Agent 当时明确表达过的原始动机。

### 3.3 Final Diff

Final Diff 指 Agent 开始任务之前的基准版本与任务完成之后的最终工作区之间的完整代码差异。

它是整个 Review 的最终事实依据。

所有 Batch Diff 都必须能够还原这个 Final Diff。

### 3.4 Review Projection

Review Projection 是将 Raw Change Events、Intent Groups 和 Final Diff 组织成可供开发者审查的视图。

它负责：

- 展示逻辑 Batch。
- 展示 Batch 的 Diff。
- 展示 Batch 对应的原始修改事件。
- 展示 Batch 之间的依赖关系。
- 展示 Review 状态。
- 检查 Batch 与 Final Diff 的一致性。

---

## 4. 整体工作流程

核心流程分为三个阶段：

```text
          Agent Coding
               |
               v
    +-----------------------+
    | 1. Raw Change Capture |
    |                       |
    | Capture tool calls    |
    | Capture file changes  |
    | Store raw diffs       |
    +-----------------------+
               |
               v
    +-----------------------+
    | 2. Intent Organization|
    |                       |
    | Analyze raw events    |
    | Group related changes |
    | Generate explanations |
    | Identify dependencies |
    +-----------------------+
               |
               v
    +-----------------------+
    | 3. Review Projection  |
    |                       |
    | Generate batch diffs  |
    | Verify completeness   |
    | Present review UI     |
    +-----------------------+
               |
               v
         Developer Review
```

### 4.1 阶段一：Raw Change Capture

**目标：静默记录 Agent 的实际代码修改过程。**

通过 Agent Hooks 捕获可能导致文件变化的工具调用，例如：

- Write
- Edit
- Bash
- 其他具有文件修改能力的工具

在工具执行前后采集工作区状态，计算实际产生的 Diff。

这里需要区分工具调用记录与实际文件变化：

- Tool Call 是一次操作行为。
- Raw Change Event 是这次操作造成的实际文件变化。

两者可以关联，但不能假设每次工具调用都必然产生文件变化。

采集过程应尽可能不影响 Agent 原本的工作流。

#### 采集原则

1. 不要求 Agent 额外生成 Review 专用说明。
2. 优先使用 Agent 原生提供的工具调用描述。
3. 以实际文件变化为准。
4. 保留时间顺序。
5. 保留原始事件，不允许后续整理覆盖。
6. 不依赖某个特定 Agent 的内部实现。

对于 Bash 等可能影响多个文件的操作，需要关注实际文件系统变化，而不仅仅是工具参数中声明的目标文件。

### 4.2 阶段二：Intent Organization

**目标：将零散的 Raw Change Events 整理成具有逻辑意义的 Batch。**

这一阶段允许 Agent 参与。

它可以读取：

- Raw Change Events
- Agent 原始工具调用描述
- 原始开发任务
- 相关 Prompt 和上下文
- 实际代码修改内容

然后组织出逻辑分组。

例如：

```text
Raw Events:

E01: Edit models/user.go
E02: Edit migrations/001.sql
E03: Edit api/user_dto.go
E04: Edit repository/user.go
E05: Edit tests/user_test.go
E06: Edit api/user_handler.go
```

组织结果：

```text
Batch 1: Add user age field
  Events: E01, E02, E04

Batch 2: Expose age in API
  Events: E03, E06

Batch 3: Add age validation
  Events: E05
```

Agent 需要提供：

- 每个 Batch 的意图说明。
- 包含哪些 Raw Events。
- 为什么这些修改属于同一个意图。
- 不同 Batch 之间的依赖关系。
- 无法合理归类的修改。
- 意图说明的来源及可信度。

#### 重要限制

Intent Organization 不允许修改原始代码和 Raw Change Events。

Agent 可以重新组织分组、修改说明，但不能伪造历史记录。

如果无法确定某个修改的原始动机，应该明确标记为 Inferred，而不是编造一个看似合理的历史原因。

#### 触发时机

MVP 可以优先在 Agent 完成任务时执行一次完整整理。

未来可以考虑：

- Agent 完成明显的子任务时。
- Agent 即将进行上下文压缩时。
- 开发者主动请求 Review 时。
- 开发者手动触发重新组织时。

不建议在每次 Tool Call 后都让 Agent 进行整理，以免频繁打断正常开发。

### 4.3 阶段三：Review Projection

**目标：将逻辑 Batch 转化为可以独立审查的 Diff。**

每个 Batch 应该展示：

- Intent Title
- Intent Description
- 涉及的文件
- 代码 Diff
- 原始 Change Events
- 依赖的其他 Batch
- Review Status

Review 的主要视角是 Batch，而不是文件。

同时，开发者可以在需要时查看原始文件 Diff 和 Raw Event 历史。

---

## 5. Diff 完整性与一致性

这是整个系统最重要的正确性要求之一。

设：

- `B` 为 Agent 开始任务之前的基准代码状态。
- `F` 为 Agent 任务完成后的最终代码状态。
- `D` 为 B 到 F 的 Final Diff。
- `D_i` 为第 i 个 Batch 对应的 Diff。

要求所有 Batch 合并后必须还原最终代码状态：

```text
Apply(B, D_1, D_2, ..., D_n) == F
```

如果各 Batch 可以严格独立归属最终 Diff 的修改，还应该满足：

```text
Union(D_1, D_2, ..., D_n) == D
```

每个最终修改只能归属一个 Batch，不能遗漏，也不能重复。

### 为什么不能直接累加 Raw Event Diff？

例如：

```text
Event 1:
  timeout: 10 -> 20

Event 2:
  timeout: 20 -> 30
```

Final Diff 实际上只有：

```diff
- timeout = 10
+ timeout = 30
```

而不是两个独立的 Diff。

另一个例子：

```text
Event 1:
  新增函数 A

Event 2:
  删除函数 A
```

最终代码中根本不存在函数 A，因此 Final Diff 中也不会出现它。

所以必须明确区分：

- Raw Event Diff：记录每次工具操作的实际变化。
- Final Diff：记录最终代码状态相对基准版本的变化。
- Batch Diff：从最终修改中分配给某个逻辑意图的变化。

Raw Event Diff 负责历史追溯，Batch Diff 负责 Review。

### Batch 依赖关系

某些 Batch 不是完全独立的。

例如：

```text
Batch 1: Database Schema
       |
       v
Batch 2: Repository
       |
       v
Batch 3: API
```

可以使用有序 Changeset Stack 表达依赖关系。

不应该为了让 Batch 看起来独立，而隐藏它们之间的依赖。

---

## 6. Review 交互

Review 的核心交互应该围绕 Batch 展开。

一个典型的 Review 页面可以包含：

```text
Task: Add user age support

Final Diff: +1284 / -326

Batches:
  [Approved] Batch 1: Database Schema
  [Pending]  Batch 2: API Contract
  [Pending]  Batch 3: Validation
```

选择一个 Batch 后，展示：

1. Intent Description
2. 关联文件
3. Batch Diff
4. Source Events
5. Batch Dependencies
6. Intent Provenance

### Review 操作

建议支持以下操作：

- Approve：接受该 Batch。
- Request Changes：要求 Agent 修改该 Batch。
- Split：将一个 Batch 拆分。
- Merge：合并多个 Batch。
- Reassign：重新分配部分修改。
- Ignore：标记某些修改不需要审查。

其中，Request Changes 是一个重要的交互方向。

例如：

```text
Batch 3: Add age validation

Developer Feedback:
  Age should allow null,
  but negative values should be rejected.
```

Agent 根据反馈修改代码。

修改完成后，原有 Batch 更新，并保留 Review 历史。

这样 Code Review 就不再只是开发完成之后的检查，而是可以成为 Agent Coding 工作流中的反馈环节。

---

## 7. MVP 范围

MVP 的重点是验证一个核心假设：

**通过在开发过程中采集 Raw Change Events，并在开发结束后进行 Intent Organization，能否产生比传统 Git Diff 更容易理解和审查的代码变更视图？**

### 7.1 MVP 必须具备

- 至少集成一种 Coding Agent。
- 通过 Hook 捕获工具调用。
- 记录实际文件变化及 Raw Diff。
- 保存 Agent 原始提供的工具调用说明。
- 支持一次任务的完整 Raw Change Event 记录。
- Agent 完成任务后自动组织 Intent Groups。
- 每个 Batch 提供意图说明。
- 每个 Batch 可以展示独立 Diff。
- 可以追溯 Batch 对应的 Raw Events。
- 验证所有 Batch 能够完整还原 Final Diff。
- 提供基本的 Batch Review UI。

### 7.2 MVP 暂不考虑

以下问题有价值，但不应该成为第一版的阻碍。

**Batch 粒度优化**

暂时不追求自动找到最完美的 Batch 粒度。

先验证逻辑分组本身是否能显著改善 Review 体验。

**用户自定义粒度**

未来可以让用户自由选择粒度，例如：

- 更粗粒度：按照完整功能组织。
- 中等粒度：按照功能性意图组织。
- 更细粒度：按照小型逻辑修改组织。

也可以基于同一批 Raw Change Events 生成不同粒度的 Intent Groups。

**复杂的 Agent 协作**

暂时不考虑多个 Agent 同时修改同一个任务的复杂情况。

**高级 Git 集成**

MVP 不要求 Batch 一定对应独立 Git Commit、Git Branch 或 Git Worktree。

首先把 Batch 定义为 Review 分组。

**自动化 Review 决策**

不要求工具自动判断代码是否正确。

核心价值是改善代码修改的组织和审查体验，而不是替代开发者做最终决定。

---

## 8. 非目标

这个项目不是：

- 一个通用 AI Code Review Bot。
- 一个单纯的 Git Diff Viewer。
- 一个 Agent Session Recorder。
- 一个 Multi-Agent Orchestrator。
- 一个 Spec-driven Development 平台。
- 一个新的 Git 替代品。

这些方向可能与项目产生集成关系，但不是核心定位。

---

## 9. 技术与产品上的开放问题

以下问题暂时不需要在产品设计阶段全部确定。

### 9.1 Agent 集成

- 不同 Agent 的 Hook 能力有什么差异？
- 是否需要统一的 Agent Adapter？
- 是否能通过 MCP 或其他协议提供通用接口？
- 如何处理 Agent 不支持完整 Hook 生命周期的情况？

### 9.2 文件变化捕获

- 如何捕获 Bash 产生的文件变化？
- 如何处理 Agent 工具调用之外的文件修改？
- 如何处理后台进程产生的变化？
- 如何处理文件重命名、删除和二进制文件？
- 如何避免采集过程中产生的文件变化被误认为 Agent 修改？

### 9.3 Raw Change Event

- 是否需要完整保存每次操作前后的文件快照？
- 是否只保存 Patch 和必要的上下文？
- 如何控制存储量？
- 如何处理敏感信息和 Secret？
- 如何在 Hook 异常或进程中断时保证记录完整性？

### 9.4 Intent Organization

- 应该向 Agent 提供多少原始上下文？
- 是否需要使用独立模型进行整理？
- 如何处理无法确定意图的修改？
- 如何避免 Agent 生成虚假的修改原因？
- 如何识别相互依赖的 Batch？

### 9.5 Diff 完整性

- 如何将 Final Diff 精确分配到不同 Batch？
- 同一个代码片段被多次修改时如何归属？
- 多个 Batch 修改相互依赖的代码时如何表示？
- 如何保证 Batch Stack 可以精确还原最终代码状态？

### 9.6 Review 体验

- Batch 是否应该支持嵌套？
- 是否应该提供文件视角和 Batch 视角的双向切换？
- 是否需要展示 Raw Event 时间线？
- 如何将 Review Feedback 准确反馈给 Agent？

---

## 10. 产品成功标准

MVP 不应该仅仅以能否成功捕获 Diff 来判断是否成功。

真正需要验证的是：

1. **理解效率：** 开发者能否更快理解一次 Agent 任务的完整修改？
2. **认知负担：** 跨文件修改是否更容易理解？
3. **Review 质量：** 是否更容易发现逻辑不一致、遗漏和不合理修改？
4. **组织质量：** 自动生成的 Batch 是否符合开发者的直觉？
5. **完整性：** 所有 Batch 是否可以准确还原 Final Diff？
6. **工作流影响：** 采集和整理是否明显干扰 Agent 正常工作？

最终可以通过真实 Agent Coding 任务进行对照实验：

- 使用传统 Git Diff 进行 Review。
- 使用 Intent-based Review 进行 Review。

比较两种方式的理解时间、Review 完成时间、问题发现情况以及开发者主观体验。

---

## 11. 项目核心定义

**Intent-based Code Review**

A code review tool that captures AI coding agents' actual changes during development, organizes them into intent-based change batches, and allows developers to review code by logical intent rather than file order.

核心理念：

> Capture what actually changed.  
> Understand why changes belong together.  
> Review code the way developers think.

</WritingBlock>

---

我建议你接下来先不要急着写代码，重点想清楚三个问题：

1. **Review UI 长什么样**：这是最值得先做交互原型验证的部分。
2. **Raw Change Event 的数据模型**：这是整个系统未来可扩展性的基础。
3. **Batch Diff 的精确还原机制**：这是产品正确性的底线。

尤其是第一点，我觉得可以先拿一个真实的 Agent Coding 任务，手动整理成 3～5 个 Batch，看看这种 Review 方式是否真的比传统 Diff 舒服。这样可以先验证产品本身，而不必过早陷入 Hook 和 Patch 算法的实现细节。

---

**User:**

你觉得这个项目应该叫什么。我希望是一个或两个单词。最好不超过 8 个字母。最好不是项目功能的直接翻译比如 intent-cr ，有创意一点。

---

**Assistant:**

我觉得这个项目的名字最好不要带 `AI`、`Code`、`Review`、`Diff` 这些词。

它真正有意思的地方是：**把分散在不同文件、不同位置的修改，编织成一个有逻辑、有意义的整体。**

所以我会从编织、组合、结构、叙事这些隐喻寻找名字。

## 我最推荐的 8 个

<box gap={3}>
  <row align="start" gap={3}>
    <box background="surface-secondary" radius="lg" size="52px" align="center" justify="center">
      <title size="xl">Loom</title>
    </box>
    <box flex="1" gap={1}>
      **1. Loom** <badge color="success">个人最推荐</badge>
      <text color="secondary" size="xs">4 letters · 编织机</text>
      将分散的修改像丝线一样编织成有意义的整体。隐喻非常贴切，而且简短、好记。
    </box>
  </row>
  <divider color="subtle"/>
  <row align="start" gap={3}>
    <box background="surface-secondary" radius="lg" size="52px" align="center" justify="center">
      <title size="xl">Motif</title>
    </box>
    <box flex="1" gap={1}>
      **2. Motif**
      <text color="secondary" size="xs">5 letters · 主题、动机、模式</text>
      每组代码修改背后都有一个 Motif。这个名字强调的是修改背后的意图，而不是修改本身。
    </box>
  </row>
  <divider color="subtle"/>
  <row align="start" gap={3}>
    <box background="surface-secondary" radius="lg" size="52px" align="center" justify="center">
      <title size="xl">Quilt</title>
    </box>
    <box flex="1" gap={1}>
      **3. Quilt**
      <text color="secondary" size="xs">5 letters · 拼布、 quilt</text>
      不同的 Patch 最终拼成一个完整作品。很符合多个逻辑变更组成最终 Diff 的概念。
    </box>
  </row>
  <divider color="subtle"/>
  <row align="start" gap={3}>
    <box background="surface-secondary" radius="lg" size="52px" align="center" justify="center">
      <title size="xl">Weave</title>
    </box>
    <box flex="1" gap={1}>
      **4. Weave**
      <text color="secondary" size="xs">5 letters · 编织</text>
      和 Loom 类似，但更强调编织这个动作。可以理解为把零散的代码变化组织起来。
    </box>
  </row>
  <divider color="subtle"/>
  <row align="start" gap={3}>
    <box background="surface-secondary" radius="lg" size="52px" align="center" justify="center">
      <title size="xl">Tessera</title>
    </box>
    <box flex="1" gap={1}>
      **5. Tessera**
      <text color="secondary" size="xs">7 letters · 马赛克中的一块拼片</text>
      每个 Batch 是一块 Tessera，所有拼片组合起来形成完整的作品。比较有设计感和辨识度。
    </box>
  </row>
  <divider color="subtle"/>
  <row align="start" gap={3}>
    <box background="surface-secondary" radius="lg" size="52px" align="center" justify="center">
      <title size="xl">Stitch</title>
    </box>
    <box flex="1" gap={1}>
      **6. Stitch**
      <text color="secondary" size="xs">6 letters · 缝合、连接</text>
      把散落在不同文件的修改连接起来。直观，但不会直接暴露产品功能。
    </box>
  </row>
  <divider color="subtle"/>
  <row align="start" gap={3}>
    <box background="surface-secondary" radius="lg" size="52px" align="center" justify="center">
      <title size="xl">Skein</title>
    </box>
    <box flex="1" gap={1}>
      **7. Skein**
      <text color="secondary" size="xs">5 letters · 一束缠绕整理的线</text>
      一个很有意思的隐喻：原本零散、缠绕的线索，经过整理成为有序的整体。比较小众。
    </box>
  </row>
  <divider color="subtle"/>
  <row align="start" gap={3}>
    <box background="surface-secondary" radius="lg" size="52px" align="center" justify="center">
      <title size="xl">Trellis</title>
    </box>
    <box flex="1" gap={1}>
      **8. Trellis**
      <text color="secondary" size="xs">7 letters · 格架、支架</text>
      为分散的代码变化提供一个结构，让它们有序生长。更偏工程工具的气质。
    </box>
  </row>
</box>

## 我的排名

| 名字 | 品牌感 | 与产品理念的契合度 |
|---|---|---|
| Loom | 很强 | 很高 |
| Motif | 很强 | 很高 |
| Tessera | 很强 | 高 |
| Quilt | 强 | 很高 |
| Weave | 强 | 高 |
| Stitch | 中高 | 高 |
| Skein | 很强 | 高 |
| Trellis | 强 | 中高 |

## 我个人的最终选择

<box border radius="xl" padding={3} gap={2}>
  <title size="2xl">Loom</title>
  <title size="lg" color="secondary" weight="medium">Make changes make sense.</title>
  一个将 Agent 产生的零散代码修改，编织成清晰、可理解、可审查的逻辑变更的工具。
  <divider color="subtle"/>
  <title size="2xl">Motif</title>
  <title size="lg" color="secondary" weight="medium">Every change has a reason.</title>
  一个以修改意图为中心，重新组织代码审查体验的工具。
</box>

我其实觉得 **Loom 和 Motif 是两个不同气质的好名字**：

- Loom 更像一个基础设施或开发者工具，可以随着未来功能扩展而成长。
- Motif 更像一个有鲜明理念的产品，突出理解代码修改背后的意图。

如果让我现在创建 GitHub Repository，我会优先选择 `loom`。

不过这些名字还没有做 GitHub 仓库、域名和商标可用性核查。

---

*This file was created with GPTAha Free. [Upgrade to remove this notice →](https://www.gptaha.com/pricing?utm_source=export_watermark)*
