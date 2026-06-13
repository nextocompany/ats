import { chromium } from "@playwright/test";
const BASE="http://localhost:3000";
const b=await chromium.launch();
const c=await b.newContext();
await c.addCookies([{name:"hr_session",value:"dev",url:BASE}]);
for (const [tag,w,h] of [["1440",1440,1300],["390",390,1700]]) {
  const p=await c.newPage();
  await p.setViewportSize({width:w,height:h});
  await p.goto(`${BASE}/dashboard`,{waitUntil:"networkidle"});
  await p.waitForFunction(()=>!document.body.innerText.includes("Loading"),{timeout:8000}).catch(()=>{});
  await p.waitForTimeout(1800);
  await p.screenshot({path:`/tmp/clarity-shots/dashboard-ops-${tag}.png`});
  console.log("shot",tag); await p.close();
}
await b.close();
