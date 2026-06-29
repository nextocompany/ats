# คำขอเปิด Azure Subscription ใหม่ — ระบบ HR ATS (POC)

> เอกสารนี้สำหรับส่งให้ **ผู้ดูแล Azure billing / จัดซื้อขององค์กร** เพื่อขอเปิด subscription ใหม่
> ทดแทนตัวเดิมที่ใช้ไม่ได้แล้ว ฝั่ง technical เตรียมพร้อม deploy ทันทีที่ได้ subscription

## บริบท (ทำไมต้องขอใหม่)

ระบบ HR ATS (POC) เดิมรันบน Azure subscription แบบ **Microsoft Partner Network (MPN) credit** เครดิตรายเดือนหมด → Azure **disable subscription อัตโนมัติ** (สถานะ "Warned/disabled, read-only") → แอปทั้งหมดล่ม และไม่สามารถ reactivate ได้เพราะสิทธิ์อยู่ที่ Partner Center

แนวทางแก้: ย้ายไป **Pay-As-You-Go (ไม่มี spending limit)** เพื่อไม่ให้ล่มซ้ำทุกครั้งที่เครดิตหมด

## สิ่งที่ขอให้ดำเนินการ (3 ข้อ)

**1. สร้าง Azure Subscription ใหม่**
- ประเภท: **Pay-As-You-Go** (ผูก payment method ขององค์กร, ไม่มี spending limit)
- ชื่อแนะนำ: `hr-ats-prod`
- *(หมายเหตุ: ที่ลองสร้างเองติด error — Nexto/MOSP ต้องสร้างผ่าน signup portal, ส่วน MCA "AzureSignup" ขึ้น `NoAzurePlanFound` คือยังไม่ได้เปิด Azure plan + ผูกบัตร จึงต้องให้ผู้ดูแล billing จัดการ)*

**2. ให้สิทธิ์ผู้ใช้บน subscription นั้น**
- ให้ `nextto@ert.co.th` เป็น **Owner** บน subscription ใหม่
- *จำเป็นต้องเป็น Owner* (ไม่ใช่แค่ Contributor) เพราะการ deploy ต้องสร้าง role assignment + Managed Identity + Key Vault
- ถ้าให้ได้แค่ **Contributor** กรุณาแจ้ง — ทีม technical จะปรับวิธี deploy (โหมด no-RBAC) ได้ แต่ Owner สะดวกกว่า

**3. เปิด / ยืนยัน Azure OpenAI access**
- ระบบใช้ Azure OpenAI (คัดกรอง CV ด้วย AI) — subscription ใหม่ต้องมี access
- กรอกฟอร์ม https://aka.ms/oai/access ด้วย subscription ID ใหม่ (หรือยืนยันว่า enable แล้ว)
- *(ช่วงหลัง sub ใหม่หลายตัวได้ access ทันทีไม่ต้องกรอก — ทีม technical จะตรวจตอน deploy)*

## ข้อมูลทางเทคนิค

- **ภูมิภาค**: Southeast Asia (ข้อมูล/แอป — ใกล้ไทย, สอดคล้อง PDPA) + East US (เฉพาะ AI model)
- **ค่าใช้จ่ายโดยประมาณ**: เดือนล่าสุดระบบใช้จริง **~$82 USD/เดือน** (ดึงจาก Azure Cost Management) — แนะนำตั้งงบ **~$100-150 USD/เดือน** เผื่อ AI usage เติบโต
  - ประกอบด้วย: PostgreSQL Flexible Server (Burstable B1ms) + Container Apps × 5 + Redis (Basic) + Storage + Container Registry + Log Analytics + Azure OpenAI (จ่ายตาม token) + Document Intelligence (จ่ายตามหน้า)
- **ไม่ต้องย้ายข้อมูลเดิม** — เริ่มใหม่ทั้งหมด (ข้อมูลเดิมกู้ไม่ได้และไม่จำเป็น)

## สิ่งที่ขอให้ส่งกลับมา (เพื่อเริ่ม deploy)

1. **Subscription ID** ใหม่
2. ยืนยันว่า `nextto@ert.co.th` ได้สิทธิ์ **Owner** (หรือแจ้งว่าได้แค่ Contributor)
3. สถานะ **Azure OpenAI access** ของ subscription นั้น

---
*ฝั่ง technical พร้อม deploy เต็มชุดทันทีที่ได้ 3 ข้อนี้ (infra-as-code + แผน deploy เตรียมไว้แล้วใน repo: `infra/prod-v2.bicepparam`)*
