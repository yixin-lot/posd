# 参与贡献

感谢你愿意改进 posd。提交前请先读这一页，能省掉一轮来回。

## 1. 先开 issue，再写代码

- Bug：附**复现步骤**、期望行为、实际行为、版本（`posd --version`）与系统；
- 功能：先描述场景（什么店、多少人、现在怎么解决的），不要直接贴方案；
- 安全漏洞：**不要开公开 issue**，按 [SECURITY.md](SECURITY.md) 私下报告。

## 2. 开发环境

```bash
# 后端（Go 1.26+）
go build ./... && go test ./...
go run ./cmd/posd --addr 127.0.0.1:8080 --db ./tmp/pos.db

# 前端（Node 20+）
cd web && npm install && npm run dev   # 5173，代理到 8080
npm test                               # 组件测试
```

## 3. 提交前自检（缺一不可）

```bash
go test ./... -count=1          # 必须全绿
cd web && npm test              # 必须全绿
node scripts/e2e/smoke.mjs      # 冒烟（需要 Chrome）
node scripts/ci/license-check.mjs
node scripts/ci/secret-scan.mjs
```

## 4. 代码约定

- **钱一律用分（int64）**，只在 HTTP/UI 边界转元；服务端必须重算金额，不信任前端；
- 新增字段要同时改**读路径**（之前踩过 `Get` 改了 `List` 没改的坑）；
- 迁移文件只增不改，命名 `V{n}__{描述}.sql`；
- 打印渲染改动必须补「字段守卫」测试（`internal/escpos/coverage_test.go` 的写法）。

## 5. 提交信息

```
feat(cashier): 支持挂单超过 30 分钟自动取消

- 只有非堂食挂单会被回收
- 设置项 hold_expire_minutes，0 表示关闭

验证：
$ go test ./internal/order/ -run TestAutoCancel -count=1
ok
```

## 6. CLA

提交 PR 即表示同意 [CLA.md](CLA.md)。企业贡献请由授权代表签署第 7 节。

## 7. 评审标准

| 会被拒的 | 会被要求的 |
|---|---|
| 未经讨论的大重构 | 补测试（尤其是钱和库存） |
| 引入 GPL/AGPL 依赖 | 说明许可证与体积影响 |
| 只改 UI 不改空态/错误态 | 截图或 E2E 证据 |
| "在我机器上能跑" | 贴真实命令输出 |

## 发布节奏

发布说明只写用户可见变化，规则见 `docs/oss-templates/RELEASE-NOTES-POLICY.md`；脚本会在发版前检查，命中内部词直接中止。


**不按每个小修单独发版**。默认攒批：

| 类型 | 条件 | 命令 |
|---|---|---|
| patch（修 bug、文案、样式） | 距上个 tag **≥ 3 个提交** | `node scripts/release/release.mjs --bump patch` |
| minor（新功能） | 不受门槛限制 | `node scripts/release/release.mjs --bump minor` |
| 热修 | 线上坏了，立即发 | 加 `--force`，并在提交信息写明原因 |

门槛由脚本强制：提交数不足会直接退出（退出码 2），不会误发。

发布脚本会做：测试 → 干净源码包 → 公开检查 → 单提交 + tag → 双平台推送 → 安装包 → Release 资产上传 → 官网数据，全程一条命令。

首次使用前设置令牌：

```bash
GH_TOKEN=... GT_TOKEN=... node scripts/release/release.mjs --bump patch
```
