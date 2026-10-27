// 文件说明：三个真实180秒回合并行等待，不增加CPU或改变生产计时与浏览器预算。
export const longServiceTests = Object.freeze([
  "TestLiveChallengeComplete180Seconds",
  "TestLiveCoopComplete180Seconds",
  "TestLivePracticeDoesNotPersist180Seconds",
]);
export const longServicePattern = "^(" + longServiceTests.join("|") + ")$";
export function serviceTestArgs(includeInfra = false, phase = "all") {
  if (!["all", "short", "long"].includes(phase)) throw new Error("unknown service test phase");
  return [
    "test",
    "-count=1",
    "-v",
    "-parallel",
    "3",
    ...(phase === "short"
      ? ["-skip", longServicePattern]
      : phase === "long"
        ? ["-run", longServicePattern]
        : []),
    ...(includeInfra ? ["./tests/infrastructure"] : []),
    "./tests/integration",
  ];
}
