# Provider Credentials Guide — ของที่ต้องเตรียมเพื่อเปิด "real providers"

> ระบบ deploy ใหม่บน subscription ใหม่แล้ว (domain เปลี่ยน) เอกสารนี้บอกว่าแต่ละ provider
> ต้องเอา credential อะไร จากที่ไหน และ callback/redirect URL ที่ต้องลงทะเบียน (ค่าจริงของ domain ใหม่)
>
> **Base URLs (domain ใหม่):**
> - API: `https://hrats-prod-api.nicerock-8ba634a2.southeastasia.azurecontainerapps.io`
> - Portal: `https://hrats-prod-portal.nicerock-8ba634a2.southeastasia.azurecontainerapps.io`
> - Dashboard: `https://hrats-prod-dashboard.nicerock-8ba634a2.southeastasia.azurecontainerapps.io`
>
> ส่ง credential กลับมาให้ผม (หรือบอกว่าใส่เองใน Key Vault/container app) ผมจะ wire ด้วย `az containerapp update`

---

## 0. Azure OpenAI (AI screening) — ✅ ผมจัดการเอง ไม่ต้องทำอะไร
gpt-5-mini ที่ swedencentral กำลัง provision + ผม patch code + wire ให้เอง

---

## 1. LINE Login (ผู้สมัคร login ด้วย LINE) — `LINE_PROVIDER=real`

**ที่ไหน:** https://developers.line.biz/console/ → Provider → **LINE Login channel**

**ต้องได้:**
| ค่า | env var | หาที่ |
|---|---|---|
| Channel ID | `LINE_CHANNEL_ID` | channel → Basic settings |
| Channel secret | `LINE_CHANNEL_SECRET` | channel → Basic settings |
| LIFF ID | `NEXT_PUBLIC_LIFF_ID` (build-arg ของ portal) | channel → LIFF tab (สร้าง LIFF app, endpoint = portal URL) |

**ต้องลงทะเบียนใน console:**
- **Callback URL** (LINE Login → Callback URL):
  `https://hrats-prod-api.nicerock-8ba634a2.southeastasia.azurecontainerapps.io/api/v1/public/line/callback`
- LIFF endpoint URL: `https://hrats-prod-portal.nicerock-8ba634a2.southeastasia.azurecontainerapps.io`
- (ถ้าจะขอ email จากผู้สมัคร) เปิด **OpenID Connect → Email permission** + ตั้ง `LINE_REQUEST_EMAIL_SCOPE=true`

→ env ที่ผมจะ set: `LINE_PROVIDER=real`, `LINE_CHANNEL_ID`, `LINE_CHANNEL_SECRET`, `LINE_LOGIN_CALLBACK_URL=<callback ข้างบน>`

---

## 2. LINE Push + Email + Teams (แจ้งเตือนผู้สมัคร/HR) — `NOTIFY_PROVIDER=real`

ระบบส่งแจ้งเตือนผ่าน 3 ช่อง — ต้องการ:

| ช่อง | env var | หาที่ |
|---|---|---|
| LINE push | `NOTIFY_LINE_TOKEN` | LINE Developers → **Messaging API channel** (คนละ channel กับ Login) → Channel access token (long-lived) |
| Email (sender) | `NOTIFY_EMAIL_FROM` | อีเมลผู้ส่ง เช่น `no-reply@<domain>` (ผูกกับ ACS ข้อ 4) |
| Teams | `TEAMS_WEBHOOK_URL` | Teams channel → Connectors → **Incoming Webhook** → คัดลอก URL |

> หมายเหตุ: LINE มี 2 channel — **Login channel** (ข้อ 1, สำหรับ login) และ **Messaging API channel** (ข้อนี้, สำหรับ push) แยกกัน

---

## 3. Google OAuth (ผู้สมัคร login ด้วย Google) — `GOOGLE_PROVIDER=real`

**ที่ไหน:** https://console.cloud.google.com/ → APIs & Services → **Credentials** → Create **OAuth client ID** (type: Web application)

**ต้องได้:**
| ค่า | env var |
|---|---|
| Client ID | `GOOGLE_CLIENT_ID` |
| Client secret | `GOOGLE_CLIENT_SECRET` |

**ต้องลงทะเบียนใน console (Authorized redirect URI):**
`https://hrats-prod-api.nicerock-8ba634a2.southeastasia.azurecontainerapps.io/api/v1/public/auth/google/callback`

→ env: `GOOGLE_PROVIDER=real`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_CALLBACK_URL=<redirect ข้างบน>`

---

## 4. Email ส่งจริง (ACS) — `EMAIL_PROVIDER=real`

**แนะนำ: ผมสร้าง Azure Communication Services (ACS) + Email + Azure-managed domain ให้** (เร็ว, ส่งจาก address `donotreply@<random>.azurecomm.net`) — คุณไม่ต้องทำอะไร นอกจาก**ตัดสินใจ**:
- (ก) ใช้ **Azure-managed domain** (เร็ว, address ยาว ๆ azurecomm.net) — แนะนำเริ่มแบบนี้
- (ข) ใช้ **custom domain** (เช่น `mail.yourcompany.com`) — สวยกว่า แต่ต้องเพิ่ม DNS record (TXT/SPF/DKIM) ที่ผู้ดูแล DNS ของบริษัท

env ที่ผมจะ set เอง: `EMAIL_PROVIDER=real`, `ACS_EMAIL_ENDPOINT`, `ACS_EMAIL_ACCESS_KEY`, `ACS_EMAIL_SENDER`

> **ขอจากคุณ:** เลือก (ก) หรือ (ข) + ถ้า (ข) บอก domain ที่จะใช้

---

## 5. PeopleSoft (sync ตำแหน่ง/ผู้ได้รับการจ้าง) — `PS_PROVIDER=real` *(optional)*
ตอนนี้ปล่อย mock ไว้ก่อน ถ้าจะต่อจริงต้องมี: PeopleSoft REST endpoint + credentials (IB). บอกผมถ้าต้องการ

---

## สรุปสิ่งที่ต้องส่งกลับมา

**จำเป็นสำหรับ "real ทั้งหมด":**
1. LINE Login: channel ID + secret + LIFF ID  (+ เปิด email permission ถ้าต้องการ)
2. LINE Messaging: channel access token (`NOTIFY_LINE_TOKEN`)
3. Google: client ID + secret
4. Teams: incoming webhook URL
5. Email: เลือก managed domain (ก) หรือ custom (ข)

**callback URLs ที่ต้องไปลงทะเบียนใน console (คัดลอกได้เลย):**
- LINE: `https://hrats-prod-api.nicerock-8ba634a2.southeastasia.azurecontainerapps.io/api/v1/public/line/callback`
- Google: `https://hrats-prod-api.nicerock-8ba634a2.southeastasia.azurecontainerapps.io/api/v1/public/auth/google/callback`

> ทุกอย่างทยอยทำได้ — ระบบใช้งานได้ตอนนี้แล้ว (providers ที่ยังไม่ได้ต่อจะเป็น mock ไม่พัง) ส่งมาทีละตัวผมก็ wire ให้ทีละตัว
