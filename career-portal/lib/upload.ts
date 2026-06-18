// Shared client-side upload constraints, mirroring the server gate (415/413) for
// instant inline feedback. Used by the resume upload and the onboarding document
// upload so the accepted-type/size rules have a single source of truth.

export const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;

export const ACCEPTED_UPLOAD_TYPES = new Set([
  "application/pdf",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "image/jpeg",
  "image/png",
]);

// The matching `accept` attribute for a file <input>.
export const UPLOAD_ACCEPT_ATTR = ".pdf,.docx,image/jpeg,image/png";

// An i18n-key error code (not display copy) so each caller renders it in its own
// locale namespace.
export type UploadFileError = "fileTypeInvalid" | "fileTooLarge";

export function validateUploadFile(file: File): UploadFileError | null {
  if (!ACCEPTED_UPLOAD_TYPES.has(file.type)) return "fileTypeInvalid";
  if (file.size > MAX_UPLOAD_BYTES) return "fileTooLarge";
  return null;
}
