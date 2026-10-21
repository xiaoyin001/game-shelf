// 文件说明：启动真实开发依赖并运行基础设施烟测，finally 只回收自己的保活管道。
import { startInfra, run } from "./infra.mjs";
const stop = await startInfra();
try {
  run("go", ["test", "-count=1", "-v", "./tests/infrastructure"], { GAME_SHELF_INFRA: "1" });
} finally {
  await stop();
}
