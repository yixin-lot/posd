// v03 导航口径：10 个视图只在这里定义一次。
// 管理页左侧 nav64 与收银台 ☰ 抽屉都从这里取，按角色过滤，不再各写一份。
// manager: true 的项只对店长开放。
export const NAV_ITEMS = [
  {
    view: 'cashier',
    label: '收银台',
    menu: '收银台',
    desc: '回主屏，加购与结算',
    tone: 'teal',
    icon: [
      'M4 7h16v12H4z',
      'M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2',
      'M8 13h.01M16 13h.01M8 17h.01M12 17h.01'
    ]
  },
  {
    view: 'orders',
    label: '订单查询',
    menu: '订单查询',
    desc: '查单、退款、重打小票',
    tone: 'teal',
    icon: ['M8 6h13M8 12h13M8 18h13', 'M3.5 6h.01M3.5 12h.01M3.5 18h.01']
  },
  {
    view: 'tables',
    label: '桌台',
    menu: '桌台',
    desc: '开台、并台、转台',
    tone: 'teal',
    icon: ['M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z']
  },
  {
    view: 'products',
    label: '商品管理',
    menu: '商品管理',
    desc: '加商品、改价、停售',
    tone: 'teal',
    manager: true,
    icon: ['M21 8l-9-5-9 5 9 5z', 'M3 8v8l9 5 9-5V8', 'M12 13v8']
  },
  {
    view: 'shift',
    label: '交接班',
    menu: '交接班',
    desc: '交班结算、打印交接单',
    tone: 'amber',
    icon: ['M3 4h18v17H3z', 'M3 9h18', 'M9 4v17']
  },
  {
    view: 'members',
    label: '会员',
    menu: '会员',
    desc: '会员余额、充值',
    tone: 'blue',
    icon: ['M12 4a4 4 0 1 0 0 8 4 4 0 0 0 0-8z', 'M4 21a8 8 0 0 1 16 0']
  },
  {
    view: 'reports',
    label: '报表',
    menu: '报表',
    desc: '日报、月报、导出',
    tone: 'blue',
    manager: true,
    icon: ['M4 20V10M10 20V4M16 20V13M20 20H3']
  },
  {
    view: 'audit',
    label: '审计',
    menu: '审计',
    desc: '操作留痕、异常排查',
    tone: 'slate',
    manager: true,
    icon: ['M12 3l8 4v6c0 5-3.5 7.5-8 8-4.5-.5-8-3-8-8V7z']
  },
  {
    view: 'staff',
    label: '员工',
    menu: '员工',
    desc: '账号与权限',
    tone: 'slate',
    manager: true,
    icon: [
      'M9 4.5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7z',
      'M2 21a7 7 0 0 1 14 0',
      'M18 6.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5zM15.5 21a6 6 0 0 1 7-4.5'
    ]
  },
  {
    view: 'settings',
    label: '设置',
    menu: '设置',
    desc: '打印、门店、参数',
    tone: 'teal',
    manager: true,
    icon: [
      'M12 9a3 3 0 1 0 0 6 3 3 0 1 0 0-6',
      'M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1 1.55V21a2 2 0 1 1-4 0v-.09a1.7 1.7 0 0 0-1-1.55 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.7 1.7 0 0 0 .34-1.87 1.7 1.7 0 0 0-1.55-1H3a2 2 0 1 1 0-4h.09a1.7 1.7 0 0 0 1.55-1 1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.7 1.7 0 0 0 1.87.34h.01a1.7 1.7 0 0 0 1-1.55V3a2 2 0 1 1 4 0v.09a1.7 1.7 0 0 0 1 1.55 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87v.01a1.7 1.7 0 0 0 1.55 1H21a2 2 0 1 1 0 4h-.09a1.7 1.7 0 0 0-1.55 1z'
    ]
  }
]

// 隐藏页签（StatusBar 的 .tabs）沿用旧的短标签：脚本与快捷键按这套口径切页，别改
const SHORT = {
  cashier: '收银',
  orders: '订单',
  tables: '桌台',
  products: '商品',
  shift: '班次',
  members: '会员',
  reports: '报表',
  audit: '审计',
  staff: '员工',
  settings: '设置'
}

/** 按角色过滤后的导航项（short 供隐藏页签与旧脚本匹配） */
export function navItems(role) {
  return NAV_ITEMS.filter((t) => !t.manager || role === 'manager').map((t) => ({ ...t, short: SHORT[t.view] || t.label }))
}
