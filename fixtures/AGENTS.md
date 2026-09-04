# Fixtures Agent Guide

先遵守根 `AGENTS.md`，分类、命名和包装结构按 `fixtures/README.md` 与对应校验器维护。

## Fixtures

- fixture 是从契约派生的可校验回归依据，不创造字段、状态或接口语义。`expect.notes` 只解释已有契约。
- 契约变化影响 fixture 时，同轮更新相关样例；根据实际风险提供正常、失败或边界 case。
- 样例可与契约先后编辑，合并前引用、README 和 CI 枚举须一致且必要验证通过。

## Sensitive Data

- 凭据使用明确假值；模拟鉴权失败时结合正式错误码，不构造接近真实平台凭据的字符串。
- 覆盖敏感字段的配置读取 fixture 必须断言输出已脱敏，不能把明文返回固定为正确行为。
