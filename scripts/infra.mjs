// 文件说明：管理真实本地依赖与自己的 WSL 保活管道，不接管外部服务或删除卷。
import {spawn,spawnSync} from "node:child_process";
import {createConnection} from "node:net";
import {resolve,dirname} from "node:path";
import {fileURLToPath} from "node:url";
import {setTimeout as delay} from "node:timers/promises";
export const root=resolve(dirname(fileURLToPath(import.meta.url)),"..");
// listening 只探测本机 TCP，后续真实协议测试仍负责验证服务种类。
export function listening(port){return new Promise(done=>{
 const conn=createConnection({host:"127.0.0.1",port});
 const finish=ok=>{conn.destroy();done(ok);};conn.setTimeout(700);
 conn.once("connect",()=>finish(true));conn.once("error",()=>finish(false));conn.once("timeout",()=>finish(false));
});}
// run 用确定命令执行本地构建或依赖操作，不执行外部输入拼接的 shell。
export function run(file,args,env={}){const r=spawnSync(file,args,{cwd:root,stdio:"inherit",windowsHide:true,env:{...process.env,...env}});if(r.error||r.status!==0)throw r.error||new Error(file+" exited "+r.status);}
// startInfra 返回只回收自有保活进程的函数，已有依赖不主动停止。
export async function startInfra(){
 const ports=[2379,6379,27017];let keeper=null;
 const stop=async()=>{
  if(!keeper||keeper.exitCode!==null||keeper.signalCode!==null)return;
  keeper.stdin.end();
  await Promise.race([new Promise(done=>keeper.once("exit",done)),delay(3000)]);
  if(keeper.exitCode===null&&keeper.signalCode===null)keeper.kill();
 };
 try{
  const docker=spawnSync("docker",["compose","version"],{stdio:"ignore",windowsHide:true});
  let wsl=false;
  if((docker.error||docker.status!==0)&&process.platform==="win32"){
   const probe=spawnSync("wsl",["--","docker","compose","version"],{stdio:"ignore",windowsHide:true});
   if(!probe.error&&probe.status===0){
    wsl=true;keeper=spawn("wsl",["--exec","cat"],{cwd:root,stdio:["pipe","ignore","ignore"],windowsHide:true});
    await new Promise((done,fail)=>{keeper.once("spawn",done);keeper.once("error",fail);});
   }
  }
  const found=await Promise.all(ports.map(listening));
  if(!found.every(Boolean)){
   if(found.some(Boolean))throw new Error("依赖端口部分被占用，拒绝覆盖已有实例。");
   const args=["compose","-p","game-shelf","-f","deploy/docker-compose.yml","up","-d"];
   if(!docker.error&&docker.status===0)run("docker",args);
   else if(wsl)run("wsl",["--","docker",...args]);
   else throw new Error("需要本地 Docker Compose 或已就绪的 etcd、Redis、MongoDB。");
   const deadline=Date.now()+45000;
   while(!(await Promise.all(ports.map(listening))).every(Boolean)){
    if(Date.now()>deadline)throw new Error("依赖未在 45 秒内就绪。");await delay(300);
   }
  }
  return stop;
 }catch(error){await stop();throw error;}
}
