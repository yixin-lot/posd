# E2E 脚本

用无头 Chrome 跑真实页面，避免"接口通了但界面是坏的"。

## 前置

```powershell
node scripts/package.mjs        # 或至少 go build -o dist/posd.exe ./cmd/posd
cd web; npm install; cd ..
```

## 跑

```powershell
node scripts/e2e/smoke.mjs      # 登录→现金→挂单→取单→退款→报表
node scripts/e2e/import.mjs     # 商品 CSV 导入（UI 对话框）
node scripts/e2e/member.mjs     # 会员开卡→充值→余额支付
node scripts/e2e/monthly.mjs    # 月度报表（接口 + CSV；页面就绪后连 UI 一起验）
node scripts/e2e/audit.mjs      # 审计（改设置 → 留痕；页面就绪后连 UI 一起验）
node scripts/e2e/reprint.mjs    # 重打小票（会员折扣/积分/支付方式不丢行）
node scripts/e2e/d14.mjs        # 订单筛选 / 挂单时长 / 打印区块 / 375px
node scripts/e2e/stock.mjs      # 库存列 / 未跟踪 / 低库存提醒 / 报损
node scripts/e2e/bundle-options.mjs  # 套餐可选项（选一个 / 组件快照）
node scripts/e2e/purchase.mjs    # 采购入库（元→分 / 库存增加）
node scripts/e2e/stocktakes.mjs  # 盘点历史 / 只读详情
node scripts/e2e/mobile375.mjs   # 375×812 移动端截图与溢出采集（只测不改）

# 本地回归（CI 未接入）
node scripts/e2e/d13.mjs        # 并台/转台/到期横幅/档口校验（断言依赖 python 直写库，待重写）
node scripts/e2e/wpd12.mjs      # 月结/审计/会员/加菜
```

脚本自己起 posd（临时库）+ vite + Chrome，跑完自动清理；失败退出码非 0。

## 环境变量

| 变量 | 默认 |
|---|---|
| `E2E_CHROME` | `C:\Program Files\Google\Chrome\Application\chrome.exe` |
| `E2E_POSD` | `dist/posd.exe` |
| `E2E_ROOT` | 当前目录 |

## 新增场景

在 `scripts/e2e/` 下加一个 `xxx.mjs`，复用 `lib.mjs` 的 `launchStack/openPage/login/assert`。
每个断言都会打印 ✓/✗，失败即抛错。

## 已知问题

- `d13.mjs` 用 python 直写 SQLite 造"积分到期"数据，且多处断言依赖瞬时 toast 文案。
  改为纯 API 造数 + 状态断言后再接入 CI。
- 所有脚本启动前会强杀 8080/5173 的监听进程（Windows 上 `kill()` 常杀不掉，残留会串库）。
- `stock.mjs` 的接口步骤偶发 401（多实例串库时），当前只作为本地回归；
  库存弹窗交互暂未自动化，靠人工验证。

## 已知问题（2026-11-04 更新）

- **测试环境串库**已部分修复：启动前按端口 `taskkill /T`、启动后用 `/version` 的 `db` 路径校验连的是本次实例。
- **残留 token**：Chrome profile 复用时会带着上次的 token，导致全流程 401（表象像服务端问题）。
  `stock.mjs` 现在会先 `localStorage.clear()` 再登录；其他脚本尚未统一，后续应提升到 lib。
