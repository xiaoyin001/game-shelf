// 文件说明：验证 AI 入口、技术决策与文档链接可用，且不会被 Git 忽略。
import test from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir, access } from "node:fs/promises";
import { resolve, dirname, join, relative } from "node:path";
import { spawnSync } from "node:child_process";
const root=resolve(".");
async function markdown(dir) {
 const result=[];
 for(const entry of await readdir(dir,{withFileTypes:true})) {
  if(entry.name===".git"||entry.name==="node_modules"||entry.name===".local")continue;
  const path=join(dir,entry.name);
  if(entry.isDirectory()) result.push(...await markdown(path));
  else if(entry.name.endsWith(".md"))result.push(path);
 }
 return result;
}
test("跨 AI 入口、阶段记录与技能齐全",async()=>{
 for(const file of ["AGENTS.md","README.md","docs/progress.md","docs/playable-mvp.md","docs/ai-development.md",
 ".agents/skills/coding-standards/SKILL.md",".agents/skills/validate-change/SKILL.md",".agents/skills/prepare-commit/SKILL.md"]) {
  assert.ok((await readFile(join(root,file),"utf8")).trim(),file);
 }
 const rules=await readFile(join(root,"AGENTS.md"),"utf8");
 assert.match(rules,/master/);assert.match(rules,/不创建或使用.*main/);assert.match(rules,/每次 Push/);
});
test("相对 Markdown 链接在当前版本确实存在",async()=>{
 for(const file of await markdown(root)) {
  const body=await readFile(file,"utf8");
  for(const match of body.matchAll(/\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g)) {
   const link=match[1].split("#")[0];
   if(!link||/^[a-z]+:/i.test(link))continue;
   await access(resolve(dirname(file),decodeURIComponent(link)));
  }
 }
});
test("已采用的 ADR 有比较、原因、验证与重评条件",async()=>{
 for(const name of await readdir(join(root,"docs/decisions"))) {
  if(!/^\d{4}-.+\.md$/.test(name))continue;
  const body=await readFile(join(root,"docs/decisions",name),"utf8");
  for(const heading of ["状态：","## 背景","## 比较","## 决定","## 验证","## 重评条件"])assert.ok(body.includes(heading),name+":"+heading);
 }
});
test("目标、阶段与决策文档不被 Git 忽略",async()=>{
 const files=(await markdown(join(root,"docs"))).map(file=>relative(root,file).replaceAll("\\","/"));
 const result=spawnSync("git",["check-ignore","--no-index","--non-matching","--verbose","--stdin"],{input:files.join("\n")+"\n",encoding:"utf8",cwd:root});
 // git 以 1 表示没有任何忽略项，这正是预期结果；其他错误不能放行。
 assert.ok(result.status===0||result.status===1,result.stderr||String(result.error));
 const lines=result.stdout.trim().split("\n"); assert.equal(lines.length,files.length);
 for(const line of lines)assert.ok(line.startsWith("::"),"文档被忽略："+line);
});
