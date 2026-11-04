# 行评论输入的定位和焦点

MDN 的 [focus 文档](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/focus)说明，默认聚焦会将目标滚入视图，preventScroll 可关闭这个行为。[getBoundingClientRect 文档](https://developer.mozilla.org/en-US/docs/Web/API/Element/getBoundingClientRect)说明其坐标相对视口，适合为 fixed 浮层提供锚点。原文见 [参考摘录](../reference/dom/line-comment-positioning.md)。

输入框挂载在 body，并以行号按钮的视口矩形定位，避免被 diff 的 overflow 容器裁切。聚焦输入框及返回行号时均使用 preventScroll。监听文档捕获阶段的 scroll，可以同时处理页面滚动和 diff 内部滚动；resize 及浮层尺寸变化时重新约束位置。锚点滚出代码可视区时隐藏浮层但保留草稿，滚回后恢复。

刷新评论时保留原 diff DOM，记录并恢复文档及各 diff 的滚动位置，避免整块 diff 被重新创建。定位与滚动行为需要真实浏览器验证，单纯的模拟 DOM 测试无法证明视口位置正确。
