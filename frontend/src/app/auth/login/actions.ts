"use server";

import { redirect } from "next/navigation";

import { writeSessionKey } from "@/lib/session-store";
import {
  checkKey,
  describeKeyRejection,
  DASHBOARD_PATH,
  LOGIN_PATH,
} from "@/lib/session";

function readSubmittedKey(formData: FormData): string {
  const submitted = formData.get("key");
  return typeof submitted === "string" ? submitted : "";
}

export async function login(formData: FormData): Promise<void> {
  const result = checkKey(readSubmittedKey(formData));

  if (!result.ok) {
    redirect(`${LOGIN_PATH}?error=${encodeURIComponent(describeKeyRejection(result.reason))}`);
  }

  await writeSessionKey(result.key);
  redirect(DASHBOARD_PATH);
}
