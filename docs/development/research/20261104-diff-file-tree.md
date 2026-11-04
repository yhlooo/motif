# Diff 文件树的交互参考

参考 GitHub Primer 的 [TreeView 指南](https://primer.style/product/components/tree-view/guidelines/)，文件层级适合以树展示；节点名称默认单行截断，无独立激活行为的目录可以整行展开或折叠。相关原文见 [参考摘录](../reference/github/primer-tree-view.md)。

采用类似的紧凑行、层级缩进、展开箭头、目录图标、单行文件名及行末增删计数。不引入 Primer 依赖，也不复制其组件代码。目录使用原生 details/summary，文件使用按钮；以嵌套列表和导航区表达层级，不声明需要额外方向键行为的 ARIA tree 角色。目录可用原生键盘行为折叠，文件按钮可用键盘定位 diff。

行数直接统计当前 diff 的 hunk 中增加和删除的行，不累计各批次原始差异，以免合并选择后相互抵消的修改重复计数。diff 头部和文件标签不计入；hunk 内以多个加减号开头的真实代码仍应计数。二进制文件及采集范围外文件无法提供文本行数，在计数旁加星号并说明，不伪造文本行数。
