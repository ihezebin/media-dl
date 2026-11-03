import type { MusicSong } from "./media";

export function needsKugouVerification(song: MusicSong) {
  return (
    song.source === "kugou" &&
    (song.extra?.invalid_reason || song.extra?.access_reason || "").includes(
      "酷狗网页需要安全验证",
    )
  );
}

// Returning from the provider window is not proof of successful verification.
// Invalidate only the stale challenge result; the next action resolves again.
export function resetVerificationState(song: MusicSong): MusicSong {
  if (!needsKugouVerification(song)) return song;
  const extra = { ...song.extra };
  for (const key of ["invalid_reason", "access_reason"]) {
    if (extra[key]?.includes("酷狗网页需要安全验证")) delete extra[key];
  }
  return { ...song, url: "", is_invalid: true, extra };
}

export function isTrialSong(song: MusicSong) {
  const extra = song.extra || {};
  const truthy = (value: unknown) =>
    value === true || value === 1 || value === "1" || value === "true";
  const reason = extra.invalid_reason || extra.access_reason || "";
  return (
    truthy(song.is_trial) ||
    truthy(extra.is_trial) ||
    truthy(extra.is_preview) ||
    /仅.{0,2}试听|只能试听|试听片段|preview stream|trial only/i.test(reason)
  );
}

/** Only explicit metadata/reasons identify access restrictions; a failed URL is not proof of VIP. */
export function songAccessTags(song: MusicSong) {
  const extra = song.extra || {};
  const truthy = (value: unknown) =>
    value === true || value === 1 || value === "1" || value === "true";
  const reason = extra.invalid_reason || extra.access_reason || "";
  const vip =
    truthy(song.is_vip) ||
    truthy(extra.is_vip) ||
    /需要\s*VIP|VIP\s*(专享|歌曲|权限)|会员专享|requires?\s+(?:a\s+)?(?:vip|premium)/i.test(
      reason,
    );
  const login =
    truthy(song.requires_login) ||
    truthy(extra.requires_login) ||
    truthy(extra.need_login) ||
    /需要登录|请先登录|未登录|登录后|login required|requires? login|not logged in/i.test(
      reason,
    );
  const tags: { label: string; color: string }[] = [];
  if (vip) tags.push({ label: "VIP", color: "gold" });
  if (/需要付费|需要购买|付费专享/.test(reason))
    tags.push({ label: "需付费", color: "gold" });
  if (login) tags.push({ label: "需要登录", color: "blue" });
  if (isTrialSong(song)) tags.push({ label: "仅试听", color: "purple" });
  if (song.is_invalid && !needsKugouVerification(song))
    tags.push({ label: "解析待重试", color: "default" });
  return tags;
}
