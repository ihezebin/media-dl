/** Catalog duration does not describe the actual stream, especially a preview. */
export function playableDuration(mediaDuration: number, trialDuration = 0) {
  const media =
    Number.isFinite(mediaDuration) && mediaDuration > 0 ? mediaDuration : 0;
  const trial =
    Number.isFinite(trialDuration) && trialDuration > 0 ? trialDuration : 0;
  return media && trial ? Math.min(media, trial) : media || trial;
}

export function playbackSeek(value: number, duration: number) {
  if (!Number.isFinite(value) || !Number.isFinite(duration) || duration <= 0)
    return null;
  return Math.max(0, Math.min(value, duration));
}
