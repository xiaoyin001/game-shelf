// 文件说明：三个真实180秒回合并行等待，不增加CPU或改变生产计时与浏览器预算。
export function serviceTestArgs(includeInfra = false) {
  return [
    "test",
    "-count=1",
    "-v",
    "-parallel",
    "3",
    ...(includeInfra ? ["./tests/infrastructure"] : []),
    "./tests/integration",
  ];
}
