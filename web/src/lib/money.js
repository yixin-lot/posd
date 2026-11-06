// 金额与抹零的纯计算，从 App.vue 抽出供单测覆盖（行为与原来逐行一致）。
// 口径与服务端一致：抹零向下取整到分（jiao = 抹分，yuan = 抹角）。

/** 购物车合计（元）。items: [{ price, qty }] */
export function cartTotal(items) {
  return (items || []).reduce((sum, it) => sum + it.price * it.qty, 0)
}

/**
 * 应收金额（元）。
 * @param {number} total 合计金额（元）
 * @param {'none'|'jiao'|'yuan'} mode 抹零方式
 */
export function roundPayable(total, mode = 'none') {
  const cents = Math.round(total * 100)
  let off = 0
  if (mode === 'jiao') off = cents % 10
  else if (mode === 'yuan') off = cents % 100
  return (cents - off) / 100
}
