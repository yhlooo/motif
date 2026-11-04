# 审查侧栏按 commit 分节

## 用户决定与展示

用户要求左侧分批列表体现 commit 边界，每节显示 commit hash 和标题，下方容纳该提交的一个或多个分批。

按当前有序审阅项中相邻的 `commit_oid` 划分节，节头以小字显示 12 位 hash 和完整 subject，并提供完整 hash 的悬停提示。节间使用分隔线及留白。标题通过 textContent 写入并允许换行。

不排序、不合并审阅项，不跨不相邻的项重新分组；保持投影连续状态链及全局索引。Shift 连续选择可以跨 commit 节，审批、评论和 diff 仍使用原审阅项身份。没有提交归属的兼容数据明确标记未归属，不猜测。

## 数据与存储边界

- Git adapter 按完整 OID 批量读取 subject；领域模型 `Review.CommitTitles` 保存 OID 到标题的映射，原 `commits` 数组保持兼容。
- 应用构建阶段读取非 merge 提交及 head 的标题。head 用于最终差异补齐项，也覆盖只有 merge 独有差异的审查。
- 标题随不可变 projection 的 JSON payload 保存，无数据库 schema 变更。详情、历史投影和列表读取不新增 Git 访问，分支删除或刷新失败不影响已保存标题。
- 输入算法标识更新为 `immutable-state/2`，标题纳入视图摘要；旧审查在正常刷新时补齐元数据，通过原稳定身份保留批准及评论。旧 payload 若刷新失败，仍按 hash 分节并显示“提交标题不可用”；空 subject 显示“（无提交标题）”。

Git 格式调查见 [研究记录](../research/20261104-git-commit-subjects.md)。

## 验证

- Git adapter 真实仓库测试覆盖批量 subject 读取及空标题、中文、特殊字符。
- Web API 测试覆盖创建、更新、历史投影及删除分支后的标题读取。
- 审查测试覆盖 merge-only 最终补齐项的标题及旧 payload 刷新补齐后保留批准。
- 页面脚本测试覆盖同 commit 多批次、多个 commit、跨节 Shift 选择及旧 payload 降级显示。

验证结果：6 项页面脚本测试通过；review、web、domain、sqlite 相关 Go 包及新增 Git adapter 测试通过；旧 payload 补齐专项测试通过。独立无头 Chrome 在 1280px 和 390px 下用两个 commit、三个批次及超长中文标题检查分节、文字转义、跨节 Shift 选择，均无横向溢出或页面错误。
