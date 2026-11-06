#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""生成 Windows 应用图标资源（零外部依赖，除 Chrome 用于矢量渲染）。

产出：
  packaging/pos.ico                        —— Inno Setup 的 SetupIconFile / 快捷方式图标
  packaging/icon-master-1024.png           —— 主图（文档、商店素材可复用）
  cmd/posd/rsrc_windows_amd64.syso         —— go build 时自动链接的 Windows 资源（exe 图标）
  web/public/favicon.ico                   —— 收银台浏览器标签页图标（随前端内嵌进 exe）
  web/public/apple-touch-icon.png          —— 手机端「添加到主屏幕」的图标

品牌依据：design/qukeduo-vi 图形标 —— #0D9488 圆角底 + #0F172A 微笑弧 + #F59E0B 双点。
应用图标为「满版」变体（去掉了 UI 场景里的 4px 内缩），保证 16px 任务栏尺寸下底色完整。

用法：
  python scripts/pack/make-app-icon.py
  python scripts/pack/make-app-icon.py --skip-syso      # 只出 ico
"""

import argparse
import io
import os
import struct
import subprocess
import sys
import tempfile

from PIL import Image

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

CHROME_CANDIDATES = [
    r"C:\Program Files\Google\Chrome\Application\chrome.exe",
    r"C:\Program Files (x86)\Google\Chrome\Application\chrome.exe",
]

BRAND = "#0D9488"
INK = "#0F172A"
ACCENT = "#F59E0B"

# VI 图形标（96 网格）的满版应用版：外框 0..96，rx 21 ≈ 原 rx22 等比
SVG = """<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 96 96">
  <rect x="0" y="0" width="96" height="96" rx="21" fill="{brand}"/>
  <path d="M20 60c13-16 43-16 56 0" fill="none" stroke="{ink}" stroke-width="8" stroke-linecap="round"/>
  <circle cx="34" cy="40" r="7.5" fill="{accent}"/>
  <circle cx="62" cy="40" r="7.5" fill="{accent}"/>
