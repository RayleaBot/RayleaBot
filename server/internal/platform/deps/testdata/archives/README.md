# XZ archive fixtures

这些固定样本由测试中的 TAR 条目使用 LZMA2、CRC64 编码，验证不依赖系统 `tar`、`xz` 或网络。

十六进制文件名是 Go 测试中条目描述的 SHA-256；测试按相同描述选择样本，并复制到临时目录后执行损坏实验。覆盖路径穿越、绝对路径、重复条目、逃逸链接、正常二进制、取消及校验尾部。`dictionary-limit.tar.xz` 的 LZMA2 字典属性声明 96 MiB，block-header CRC 已重算，必须在分配超限字典前拒绝。
