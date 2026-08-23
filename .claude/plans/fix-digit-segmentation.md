# 修复：连续数字切分时不同数字系统被拼成同一数字簇

## 问题

`internal/grapheme/segmenter.go` 的数字序列归组循环（L75–L83）用 `isDigit`
（实现为 `unicode.IsNumber`）判断是否并入当前数字簇。`unicode.IsNumber`
匹配**任意脚本**的十进制数字（ASCII `0-9`、Arabic-Indic `٠-٩` U+0660-0669、
Extended Arabic-Indic/Persian `۰-۹` U+06F0-06F9、Devanagari `०-९`、Bengali、Thai…）。

因此一个混用数字系统的输入（如 `12٣٤`、或 `٠١۲۳`）会被拼成**同一个**数字簇，
字体证明引擎随后把整个码点集合当成一个单元、用一条字体选择链去覆盖——
即「字体证明把它们当成同一套脚本处理」。

包注释（L8）本就声明正确意图（“同一数字系统内的连续数字归组”），但实现用
`unicode.IsNumber` 未按系统区分，与注释不符。

## 修复范围

仅修复**切分**（“请修复切分结果”）：让数字归组按“数字系统”进行，不同系统拆成独立簇。
不改动 `script.go`、`engine.go`、`rule_service.go` 等——因为证明引擎是**码点驱动**
的（`ProveCluster` 用 `Covers`/`CoversAnyOf` 逐码点匹配字体范围，不读 `Script` 标签），
一旦不同系统成为独立簇，各自即被独立证明并路由到正确字体，症状随之消除。

## 改动

### 1. `internal/grapheme/segmenter.go`

**(a) 新增 `ndBlocks` 表 + `ndBlock(rune) (start rune, ok bool)`**

- `ndBlocks` 列出全部 Unicode `Nd`（十进制数字）块（共 64 个，已用探测脚本枚举确认）。
  每个连续块 = 一个数字系统。包含：ASCII `0030-0039`、Arabic-Indic `0660-0669`、
  Extended Arabic-Indic `06F0-06F9`、NKo `07C0-07C9`、Devanagari `0966-096F`、
  Bengali `09E6-09EF` … Thai `0E50-0E59` … Fullwidth `FF10-FF19` … 直至 `1FBF0-1FBF9`。
- `ndBlock(r)`：对 `r` 做二分查找，命中返回 `(块首, true)`；非 Nd 数字或未列出返回 `(0, false)`。

**(b) 重写数字归组循环（替换 L75–L83）**

当前以 `isDigit(cp)` 守卫、循环 `isDigit(runes[i+1])` 并入。改为：

```go
// 数字序列归组：仅同一数字系统内的连续数字合并为一个数字序列簇。
// 不同数字系统（ASCII / Arabic-Indic / Extended Arabic-Indic / Devanagari …）
// 在此切分为独立簇，避免被字体证明当作同一套脚本处理。
if kind == KindSimple {
    if blk, ok := ndBlock(cp); ok {
        for i+1 < len(runes) {
            next, ok2 := ndBlock(runes[i+1])
            if !ok2 || next != blk {
                break // 跨数字系统：结束当前簇
            }
            i++
        }
        if i > start {
            kind = KindNumeric
        }
    }
}
```

`isDigit`（`unicode.IsNumber`）函数不再被归组逻辑使用；将其删除以避免误导。
保留 `KindNumeric` 常量与 `splitReason` 的 case 6（数字序列拆分风险）不变。

### 2. `internal/grapheme/segmenter_test.go`

新增 `TestSegmenterNumericSystemsSeparate`，断言：

- ASCII + Arabic-Indic（`"12٣٤"`）→ 2 个簇：`{1,2}`(KindNumeric, Latn/Zyyy)
  与 `{٠,١}`…即 `{U+0663,U+0664}`(KindNumeric, Arab)。
- Arabic-Indic + Extended Arabic-Indic（`"٠١۰۱"`）→ 2 个簇
  （即使两者脚本都是 Arab，也因不同数字系统拆开）。

现有 `TestSegmenterNumericGrouping`（`"12345"` 全 ASCII）仍为 1 个簇、5 码点——通过。

## 不受影响的既有行为（已核对）

- `TestDetectScript` 中 `{1}→Zyyy`：不改 `scriptOf`，仍成立。
- `TestProveClusterNumericSplitRisk`：直接调 `ProveCluster`，不经切分器，不受影响。
- `smoke.go` / `service_test.go` / `engine_test.go` 样本只用单一 ASCII 数字 `12345`
  （全在同一块）→ 仍是 1 个簇，行为不变。
- 证明引擎码点驱动：独立簇各自独立证明，路由到正确字体，症状消除。

## 验证

```
go build ./...
go test ./internal/grapheme/... ./internal/proof/... ./internal/service/... ./internal/httpapi/...
go run ./cmd/fontproof --smoke-test
```
