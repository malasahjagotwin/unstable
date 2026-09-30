export const SESSION_COOKIE = "unstable_key";

export const LOGIN_PATH = "/auth/login";

export const DASHBOARD_PATH = "/dashboard";

export const SESSION_MAX_AGE_SECONDS = 60 * 60 * 24 * 30;

const MAX_KEY_LENGTH = 128;

const VISIBLE_PREFIX_LENGTH = 4;

const MAX_HIDDEN_CHARACTERS = 12;

export type KeyRejection = "empty" | "too-long" | "illegal-characters";

export type KeyCheck =
  | { ok: true; key: string }
  | { ok: false; reason: KeyRejection };

const PRINTABLE_ASCII = /^[\x21-\x7e]+$/;

export function checkKey(raw: string): KeyCheck {
  const key = raw.trim();

  if (key.length === 0) {
    return { ok: false, reason: "empty" };
  }
  if (key.length > MAX_KEY_LENGTH) {
    return { ok: false, reason: "too-long" };
  }
  if (!PRINTABLE_ASCII.test(key)) {
    return { ok: false, reason: "illegal-characters" };
  }
  return { ok: true, key };
}

export function maskKey(key: string): string {
  if (key.length <= VISIBLE_PREFIX_LENGTH) {
    return "*".repeat(key.length);
  }
  const hidden = Math.min(key.length - VISIBLE_PREFIX_LENGTH, MAX_HIDDEN_CHARACTERS);
  return `${key.slice(0, VISIBLE_PREFIX_LENGTH)}${"*".repeat(hidden)}`;
}

export function describeKeyRejection(reason: KeyRejection): string {
  switch (reason) {
    case "empty":
      return "请填写 key";
    case "too-long":
      return `key 不能超过 ${MAX_KEY_LENGTH} 个字符`;
    case "illegal-characters":
      return "key 只能包含不带空格的可见 ASCII 字符";
  }
}
