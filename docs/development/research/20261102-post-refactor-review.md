# main 重构后实现评审

基线：2026-11-02 实际 fetch origin/main，切换 main 并确认与 origin/main 同为 `b2f7b56`。评审不修改实现。

## 契约与结论

以 capture/organization/web 中已确认决定、用户已授权的 review-refactor 方案为准；未确认提案不整体作为契约。实现记录只用于导航，不把其中历史测试结果当作本次验证。

核心行为基本对齐：commit 范围与分支跟踪分离；刷新构建成功才发布；读取不隐式刷新；分支缺失保留已发布范围；输入签名复用；稳定身份保留决定；批量决定事务提交；评论/回复版本检查；最终闭合与历史归属分开表达。原评审指出的列表依赖分支解析、读取端点重复刷新两项问题已经解决。

本次重点读取 capture、change、gitrepo、review、store、web 的主要路径与相关回归测试源码，并核对采集/整理/Web 的已确认条款。不是全部场景的运行时验收。

## 尚未完成的架构目标

1. **内容解析和存储仍有运行时互相依赖。** gitrepo.Open 调用 Store.SetBaseReader，Tree 调用 Store.RegisterGitBlob；Store.baseState/Blob 回调 Git。change 虽已独立，review/gitrepo 仍广泛使用 store 类型。preparedIO 确实把 Git/内容读取移出写事务，但经 context.WithValue 隐式传给事务函数，未准备时运行期报错。是有效的增量修复，不能等同于重构方案第 6 节的职责拆分已全部完成。从零实现更适合显式的 PreparedObservation/PreparedProjection，应用服务依赖独立 SnapshotResolver/ContentResolver，Store 接收待发布数据并校验版本，不注册 Git 回调。

2. **稀疏持久化尚未延伸到投影内存表示。** review.build 为每项 Clone 完整 Before/After；每个事件重新恢复两个 Snapshot，LoadReviewStates 又恢复全部项两端状态。prepareIO 遍历输入完整状态，并为每个不同 hash 读取内容验证。主要状态元数据成本随文件数乘审阅项数增长；缓存减少外部 IO，却没有消除全状态遍历/复制。这与方案第 7 节共享不可变基准的方向还有距离。建议基准加稀疏覆盖/状态引用，组合选择只物化必要端点；内容验证仍应保留，围绕引用与变动内容设计，不能直接删掉校验。源码可以确定成本形态，本次没有测量实际延迟或内存。

3. **Git 内容来源登记无容量限制。** store.RegisterGitBlob 将 hash → 来源 key → 读取闭包保存在 gitBlobs；树缓存和 gitContent 的清空不会淘汰该表。长时间打开多个提交会累计不同 blob 的登记与闭包。32 MiB 只限制缓存字节，不限制来源索引；不能笼统说所有缓存均有界。应让 Git 内容引用显式携带仓库/OID，或设计可重新解析的有界来源索引，避免简单淘汰后破坏旧快照解析。

4. **失效粒度仍偏粗。** 输入签名依赖全局 schema Generation；sourceUpdate 即使幂等回执或无效果观察返回成功也会递增。其他项目采集同样触发本项目投影签名失效。实现记录已披露全局 Generation 边界，属于接受的保守实现，不是产品语义错误。后续按仓库及真实来源变化细化即可，无需引入复杂依赖图。

## 从零实现的判断

保留事件、batch、CR 投影三种生命周期，保留内容寻址和 Git 基准，也保留准确重放失败后明确未知补齐。这些复杂度来自产品语义。无需引入通用事件溯源框架或每表一个仓储。

优先改变依赖方向和数据表示：应用服务编排解析/准备/计算/发布；领域计算接收显式不可变输入；存储只负责记录、约束及版本发布；快照共享基准，使用稀疏覆盖。现状已经适合继续迭代，不建议推倒重写；优先处理显式准备接口和状态复制，再按测量结果优化索引及失效范围。

## 本次验证

- git fetch 成功；main 与 origin/main 一致，工作区原本干净。
- node --check pkg/web/assets/app.js、git diff --check 通过。
- go test ./... 无法执行：/usr/bin/go 返回 `Go: Unknown option: test`；常见工具链位置及实施记录中的 /tmp/go/bin/go 均不存在。本次未安装新工具链，未声称 Go 测试通过。
- 未执行真实 Codex 模型会话、浏览器及大仓性能测试；没有将旧实施记录中的验证复述为本次证据。
