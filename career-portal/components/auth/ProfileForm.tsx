"use client";

import { useState } from "react";

import { ConsentStep } from "@/components/ConsentStep";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { updateProfile } from "@/lib/auth";
import { useCandidate } from "@/lib/session";
import type { Account } from "@/lib/types";

const CONSENT_VERSION = "1.0";

interface ProfileFormProps {
  account: Account;
  // requireConsent shows + enforces the PDPA checkbox (signup). On the account page
  // consent is already given, so it can be hidden.
  requireConsent?: boolean;
  submitLabel: string;
  onSaved: () => void;
}

// ProfileForm edits the saved profile. The identity is split into three names:
// a cosmetic Display Name (prefilled from the LINE/Google login, optional, never
// matched) and the Thai + English full names, which are REQUIRED and are what the
// resume name-match compares against. Email/Google signups type their @line id here.
export function ProfileForm({ account, requireConsent, submitLabel, onSaved }: ProfileFormProps) {
  const { refresh } = useCandidate();
  const [displayName, setDisplayName] = useState(account.display_name);
  const [nameTH, setNameTH] = useState(account.name_th);
  const [nameEN, setNameEN] = useState(account.name_en);
  const [phone, setPhone] = useState(account.phone);
  // Email is set-once. Show an editable field only when the account has none yet
  // (LINE signups); once set it is shown read-only so an identity email is never
  // edited away here.
  const canSetEmail = !account.email;
  const [email, setEmail] = useState(account.email);
  const [lineId, setLineId] = useState(account.line_display_id);
  const [province, setProvince] = useState(account.province);
  const [consent, setConsent] = useState(account.pdpa_consent);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const needsConsent = requireConsent && !consent;
  // Both names are required and always submitted together (they're the pair used to
  // match against the resume name).
  const valid = nameTH.trim().length > 0 && nameEN.trim().length > 0 && !needsConsent;

  async function save() {
    setError(null);
    if (!valid) return;
    setBusy(true);
    try {
      await updateProfile({
        name_th: nameTH.trim(),
        name_en: nameEN.trim(),
        display_name: displayName.trim() || undefined,
        phone: phone.trim(),
        email: canSetEmail && email.trim() ? email.trim() : undefined,
        line_display_id: lineId.trim(),
        province: province.trim(),
        consent_given: requireConsent ? consent : undefined,
        consent_version: requireConsent ? CONSENT_VERSION : undefined,
      });
      await refresh();
      onSaved();
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setError("อีเมลนี้ถูกใช้กับบัญชีอื่นแล้ว");
      } else if (e instanceof ApiError && e.status === 400) {
        setError("ข้อมูลไม่ถูกต้อง กรุณาตรวจสอบอีกครั้ง");
      } else {
        setError("บันทึกข้อมูลไม่สำเร็จ กรุณาลองใหม่");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-5">
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="display_name">ชื่อที่แสดง</Label>
          <Input id="display_name" value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="ชื่อจาก LINE" />
          <p className="text-xs text-muted-foreground">ดึงจากบัญชี LINE โดยอัตโนมัติ ใช้แสดงผลเท่านั้น (ไม่บังคับ)</p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="name_th">
            ชื่อ-นามสกุล (ภาษาไทย) <span className="text-destructive">*</span>
          </Label>
          <Input id="name_th" value={nameTH} onChange={(e) => setNameTH(e.target.value)} autoComplete="name" placeholder="เช่น สมชาย ใจดี" />
        </div>
        <div className="space-y-2">
          <Label htmlFor="name_en">
            ชื่อ-นามสกุล (ภาษาอังกฤษ) <span className="text-destructive">*</span>
          </Label>
          <Input id="name_en" value={nameEN} onChange={(e) => setNameEN(e.target.value)} autoComplete="name" placeholder="e.g. Somchai Jaidee" />
          <p className="text-xs text-muted-foreground">ชื่อ-นามสกุลใช้ตรวจสอบให้ตรงกับชื่อในเรซูเม่ที่อัปโหลด</p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="phone">เบอร์โทรศัพท์</Label>
          <Input id="phone" type="tel" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} autoComplete="tel" />
        </div>
        {canSetEmail ? (
          <div className="space-y-2">
            <Label htmlFor="email">อีเมล</Label>
            <Input id="email" type="email" inputMode="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" placeholder="you@example.com" />
            <p className="text-xs text-muted-foreground">ใช้สำหรับรับการแจ้งเตือนสถานะใบสมัคร</p>
          </div>
        ) : (
          <div className="space-y-2">
            <Label htmlFor="email">อีเมล</Label>
            <Input id="email" value={account.email} readOnly disabled className="bg-surface-muted text-muted-foreground" />
          </div>
        )}
        {!account.line_linked ? (
          <div className="space-y-2">
            <Label htmlFor="line_id">LINE ID</Label>
            <Input id="line_id" value={lineId} onChange={(e) => setLineId(e.target.value)} placeholder="@yourlineid" />
          </div>
        ) : null}
        <div className="space-y-2">
          <Label htmlFor="province">จังหวัด</Label>
          <Input id="province" value={province} onChange={(e) => setProvince(e.target.value)} />
        </div>
      </div>

      {requireConsent ? <ConsentStep checked={consent} onChange={setConsent} /> : null}

      {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}

      <Button type="button" size="tap" onClick={save} disabled={!valid || busy} className="w-full">
        {busy ? "กำลังบันทึก…" : submitLabel}
      </Button>
    </div>
  );
}
