import { redirect } from "next/navigation";

// The portal entry point is the jobs list.
export default function Home() {
  redirect("/jobs");
}
