import { redirect } from "next/navigation";

// A member id IS an account id, which the unified person detail at
// /candidates/[id] resolves directly. Redirect old /members/[id] bookmarks there.
export default async function MemberDetailRedirect({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  redirect(`/candidates/${id}`);
}
