// 文件说明：双浏览器用真实按键迈上现有低石，验证模型/权威高度、停留和走离落地，不按跳跃键。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";

const coordinate = async (page, axis) =>
  Number(await page.locator("#coordinates").getAttribute("data-" + axis));

test("矮石可以直接迈上并走过，另一旅人看到真实顶面高度", async ({ browser }, info) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      name = unique("step");
    const errors = [];
    pa.on("pageerror", (error) => errors.push(error.message));
    pb.on("pageerror", (error) => errors.push(error.message));
    await join(pa, name);
    await join(pb, unique("stepwatch"));
    const layout = await (await pa.request.get("/scene.json")).json();
    const rock = layout.entities.find((entity) => entity.id === "rock-4");
    expect(rock.height).toBe(8);
    await pb.keyboard.down("w");
    await expect.poll(() => coordinate(pb, "y"), { intervals: [50] }).toBeLessThan(340);
    await pb.keyboard.up("w");
    await pa.keyboard.down("s");
    await expect.poll(() => coordinate(pa, "y"), { intervals: [25] }).toBeGreaterThan(445);
    await pa.keyboard.up("s");
    expect(Math.abs((await coordinate(pa, "y")) - rock.y)).toBeLessThan(28);
    // 短按在小石头上停住，不能只记录已经走过去的一瞬高度当作站稳。
    for (let i = 0; i < 8 && (await coordinate(pa, "z")) !== rock.height; i++) {
      const before = await coordinate(pa, "x");
      await pa.keyboard.press("d", { delay: 120 });
      await expect.poll(() => coordinate(pa, "x"), { intervals: [25] }).toBeGreaterThan(before);
    }
    await expect(pa.locator("#coordinates")).toHaveAttribute("data-z", String(rock.height));
    await expect(pa.locator("#coordinates")).toHaveAttribute("data-grounded", "true");
    const peer = pb.locator("#players li").filter({ hasText: name });
    await expect(peer).toHaveAttribute("data-z", String(rock.height));
    await expect(peer).toHaveAttribute("data-grounded", "true");
    await expect
      .poll(async () => Number(await pa.locator("#world").getAttribute("data-self-height")))
      .toBeGreaterThan(7.5);
    await pa.waitForTimeout(300);
    expect(await coordinate(pa, "z")).toBe(rock.height);
    await pa.screenshot({ path: info.outputPath("low-stone-step.png"), fullPage: true });
    await pa.keyboard.down("d");
    await expect.poll(() => coordinate(pa, "x"), { intervals: [50] }).toBeGreaterThan(rock.x + 50);
    await pa.keyboard.up("d");
    await expect(pa.locator("#coordinates")).toHaveAttribute("data-z", "0");
    await expect(pa.locator("#coordinates")).toHaveAttribute("data-grounded", "true");
    await expect(peer).toHaveAttribute("data-z", "0");
    expect(errors).toEqual([]);
  } finally {
    await Promise.all([a.close(), b.close()]);
  }
});
