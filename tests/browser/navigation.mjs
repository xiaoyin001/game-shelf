// 文件说明：测试按真实公开布局规划轴向绕障路线，不写游戏状态或关闭碰撞。
export function route(layout, start, goal, bodies = []) {
  const clear = (p, radius) =>
    p.x >= radius &&
    p.x <= layout.width - radius &&
    p.y >= radius &&
    p.y <= layout.height - radius &&
    layout.entities.every((e) =>
      e.radius
        ? Math.hypot(p.x - e.x, p.y - e.y) >= radius + e.radius
        : Math.hypot(
            p.x - Math.max(e.x - e.half_x, Math.min(e.x + e.half_x, p.x)),
            p.y - Math.max(e.y - e.half_y, Math.min(e.y + e.half_y, p.y)),
          ) >= radius,
    ) &&
    bodies.every((b) => Math.hypot(p.x - b.x, p.y - b.y) >= radius + 20 - 1e-6);
  const segment = (a, b, radius) => {
    const n = Math.ceil(Math.hypot(a.x - b.x, a.y - b.y) / 3);
    for (let i = 0; i <= n; i++) {
      const f = i / Math.max(1, n);
      if (!clear({ x: a.x + (b.x - a.x) * f, y: a.y + (b.y - a.y) * f }, radius)) return false;
    }
    return true;
  };
  const cols = layout.width / 20 - 1,
    rows = layout.height / 20 - 1,
    point = (i) => ({ x: ((i % cols) + 1) * 20, y: (Math.floor(i / cols) + 1) * 20 }),
    available = Array.from({ length: cols * rows }, (_, i) => clear(point(i), 36)),
    candidates = [];
  for (let i = 0; i < available.length; i++) {
    const p = point(i),
      corner = { x: p.x, y: start.y };
    if (
      available[i] &&
      Math.hypot(p.x - start.x, p.y - start.y) <= 80 &&
      segment(start, corner, 20) &&
      segment(corner, p, 20)
    )
      candidates.push(i);
  }
  candidates.sort(
    (a, b) =>
      Math.hypot(point(a).x - start.x, point(a).y - start.y) -
      Math.hypot(point(b).x - start.x, point(b).y - start.y),
  );
  if (!candidates.length) throw new Error("真实起点没有可步行的连接");
  const root = candidates[0],
    queue = [root],
    parents = new Map([[root, -1]]);
  let end = -1;
  for (let head = 0; head < queue.length; head++) {
    const i = queue[head],
      p = point(i);
    if (Math.hypot(p.x - goal.x, p.y - goal.y) <= 30) {
      end = i;
      break;
    }
    for (const [dx, dy] of [
      [1, 0],
      [-1, 0],
      [0, 1],
      [0, -1],
    ]) {
      const x = (i % cols) + dx,
        y = Math.floor(i / cols) + dy,
        j = y * cols + x;
      if (x < 0 || x >= cols || y < 0 || y >= rows || !available[j] || parents.has(j)) continue;
      if (segment(p, point(j), 36)) {
        parents.set(j, i);
        queue.push(j);
      }
    }
  }
  if (end < 0) throw new Error("目标附近没有真实可达的地面路线: " + JSON.stringify(goal));
  const path = [];
  for (let i = end; i >= 0; i = parents.get(i)) path.push(point(i));
  path.reverse();
  const out = [{ x: path[0].x, y: start.y }, path[0]];
  for (const p of path.slice(1)) {
    const a = out.at(-2),
      b = out.at(-1);
    if (a && ((a.x === b.x && b.x === p.x) || (a.y === b.y && b.y === p.y)))
      out[out.length - 1] = p;
    else out.push(p);
  }
  return out;
}
