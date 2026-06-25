import { test, expect } from "@playwright/test";

import { buildApplyForm } from "../lib/queries";

// Pure unit checks for the multipart builder — no browser needed. Verifies the
// exact field names the Go handler reads (name_th, name_en, position_id, …).
test.describe("buildApplyForm", () => {
  const resume = new File([new Uint8Array([1, 2, 3])], "cv.pdf", { type: "application/pdf" });

  test("sets the required fields the backend expects", () => {
    const form = buildApplyForm({
      positionId: "pos-1",
      nameTH: "สมชาย ใจดี",
      nameEN: "Somchai Jaidee",
      consentVersion: "1.0",
      resume,
    });
    expect(form.get("position_id")).toBe("pos-1");
    expect(form.get("name_th")).toBe("สมชาย ใจดี");
    expect(form.get("name_en")).toBe("Somchai Jaidee");
    expect(form.get("consent_given")).toBe("true");
    expect(form.get("consent_version")).toBe("1.0");
    expect(form.get("resume")).toBeInstanceOf(File);
  });

  test("omits empty optional fields", () => {
    const form = buildApplyForm({
      positionId: "pos-1",
      nameTH: "Test",
      nameEN: "Test",
      consentVersion: "1.0",
      resume,
    });
    expect(form.has("phone")).toBe(false);
    expect(form.has("email")).toBe(false);
    expect(form.has("id_card")).toBe(false);
    expect(form.has("province")).toBe(false);
  });

  test("includes provided optional fields", () => {
    const form = buildApplyForm({
      positionId: "pos-1",
      nameTH: "Test",
      nameEN: "Test",
      phone: "0812345678",
      province: "กรุงเทพมหานคร",
      consentVersion: "1.0",
      resume,
    });
    expect(form.get("phone")).toBe("0812345678");
    expect(form.get("province")).toBe("กรุงเทพมหานคร");
  });
});
