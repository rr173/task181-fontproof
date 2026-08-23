# BENZHI_README.md — task181-fontproof 评测说明

## 业务

**数字字体回退覆盖证明工作台**：字体工程师登记字体资产（Unicode 覆盖范围、OpenType 特性标签、脚本），创建按优先级排序的回退规则并绑定字体；提交文本样本后系统按 UAX#29 子集切分字素簇（合并组合标记、变体选择符、ZWJ 序列与数字序列），逐簇生成字体选择链。组合字符/变体/数字系统被拆分到多字体时标记**拆分风险**；无字体覆盖的码点输出**最短缺失链**证明。覆盖报告经复核后发布，回退配置发布时冻结规则与字体快照，可版本比较；替换字体覆盖摘要后新规则生成新报告而旧配置不变。

## 标准命令（必须全部成功）

```bash
export GO_BIN=$(command -v go)   # go1.26.3, GOTOOLCHAIN=local
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local $GO_BIN test  ./...
$GO_BIN run ./cmd/fontproof --smoke-test
```

`--smoke-test` 契约：不启动长驻服务；真实执行「登记 3 字体并扫描 → 创建并发布 2 条规则 → 创建样本（含组合字符、变体、数字序列、缺字）→ 字素簇覆盖分析 → 校验拆分风险链与最短缺失链 → 生成/复核/发布报告 → 发布配置 → **关闭并重新打开数据库**验证持久化与重启恢复 → 新增 Devanagari 字体与规则后新报告通过、旧配置快照不变且被替代」，成功以 0 退出并打印 `fontproof smoke test OK`。

## 双架构 Docker 验证

使用固定评测脚本 `build_benzhi_docker.sh`（`IMAGE_NAME=${1:-my-project}`，`DOCKER_PLATFORM=${2:-linux/amd64}`）：

```bash
bash build_benzhi_docker.sh my-project linux/amd64
docker run --rm --platform linux/amd64 my-project --smoke-test   # 必须打印 fontproof smoke test OK

bash build_benzhi_docker.sh my-project linux/arm64
docker run --rm --platform linux/arm64 my-project --smoke-test   # 必须打印 fontproof smoke test OK
```

镜像入口 `ENTRYPOINT ["/fontproof"]`，默认 `CMD ["--smoke-test"]`；Dockerfile 与 benzhi.Dockerfile 同源（多阶段构建，运行阶段基于 alpine:3.20）。也可用 `docker run --rm my-project --addr=:8080 --db=/tmp/fontproof.db` 启动 API 与浏览器工作台。

## API（JSON，/api 前缀，40 个）

核心闭环接口：

1. `POST /api/fonts` `{"name":"...","family":"...","ranges":[{"start":32,"end":126}],"scripts":["Latn"]}` 登记字体（指纹幂等）
2. `POST /api/fonts/{id}/scan` 扫描评估（available/conflict）
3. `POST /api/rules` `{"name":"latin-fallback","required_scripts":["Latn"],"font_ids":[...]}` 创建规则
4. `POST /api/rules/{id}/validate` 验证（verifiable/gapped）
5. `POST /api/rules/{id}/publish` 发布（gapped 拒绝）
6. `POST /api/samples` `{"name":"s","content":"a\u0301 \u0915"}` 创建样本
7. `POST /api/samples/{id}/analyze` 字素簇覆盖分析
8. `GET /api/analyses/{id}/gaps` 最短缺失链证明
9. `GET /api/analyses/{id}/risks` 拆分风险字素簇
10. `POST /api/reports` `{"analysis_id":"..."}` 生成覆盖报告
11. `POST /api/reports/{id}/review` `{"approve":true}` 复核
12. `POST /api/configs` `{"name":"prod-v1"}` 发布冻结配置
13. `GET /api/configs/compare?base={a}&target={b}` 版本比较
14. `GET /api/selfcheck` 自检（store/grapheme/fallback/proof/release）

完整列表见 `README.md`。
