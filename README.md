# task181-fontproof — 数字字体回退覆盖证明工作台

字体工程师在此审查一组字体回退规则是否完整覆盖目标语言集合，并找出会把组合字符、变体选择符或数字系统错误拆分的最短回退链。系统按字素簇切分输入文本，逐簇解析所需字形与特性，生成字体选择链与缺失证明；工程师可调整规则并比较覆盖结果，发布冻结配置供渲染服务使用。

## 标准命令（必须全部成功）

```bash
export GO_BIN=$(command -v go)   # go1.26.3, GOTOOLCHAIN=local
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN test  ./...
$GO_BIN run ./cmd/fontproof --smoke-test
```

`--smoke-test`：不启动长驻服务；真实执行「登记字体 → 扫描 → 创建/验证/发布规则 → 创建样本 → 字素簇覆盖分析 → 校验拆分风险链与最短缺失链 → 生成/复核/发布报告 → 发布配置 → **关闭并重新打开数据库**验证持久化与重启恢复 → 替换字体摘要后新报告通过而旧配置快照不变」，成功以 0 退出并打印 `fontproof smoke test OK`。

## 启动服务

```bash
$GO_BIN run ./cmd/fontproof --addr :8080 --db /tmp/fontproof.db
# 浏览器打开 http://localhost:8080/ 查看工作台入口；全部数据经 /api JSON 接口读写
```

## API（JSON，/api 前缀，40 个）

字体资产：`POST /api/fonts`（登记，指纹幂等）、`GET /api/fonts`、`GET /api/fonts/{id}`、`POST /api/fonts/import`、`POST /api/fonts/{id}/scan`、`POST /api/fonts/{id}/disable`、`POST /api/fonts/{id}/enable`、`GET /api/fonts/{id}/ranges`

回退规则：`POST /api/rules`、`GET /api/rules`、`GET /api/rules/{id}`、`PUT /api/rules/{id}`（乐观锁 `expected_version`）、`POST /api/rules/{id}/validate`、`POST /api/rules/{id}/publish`、`POST /api/rules/{id}/supersede`、`POST /api/rules/{id}/fonts`、`DELETE /api/rules/{id}/fonts/{fontId}`、`POST /api/rules/reorder`（`expect_checksum` 冲突检测）、`GET /api/rules/{id}/cycle-check`

样本与分析：`POST /api/samples`、`GET /api/samples`、`GET /api/samples/{id}`、`POST /api/samples/{id}/analyze`、`GET /api/analyses/{id}`、`GET /api/analyses/{id}/chains`、`GET /api/analyses/{id}/gaps`（最短缺失链）、`GET /api/analyses/{id}/risks`（拆分风险）

报告与配置：`POST /api/reports`、`GET /api/reports`、`GET /api/reports/{id}`、`POST /api/reports/{id}/review`、`POST /api/reports/{id}/publish`、`POST /api/configs`、`GET /api/configs`、`GET /api/configs/{id}`、`GET /api/configs/compare?base={a}&target={b}`、`GET /api/configs/{id}/versions`

自检：`GET /api/health`、`GET /api/selfcheck`、`GET /api/audit`

## 业务不变量

- 字体资产：`pending_scan → available / conflict → disabled`；可重新启用。
- 覆盖规则：`draft → verifiable / gapped → published → superseded`；已发布规则不可直接编辑。
- 字素簇状态：`covered / fallback / risk / missing`；组合标记、变体选择符、ZWJ 序列与数字序列被拆分到多字体时标记拆分风险。
- 覆盖报告：`generating → pending_review → passed / gapped → published`。
- 已发布配置快照不可改写，仅可被新配置替代（supersede）；版本比较输出规则增删、重排与字体变化。
- 两个工程师重排同一规则集时，`expect_checksum` 不一致返回 409。
- 重复导入相同字体指纹复用扫描结果（幂等）。
- 重启后未完成分析（running）恢复为 resumed 并断点续传。

## 持久化

SQLite（纯 Go 驱动 `modernc.org/sqlite`，CGO 无关）：`fonts`、`font_ranges`、`font_features`、`font_scripts`、`fallback_rules`、`rule_fonts`、`sample_sets`、`analyses`、`graphemes`、`reports`、`published_configs`、`config_versions`、`audit_events`。
