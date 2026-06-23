import { redirect } from "next/navigation";

// Members was unified into the account-keyed "Candidates" surface (Phase 1 of the
// Candidates+Members unify). This route is kept only to redirect any bookmarks or
// in-flight links to the single list.
export default function MembersRedirect() {
  redirect("/candidates");
}
