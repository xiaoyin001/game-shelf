// 文件说明：固定 MongoDB 8.0.34 的 CI 仅采用已验收内核范围，不覆盖或绕过数据库保护。
export function checkCiKernel(release) {
  const version = /^(\d+)\.(\d+)(?:\.|-|$)/.exec(release);
  if (
    !version ||
    Number(version[1]) > 6 ||
    (Number(version[1]) === 6 && Number(version[2]) >= 19)
  ) {
    throw new Error(
      "当前 MongoDB 8.0.34 CI 仅验收 Linux <6.19；实际内核 " +
        release +
        "。参见 ADR-0013 / SERVER-131779，不能跳过数据库或绕过内核检查。",
    );
  }
}
