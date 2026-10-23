// 文件说明：Git推送前只读检查当前提交证据与真实refs；不运行隐式测试、不替代owner授权。
import { checkProof, checkUpdates } from "./push-proof.mjs";
const args = process.argv.slice(2);
try {
  if (args.length === 1 && args[0] === "--check") {
    const proof = await checkProof();
    console.log("当前提交验收有效：" + proof.commit + "；仍需单独Push授权。");
  } else {
    if (args.length !== 2) throw new Error("pre-push需要Git提供的remote参数");
    let text = "";
    for await (const chunk of process.stdin) text += chunk;
    if (text.trim()) {
      const proof = await checkProof();
      checkUpdates(text, proof, args[1]);
      console.log("pre-push：当前提交完整验收有效，refs校验通过。");
    }
  }
} catch (error) {
  console.error(
    "拒绝推送：" +
      error.message +
      "\n请先在干净最终提交执行 npm run verify:ci；不要用候选/旧报告代替，也不要绕过本次owner授权。",
  );
  process.exitCode = 1;
}