</svg>
""".format(brand=BRAND, ink=INK, accent=ACCENT)

SIZES = [256, 128, 64, 48, 40, 32, 24, 20, 16]
MASTER = 1024


def render_with_chrome(master_px=MASTER):
    """用 Chrome headless 把 SVG 渲染成透明 PNG。返回 PIL.Image。"""
    chrome = next((p for p in CHROME_CANDIDATES if os.path.exists(p)), None)
    if not chrome:
        return None
    tmp = tempfile.mkdtemp(prefix="pos-icon-")
    html_path = os.path.join(tmp, "icon.html")
    png_path = os.path.join(tmp, "icon.png")
    with open(html_path, "w", encoding="utf-8") as f:
        f.write(
            "<!doctype html><meta charset='utf-8'>"
            "<style>html,body{margin:0;padding:0;background:transparent}"
            "svg{display:block}</style>" + SVG
        )
    profile = os.path.join(tmp, "profile")
    cmd = [
        chrome,
        "--headless=new",
        "--disable-gpu",
        "--hide-scrollbars",
        "--default-background-color=00000000",
        "--user-data-dir=" + profile,
        "--window-size=%d,%d" % (master_px, master_px),
        "--screenshot=" + png_path,
        "file:///" + html_path.replace("\\", "/"),
    ]
    subprocess.run(cmd, capture_output=True, timeout=120)
    if not os.path.exists(png_path):
        return None
    img = Image.open(png_path).convert("RGBA")
    if img.size != (master_px, master_px):
        img = img.resize((master_px, master_px), Image.LANCZOS)
    return img


def render_with_pillow(master_px=MASTER):
    """无 Chrome 时的回退：8x 超采样手绘（同样按 SVG 的几何关系）。"""
    k = 8
    n = master_px * k
    unit = n / 96.0
    img = Image.new("RGBA", (n, n), (0, 0, 0, 0))
    from PIL import ImageDraw

    d = ImageDraw.Draw(img)
    d.rounded_rectangle([0, 0, n - 1, n - 1], radius=21 * unit, fill=hex_rgb(BRAND))

    # 二次贝塞尔 M(20,60) c(33,44)(63,44)(76,60) 采样
    p0, p1, p2, p3 = (20, 60), (33, 44), (63, 44), (76, 60)
    pts = []
    steps = 120
    for i in range(steps + 1):
        t = i / steps
        mt = 1 - t
        x = mt**3 * p0[0] + 3 * mt**2 * t * p1[0] + 3 * mt * t**2 * p2[0] + t**3 * p3[0]
        y = mt**3 * p0[1] + 3 * mt**2 * t * p1[1] + 3 * mt * t**2 * p2[1] + t**3 * p3[1]
        pts.append((x * unit, y * unit))
    d.line(pts, fill=hex_rgb(INK), width=int(8 * unit), joint="curve")
    for c in (pts[0], pts[-1]):
        r = 4 * unit
        d.ellipse([c[0] - r, c[1] - r, c[0] + r, c[1] + r], fill=hex_rgb(INK))

    for cx in (34, 62):
        r = 7.5 * unit
        cy = 40
        d.ellipse([cx * unit - r, cy * unit - r, cx * unit + r, cy * unit + r], fill=hex_rgb(ACCENT))

    return img.resize((master_px, master_px), Image.LANCZOS)


def hex_rgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i : i + 2], 16) for i in (0, 2, 4)) + (255,)


def dib_entry(img):
    """ICO 内的 BMP(DIB) 条目：BITMAPINFOHEADER + BGRA 自下而上 + 空 AND 掩码。"""
    s = img.size[0]
    bgra = img.tobytes("raw", "BGRA")
    rows = [bgra[y * s * 4 : (y + 1) * s * 4] for y in range(s)]
    pixels = b"".join(reversed(rows))
    mask_row = ((s + 31) // 32) * 4
    mask = b"\x00" * (mask_row * s)
    header = struct.pack(
        "<IiiHHIIiiII", 40, s, s * 2, 1, 32, 0, len(pixels), 0, 0, 0, 0
    )
    return header + pixels + mask


def png_entry(img):
    buf = io.BytesIO()
    img.save(buf, format="PNG", optimize=True)
    return buf.getvalue()


def build_entries(master):
    """返回 [(size, blob, is_png)]，并生成各尺寸位图。"""
    out = []
    for s in SIZES:
        im = master if s == MASTER else master.resize((s, s), Image.LANCZOS)
        if s == 256:
            out.append((s, png_entry(im), True))
        else:
            out.append((s, dib_entry(im), False))
    return out


def build_ico(entries):
    head = struct.pack("<HHH", 0, 1, len(entries))
    table = b""
    blobs = b""
    offset = 6 + 16 * len(entries)
    for s, blob, _png in entries:
        w = 0 if s >= 256 else s
        table += struct.pack("<BBBBHHII", w, w, 0, 0, 1, 32, len(blob), offset + len(blobs))
        blobs += blob
    return head + table + blobs


def build_group(entries):
    """RT_GROUP_ICON 的 GRPICONDIR。"""
    out = struct.pack("<HHH", 0, 1, len(entries))
    for idx, (s, blob, _png) in enumerate(entries, start=1):
        w = 0 if s >= 256 else s
        out += struct.pack("<BBBBHHIH", w, w, 0, 0, 1, 32, len(blob), idx)
    return out


def build_rsrc(entries):
    """PE 资源目录：RT_ICON(3) 逐条 + RT_GROUP_ICON(14) 一条。

    布局（偏移均相对 .rsrc 本节起点）：
        root 表(2 项) → 类型3 表(N 项) → N 个类型3 子目录(语言项) →
        类型14 表(1 项) → 类型14 子目录 → 数据条目(N+1) → 数据块
    Go 链接器在输出 PE 时会把这里各 DataEntry.OffsetToData 加上节的 RVA。
    """
    n = len(entries)
    off_root = 0
    off_t3 = off_root + 16 + 2 * 8
    off_sub3 = off_t3 + 16 + n * 8
    off_t14 = off_sub3 + n * 24
    off_sub14 = off_t14 + 16 + 8
    off_de = off_sub14 + 24
    off_blob = off_de + (n + 1) * 16

    blobs = [e[1] for e in entries]
    blob_group = build_group(entries)
    icon_at = []
    cur = off_blob
    for b in blobs:
        icon_at.append(cur)
        cur += len(b)
    group_at = cur

    root = struct.pack("<IIHHHH", 0, 0, 0, 0, 0, 2)
    root += struct.pack("<II", 3, 0x80000000 | off_t3)
    root += struct.pack("<II", 14, 0x80000000 | off_t14)

    # 类型 3 表 + 每个图标的语言子目录（语言 0 = 中性）
    t3 = struct.pack("<IIHHHH", 0, 0, 0, 0, 0, n)
    sub3 = b""
    for i in range(n):
        sd = off_sub3 + i * 24
        t3 += struct.pack("<II", i + 1, 0x80000000 | sd)
        sub3 += struct.pack("<IIHHHH", 0, 0, 0, 0, 0, 1)
        sub3 += struct.pack("<II", 0, off_de + i * 16)

    # 类型 14 表 + 子目录
    t14 = struct.pack("<IIHHHH", 0, 0, 0, 0, 0, 1)
    t14 += struct.pack("<II", 1, 0x80000000 | off_sub14)
    t14 += struct.pack("<IIHHHH", 0, 0, 0, 0, 0, 1)
    t14 += struct.pack("<II", 0, off_de + n * 16)

    de = b""
    de_offsets = []
    for i, b in enumerate(blobs):
        de_offsets.append(off_de + i * 16)
        de += struct.pack("<IIII", icon_at[i], len(b), 0, 0)
    de_offsets.append(off_de + n * 16)
    de += struct.pack("<IIII", group_at, len(blob_group), 0, 0)

    body = root + t3 + sub3 + t14 + de + b"".join(blobs) + blob_group
    assert len(body) == group_at + len(blob_group)
    return body, de_offsets


def build_syso(rsrc, de_offsets):
    """COFF object，单个 .rsrc 节。

    Go 的 addpersrc 只在**带重定位**的 .rsrc 上工作：它把每个 DataEntry 的
    OffsetToData 改成 节RVA + addend（addend 取自字段原值，即节内相对偏移）。
    因此这里必须给出一个节符号 + 每个 DataEntry 的 ADDR32NB 重定位，
    否则图标指向错误地址，exe 显示默认图标。
    """
    data = rsrc
    if len(data) % 4:
        data += b"\x00" * (4 - len(data) % 4)
    raw_ptr = 20 + 40
    rel_ptr = raw_ptr + len(data)
    n_rel = len(de_offsets)
    # 重定位：IMAGEREL_AMD64_ADDR32NB = 0x0003
    reloc = b"".join(struct.pack("<IIH", off, 0, 0x0003) for off in de_offsets)
    sym_ptr = rel_ptr + len(reloc)
    # COFF 符号：名称为 ".rsrc"（8 字节内），STATIC(3)，节号 1
    sym = b".rsrc\x00\x00\x00" + struct.pack("<IHHBB", 0, 1, 0, 3, 0)

    coff = struct.pack("<HHIIIHH", 0x8664, 1, 0, sym_ptr, 1, 0, 0x0000)
    sect = b".rsrc\x00\x00\x00"
    sect += struct.pack(
        "<IIIIIIHHI",
        len(rsrc),          # VirtualSize
        0,                  # VirtualAddress（链接器分配）
        len(data),          # SizeOfRawData
        raw_ptr,            # PointerToRawData
        rel_ptr,            # PointerToRelocations
        0,                  # PointerToLinenumbers
        n_rel,              # NumberOfRelocations
        0,                  # NumberOfLinenumbers
        0x40000040,         # CNT_INITIALIZED_DATA | MEM_READ
    )
    return coff + sect + data + reloc + sym + struct.pack("<I", 4)  # 4 = 空字符串表长度


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--skip-syso", action="store_true")
    args = ap.parse_args()

    master = render_with_chrome()
    how = "chrome"
    if master is None:
        master = render_with_pillow()
        how = "pillow(8x超采样)"
    print("master rendered via", how, master.size)

    entries = build_entries(master)
    ico = build_ico(entries)
    ico_path = os.path.join(REPO, "packaging", "pos.ico")
    with open(ico_path, "wb") as f:
        f.write(ico)
    print("ico ->", ico_path, len(ico), "bytes", [e[0] for e in entries])

    png_path = os.path.join(REPO, "packaging", "icon-master-%d.png" % MASTER)
    master.save(png_path)
    print("png ->", png_path)

    # 前端图标：随 web/public 一起被 Vite 拷到 dist，再被 embed 进 exe
    pub_dir = os.path.join(REPO, "web", "public")
    os.makedirs(pub_dir, exist_ok=True)
    with open(os.path.join(pub_dir, "favicon.ico"), "wb") as f:
        f.write(ico)
    touch = master.resize((180, 180), Image.LANCZOS)
    touch.save(os.path.join(pub_dir, "apple-touch-icon.png"))
    print("favicon ->", os.path.join(pub_dir, "favicon.ico"), "+ apple-touch-icon.png")

    if not args.skip_syso:
        rsrc, de_offsets = build_rsrc(entries)
        syso = build_syso(rsrc, de_offsets)
        syso_path = os.path.join(REPO, "cmd", "posd", "rsrc_windows_amd64.syso")
        with open(syso_path, "wb") as f:
            f.write(syso)
        print("syso ->", syso_path, len(syso), "bytes, relocs=%d" % len(de_offsets))


if __name__ == "__main__":
    sys.exit(main())
