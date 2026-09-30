import { describe, expect, it } from "vitest";

import { checkKey, describeKeyRejection, maskKey } from "@/lib/session";

describe("checkKey", () => {
  it("接受合法 key 并去掉首尾空白", () => {
    expect(checkKey("  usr_k8f9a2b4c1d6e3f5  ")).toEqual({
      ok: true,
      key: "usr_k8f9a2b4c1d6e3f5",
    });
  });

  it("拒绝纯空白输入", () => {
    expect(checkKey("   ")).toEqual({ ok: false, reason: "empty" });
  });

  it("拒绝空字符串", () => {
    expect(checkKey("")).toEqual({ ok: false, reason: "empty" });
  });

  it("接受正好 128 个字符的 key", () => {
    const key = "a".repeat(128);
    expect(checkKey(key)).toEqual({ ok: true, key });
  });

  it("拒绝超过 128 个字符的 key", () => {
    expect(checkKey("a".repeat(129))).toEqual({ ok: false, reason: "too-long" });
  });

  it("拒绝内部含空格的 key", () => {
    expect(checkKey("usr key")).toEqual({
      ok: false,
      reason: "illegal-characters",
    });
  });

  it("拒绝非 ASCII 字符", () => {
    expect(checkKey("usr_密钥")).toEqual({
      ok: false,
      reason: "illegal-characters",
    });
  });

  it("拒绝控制字符", () => {
    expect(checkKey("usr\nkey")).toEqual({
      ok: false,
      reason: "illegal-characters",
    });
  });
});

describe("maskKey", () => {
  it("只保留前四个字符", () => {
    expect(maskKey("usr_k8f9a2b4c1d6e3f5")).toBe("usr_************");
  });

  it("长度不超过四时全部隐藏", () => {
    expect(maskKey("usr_")).toBe("****");
  });

  it("隐藏字符不超过上限", () => {
    expect(maskKey("a".repeat(40))).toBe("aaaa************");
  });
});

describe("describeKeyRejection", () => {
  it("为每种拒绝原因给出中文提示", () => {
    expect(describeKeyRejection("empty")).toBe("请填写 key");
    expect(describeKeyRejection("too-long")).toContain("128");
    expect(describeKeyRejection("illegal-characters")).toContain("ASCII");
  });
});
