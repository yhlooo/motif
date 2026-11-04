# 审查页吸附导航

依据 [MDN position](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/position)，sticky 元素仍参与正常文档流，到达指定偏移后才吸附，受滚动祖先及包含块约束。原文片段见 [参考摘录](../reference/css/sticky-position.md)。

采用 CSS sticky 和显式 top，避免通过页面 scroll 事件切换 fixed 定位。左侧 grid 项设置 align-self:start，限制最大高度并让自身内容滚动，使其不会拉伸成与长 diff 同高的普通侧栏。右侧文件树保持在 diff 区域内吸附，移除平板样式对 sticky 的禁用。

窄屏的两组导航上下排列时，需要不同的吸附偏移和受限高度，避免互相覆盖、占满屏幕；跳转文件时也需要相应的 scroll-margin-top，让目标文件头避开吸附导航。
