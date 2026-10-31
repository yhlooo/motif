1. 执行命令 `goi18n extract -format yaml -outdir pkg/i18n` 提取项目中需要翻译的消息结构体
2. 执行命令 `goi18n merge -format yaml -outdir pkg/i18n pkg/i18n/active.*.yaml pkg/i18n/translate.*.yaml` 将新增需要翻译的内容合入到 `pkg/i18n/translate.{lang}.yaml` 文件中
3. 将 `pkg/i18n/translate.{lang}.yaml` 中的每个消息从英文翻译为指定目标语言

   目标语言由文件名决定， `translate.{lang}.yaml` 中 `{lang}` 表示语言，比如 `translate.zh.yaml` 表示目标语言是中文。

   需要将消息结构体中以下字段值翻译为目标语言：（每个字段都是可选的，不存在的字段可忽略）
    - **zero** (string) 是 CLDR 复数形式 `zero` 的消息内容，用于复数值为 0 的情况；
    - **one** (string) 是 CLDR 复数形式 `one` 的消息内容，用于复数值为单数的情况；
    - **two** (string) 是 CLDR 复数形式 `two` 的消息内容，用于复数值为双数的情况；
    - **few** (string) 是 CLDR 复数形式 `few` 的消息内容，用于复数值为少量（如 3-4 ）的情况；
    - **many** (string) 是 CLDR 复数形式 `many` 的消息内容；用于复数值为大量或分数（如 10-20 ）的情况
    - **other** (string) 是 CLDR 复数形式 `other` 的消息内容，默认形式，通常是复数，也用于其他未覆盖情况；

   以下字段值不需要翻译，但是可能对于翻译有意义：
    - **description** (string) 描述信息，用于向译者提供与翻译相关的额外背景信息，以解释消息内容。
    - **hash** (string) 唯一标识此消息所翻译的原始消息的内容。（ **绝对不允许修改** ）

4. 再次命令 `goi18n merge -format yaml -outdir pkg/i18n pkg/i18n/active.*.yaml pkg/i18n/translate.*.yaml` 将 `pkg/i18n/translate.{lang}.yaml` 中翻译好的内容合入到 `pkg/i18n/active.{lang}.yaml` 文件中
