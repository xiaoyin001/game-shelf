// 文件说明：仅等待调用方已拥有的子进程；退出或到期均回收监听与计时器，不残留虚假等待。
export async function waitForExit(child, timeout) {
  const exited = () => child.exitCode !== null || child.signalCode !== null;
  if (exited()) return true;
  await new Promise((done) => {
    const finish = () => {
      clearTimeout(timer);
      child.removeListener("exit", finish);
      done();
    };
    const timer = setTimeout(finish, timeout);
    child.once("exit", finish);
    if (exited()) finish();
  });
  return exited();
}
