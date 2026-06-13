"use client";

import { useState } from "react";

import { ConsentStep } from "@/components/ConsentStep";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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

// ProfileForm edits the saved profile (name / phone / LINE id / province). LINE
// signups already have a LINE identity; email/Google signups type their @line id
// here and can link LINE separately for push.
export function ProfileForm({ account, requireConsent, submitLabel, onSaved }: ProfileFormProps) {
  const { refresh } = useCandidate();
  const [fullName, setFullName] = useState(account.full_name);
  const [phone, setPhone] = useState(account.phone);
  const [lineId, setLineId] = useState(account.line_display_id);
  const [province, setProvince] = useState(account.province);
  const [consent, setConsent] = useState(account.pdpa_consent);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const needsConsent = requireConsent && !consent;
  const valid = fullName.trim().length > 0 && !needsConsent;

  async function save() {
    setError(null);
    if (!valid) return;
    setBusy(true);
    try {
      await updateProfile({
        full_name: fullName.trim(),
        phone: phone.trim(),
        line_display_id: lineId.trim(),
        province: province.trim(),
        consent_given: requireConsent ? consent : undefined,
        consent_version: requireConsent ? CONSENT_VERSION : undefined,
      });
      await refresh();
      onSaved();
    } catch {
      setError("บันทึกข้อมูลไม่สำเร็จ กรุณาลองใหม่");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-5">
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="full_name">
            ชื่อ-นามสกุล <span className="text-destructive">*</span>
          </Label>
          <Input id="full_name" value={fullName} onChange={(e) => setFullName(e.target.value)} autoComplete="name" placeholder="เช่น สมชาย ใจดี" />
        </div>
        <div className="space-y-2">
          <Label htmlFor="phone">เบอร์โทรศัพท์</Label>
          <Input id="phone" type="tel" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} autoComplete="tel" />
        </div>
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
