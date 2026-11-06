# pos-web · 收银台（V0.1）

选品 → 现金结算 → 调 pos-service 落库并出票。

## 跑起来

```powershell
npm install
npm run dev   # http://127.0.0.1:5173
```

需要先起 `pos-service`（8080）。开发期 `/api` 由 Vite 代理到 8080（见 `vite.config.js`）。

## 测试

```powershell
npm install        # 首次装依赖（含 vitest / @vue/test-utils / jsdom，均为 devDependencies）
npm test           # = vitest run，headless 跑一遍，失败非 0 退出
```

单测放在 `src/**/__tests__/*.spec.js`（当前两个）：
- `components/__tests__/SettingsView.spec.js`：`GET /settings` 回填全部字段 + 保存提交的是回填值（防"保存把配置打回默认"）；
- `components/__tests__/CartPanel.spec.js`：加减删按钮 emit 的行数据、金额与应付（不抹零 / 抹分 / 抹角三分支）。

**mock 方式**：不碰后端，只 `vi.mock('../../api/index.js')` 替换前端 api 层（`getSettings` / `saveSettings` / `listStations` / `printJobs`），用 `mount` + `flushPromises` 断言 DOM。金额与抹零的纯计算抽在 `src/lib/money.js`（`cartTotal` / `roundPayable`），`App.vue` 用的就是它，测试与线上同一份实现。

端到端（需要能起 8080 + 5173，脚本自己起临时实例）：
```powershell
node scripts/e2e/smoke.mjs
node scripts/e2e/purchase.mjs
node scripts/e2e/stocktakes.mjs
```


## 边界（V0.1）

- 商品是内置样例（`src/data/products.js`），M1.5 接商品接口；
- 只有现金（ADR-004）；
- 无登录、无班次（M1 后续）；
- 渲染在 pos-service，浏览器不生成 ESC/POS（ADR-003）。
