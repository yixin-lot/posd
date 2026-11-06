# packaging

Windows 分发相关资产。**图标不是手绘的图片文件，而是脚本生成的**——改品牌标只改一处。

## 五处图标都从同一个源生成

| 产物 | 消费点 |
|---|---|
| `packaging/pos.ico` | Inno Setup `SetupIconFile`（安装包自身、向导窗口、控制面板卸载项） |
| `cmd/posd/rsrc_windows_amd64.syso` | `go build` 自动链入，成为 `posd.exe` 的资源（文件图标、任务栏、桌面/开始菜单快捷方式） |
| `web/public/favicon.ico` | 收银台浏览器标签页（随 Vite 进 `dist`，再 embed 进 exe） |
| `web/public/apple-touch-icon.png` | 手机端「添加到主屏幕」 |
| `packaging/icon-master-1024.png` | 主图，文档/商店素材复用 |

快捷方式不单独存图标：`pos.iss` 的 `[Icons]` 用 `IconFilename: "{app}\posd.exe"`，直接取 exe 里的资源，
所以 exe 一旦带上图标，开始菜单/桌面/卸载项全部同步。

## 生成

```powershell
python scripts\pack\make-app-icon.py            # 全部
python scripts\pack\make-app-icon.py --skip-syso  # 只要 ico 与 png
```

`scripts\pack\package.ps1` 在 Go 构建前会自动跑这一步，正常打包不需要手动执行。

依赖：Python 3 + Pillow；Chrome 仅用于把 SVG 渲染成 1024 主图，缺失时回退到 Pillow 8 倍超采样手绘同一几何。

## 品牌依据

`design/qukeduo-vi/PosdVI视觉识别手册.html`（VI 2.0）图形标：96 网格、`#0D9488` 圆角底、
`#0F172A` 微笑弧、`#F59E0B` 双点。应用图标是「满版」变体——去掉 UI 场景中的 4px 内缩、
放大主体，保证 16px 任务栏尺寸下底色与图形都还立得住。

## 为什么 .syso 要入库

`rsrc_windows_amd64.syso` 是生成物，但入库，原因有两点：

1. 没装 Pillow 的机器（CI、队友全新 clone）上 `package.ps1` 生成不了，`go build` 会产出**没有图标**的 exe；
2. 图标一旦嵌错，用户看到的是 Windows 默认白纸图标，很难在发布后被察觉。

脚本可覆盖它；入库的那份是「保底正确版本」。

## 踩过的坑（改这里的实现前先看）

Go 链接器 `addpersrc()`（`src/cmd/link/internal/ld/pe.go`）**只对带 COFF 重定位的 `.rsrc` 节**做 RVA 修正：
它遍历重定位，把每个 `DataEntry.OffsetToData` 改写成 `节RVA + addend`。

因此手工拼 `.rsrc` 时，光有资源树不够——必须同时给：

- 一个 `.rsrc` 节符号（COFF，`StorageClass=3`、`SectionNumber=1`）；
- 每个 DataEntry 的一条 `ADDR32NB` 重定位（amd64 类型 `0x0003`，`VirtualAddress` = 该字段在节内的偏移）；
- 符号表之后的空字符串表（4 字节长度字段），否则链接器报 `fail to read string table length: EOF`。

漏了重定位，图标资源树结构看起来完全正常（类型、条数、字节数都对），但指向的数据是垃圾，
exe 依然显示默认图标——这是最容易误判成「图标文件坏了」的一处。

## 快捷键（维护）

改完图标资产后不需要改任何脚本，但需要重新打包，并且**已安装的旧版本要重装**：
快捷方式记录的是指向 exe 的图标索引，explorer 还会缓存旧图标。
