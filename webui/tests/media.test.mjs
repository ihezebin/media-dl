import assert from 'node:assert/strict'
import test from 'node:test'

import { playableDuration, playbackSeek } from '../src/api/playbackTime.ts'
import { isTrialSong, needsKugouVerification, resetVerificationState, songAccessTags } from '../src/api/songAccess.ts'

test('failed URL probes remain retryable without inventing VIP or login requirements', () => {
  const song = { source: 'netease', is_invalid: true, extra: { invalid_reason: 'HTTP 403' } }
  assert.deepEqual(songAccessTags(song).map((tag) => tag.label), ['解析待重试'])
  assert.equal(isTrialSong(song), false)
})

test('explicit access reasons retain login, VIP and preview information', () => {
  const song = { source: 'kugou', is_invalid: true, extra: { invalid_reason: '请先登录，VIP 歌曲仅提供试听片段' } }
  assert.deepEqual(songAccessTags(song).map((tag) => tag.label), ['VIP', '需要登录', '仅试听', '解析待重试'])
  assert.equal(isTrialSong({ extra: { is_trial: 'true' } }), true)
})

test('returning from verification never claims that access succeeded', () => {
  const song = { source: 'kugou', url: 'https://example.com/stale', extra: { invalid_reason: '酷狗网页需要安全验证，请打开酷狗官网完成验证后重试' } }
  assert.equal(needsKugouVerification(song), true)
  const reset = resetVerificationState(song)
  assert.equal(reset.url, '')
  assert.equal(reset.is_invalid, true)
  assert.equal(needsKugouVerification(reset), false)
  assert.ok(song.extra.invalid_reason)
})

test('playback and seeking use the actual stream or declared preview bounds', () => {
  assert.equal(playableDuration(189, 45.5), 45.5)
  assert.equal(playableDuration(30, 45.5), 30)
  assert.equal(playableDuration(Infinity, 45.5), 45.5)
  assert.equal(playableDuration(NaN), 0)
  assert.equal(playbackSeek(189, 45.5), 45.5)
  assert.equal(playbackSeek(-10, 45.5), 0)
  assert.equal(playbackSeek(NaN, 45.5), null)
  assert.equal(playbackSeek(20, 0), null)
})
